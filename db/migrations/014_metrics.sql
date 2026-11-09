-- 014: analysis metrics (Plan 2 Task 8). Time series and the character graph that Grafana reads. All computed inside PG (ADR-010).
--   x axis = discourse_ord of visible scenes (one scene is one slot). Every function takes (work text) and returns 0 rows for an empty work.
--   The reader model uses 011 and 013's reader_knows, patterns use 013's pattern_result view as is. Nothing is decided again here.
SET LOCAL search_path = public, ag_catalog;

-- Narration positions of visible scenes (the metrics' x axis).
CREATE FUNCTION metric_ords(w text) RETURNS TABLE (ord int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT DISTINCT s.discourse_ord FROM scene s WHERE s.work_id = w AND s.shown $$;

-- Presence and act counts per scene (visible scenes only; a character who just stands silent has acts = 0).
CREATE FUNCTION metric_presence(w text) RETURNS TABLE (ord int, scene text, entity text, acts int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.discourse_ord, s.id, p.entity_id,
         (SELECT count(*)::int FROM act a WHERE a.work_id = w AND a.scene_id = s.id
             AND coalesce(a.resolved_actor_id, a.actor_id) = p.entity_id)
    FROM scene s JOIN (SELECT DISTINCT scene_id, entity_id FROM presence WHERE work_id = w) p ON p.scene_id = s.id
   WHERE s.work_id = w AND s.shown
   ORDER BY 1, 2, 3 $$;

-- True propositions the reader believes at the end of that scene (same rule as reader_state: including that scene's disclosures), of which
-- the number that characters in the scene do not know (char_knows = no | opp; maybe is not counted — same as the dramatic-irony pattern).
-- Prophets and omniscients (lint_exempt) are excluded. ponytail: true = only what truth_at settles (derived and unknown facts are NULL, so they drop out).
-- ponytail: char_knows is called per (scene x present character x true fact). If a big work gets slow, materialize.
CREATE FUNCTION metric_asymmetry(w text) RETURNS TABLE (ord int, entity text, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH kn AS (SELECT * FROM reader_knows(w)),
  sc AS (SELECT s.id, s.discourse_ord o, coalesce(s.end_pt, s.start_pt) pt FROM scene s
          WHERE s.work_id = w AND s.shown AND coalesce(s.end_pt, s.start_pt) IS NOT NULL),
  st AS (
    SELECT sc.id, sc.o, sc.pt, r.fact_id, r.value
      FROM sc CROSS JOIN LATERAL (SELECT DISTINCT ON (kn.fact_id) kn.fact_id, kn.value FROM kn
                                   WHERE kn.from_ord <= sc.o ORDER BY kn.fact_id, kn.from_ord DESC) r
     WHERE truth_at(w, r.fact_id, sc.pt) = r.value),
  pe AS (SELECT DISTINCT scene_id, entity_id FROM presence WHERE work_id = w)
  SELECT st.o, pe.entity_id,
         (count(*) FILTER (WHERE char_knows(w, pe.entity_id, st.fact_id, st.value, st.pt) IN ('no', 'opp')))::int
    FROM st JOIN pe ON pe.scene_id = st.id
   WHERE NOT lint_exempt(w, pe.entity_id)
   GROUP BY st.o, pe.entity_id
   ORDER BY 1, 2 $$;

-- Open setup count: among the setups that have appeared up to that position (visible scenes containing the setup's scene or point), those whose payoff has not yet arrived by that position.
-- (Equals cumulative setup − cumulative payoff. Counted per setup so it never goes negative even if a payoff precedes its setup.)
CREATE FUNCTION metric_open_setups(w text) RETURNS TABLE (ord int, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH su AS (
    SELECT s.id, coalesce((SELECT x.discourse_ord FROM scene x WHERE x.id = s.scene_id AND x.shown), point_ord(w, s.at_pt)) o
      FROM setup s WHERE s.work_id = w),
  pa AS (
    SELECT p.setup_id, min(coalesce((SELECT x.discourse_ord FROM scene x WHERE x.id = p.scene_id AND x.shown), point_ord(w, p.at_pt))) o
      FROM payoff p WHERE p.work_id = w GROUP BY p.setup_id)
  SELECT m.ord, (SELECT count(*)::int FROM su LEFT JOIN pa ON pa.setup_id = su.id
                  WHERE su.o <= m.ord AND (pa.o IS NULL OR pa.o > m.ord))
    FROM metric_ords(w) m ORDER BY 1 $$;

-- Active deception count: among those who received a deception (deception rows: lies, false-document writes), whether someone still believes the falsehood at that position.
-- Belief row = believes (value, cause_act = that lies act or the reads of that document). Withdrawn if until_pt (doubt) has passed in narration position.
-- What is counted is not people but deceptions (act, proposition): 1 if at least one recipient still believes.
CREATE FUNCTION metric_active_deceptions(w text) RETURNS TABLE (ord int, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH live AS (  -- WITH live AS (  -- interval during which a belief stands for each (deception act, proposition, recipient) [from, until)
    SELECT d.act_id, d.prop_id,
           (SELECT x.discourse_ord FROM act c JOIN scene x ON x.id = c.scene_id AND x.shown WHERE c.id = b.cause_act) f,
           point_ord(w, b.until_pt) u
      FROM deception d
      JOIN believes b ON b.work_id = w AND b.value AND b.entity_id <> d.liar_id
                     AND lint_canon(w, b.prop_id) = lint_canon(w, d.prop_id)
                     AND (b.cause_act = d.act_id
                          OR EXISTS (SELECT 1 FROM act r JOIN act wr ON wr.id = d.act_id
                                      WHERE r.id = b.cause_act AND r.verb = 'reads' AND wr.objects && r.objects))
     WHERE d.work_id = w)
  SELECT m.ord, (SELECT count(DISTINCT (l.act_id, l.prop_id))::int FROM live l
                  WHERE l.f <= m.ord AND (l.u IS NULL OR l.u > m.ord))
    FROM metric_ords(w) m ORDER BY 1 $$;

-- Weight (independent of the work). Need not sum to 1.
CREATE TABLE metric_weights (term text PRIMARY KEY, weight numeric NOT NULL);
INSERT INTO metric_weights VALUES ('promise', 0.4), ('deception', 0.3), ('pattern', 0.3);

-- Tension = Σ weight × (term / that term's maximum within the work). If the maximum is 0 the term is 0 (no division by zero).
--   open promises = open setup count (metric_open_setups), active deceptions, and the number of dramatic-irony/mystery pattern matches covering that position (pattern_result).
CREATE FUNCTION metric_tension(w text) RETURNS TABLE (ord int, value numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH t AS (
    SELECT s.ord, s.n a, d.n b,
           (SELECT count(*) FROM pattern_result m WHERE m.work_id = w AND m.pattern_id IN ('dramatic-irony', 'mystery')
               AND s.ord BETWEEN m.span_from AND m.span_to) c
      FROM metric_open_setups(w) s JOIN metric_active_deceptions(w) d USING (ord)),
  wt AS (SELECT coalesce(max(weight) FILTER (WHERE term = 'promise'), 0) p, coalesce(max(weight) FILTER (WHERE term = 'deception'), 0) d,
                coalesce(max(weight) FILTER (WHERE term = 'pattern'), 0) q FROM metric_weights)
  SELECT t.ord, round(wt.p * coalesce(t.a::numeric / nullif(max(t.a) OVER (), 0), 0)
                    + wt.d * coalesce(t.b::numeric / nullif(max(t.b) OVER (), 0), 0)
                    + wt.q * coalesce(t.c::numeric / nullif(max(t.c) OVER (), 0), 0), 4)
    FROM t CROSS JOIN wt ORDER BY 1 $$;

-- Time warp: x = narration position, y = earliest (story time) at scene start. Points with no time constraint (earliest = 0 and no latest: the solver default) are dropped.
CREATE FUNCTION metric_anachrony(w text) RETURNS TABLE (scene text, ord int, story_t int8)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.id, s.discourse_ord, p.earliest FROM scene s JOIN point p ON p.id = s.start_pt
   WHERE s.work_id = w AND s.shown AND p.earliest IS NOT NULL AND (p.earliest > 0 OR p.latest IS NOT NULL) ORDER BY 2, 1 $$;

-- Pace: the story-time width of a scene (end earliest − start earliest, seconds) versus its act count.
-- If either end is a point with no time constraint (earliest = 0 and no latest: the solver default), span_s = NULL.
CREATE FUNCTION metric_pace(w text) RETURNS TABLE (scene text, ord int, span_s int8, acts int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.id, s.discourse_ord,
         CASE WHEN (b.earliest > 0 OR b.latest IS NOT NULL) AND (e.earliest > 0 OR e.latest IS NOT NULL)
              THEN e.earliest - b.earliest END,
         (SELECT count(*)::int FROM act a WHERE a.work_id = w AND a.scene_id = s.id)
    FROM scene s LEFT JOIN point b ON b.id = s.start_pt LEFT JOIN point e ON e.id = s.end_pt
   WHERE s.work_id = w AND s.shown ORDER BY 2, 1 $$;

-- Grafana node graph: nodes(id, title, "mainStat") / edges(id, source, target, "mainStat"). Characters only. Node mainStat = act count.
CREATE FUNCTION graph_nodes(w text) RETURNS TABLE (id text, title text, "mainStat" numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT e.id, e.name, (SELECT count(*) FROM act a WHERE a.work_id = w AND coalesce(a.resolved_actor_id, a.actor_id) = e.id)::numeric
    FROM entity e WHERE e.work_id = w AND e.kind = 'character' ORDER BY 1 $$;

-- Edge = number of visible scenes they attended together + number of TOLD (in either direction). One per pair (source < target).
CREATE FUNCTION graph_edges(w text) RETURNS TABLE (id text, source text, target text, "mainStat" numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH pe AS (SELECT DISTINCT p.scene_id, p.entity_id FROM presence p JOIN scene s ON s.id = p.scene_id AND s.shown
               WHERE p.work_id = w),
  co AS (SELECT least(a.entity_id, b.entity_id) x, greatest(a.entity_id, b.entity_id) y, count(*) n
           FROM pe a JOIN pe b ON a.scene_id = b.scene_id AND a.entity_id < b.entity_id GROUP BY 1, 2),
  told AS (SELECT least(c.actor_id, au.entity_id) x, greatest(c.actor_id, au.entity_id) y, count(*) n
             FROM act_content c JOIN audience au ON au.act_id = c.act_id
            WHERE c.work_id = w AND assertive(c.verb) AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id GROUP BY 1, 2),
  u AS (SELECT * FROM co UNION ALL SELECT * FROM told),
  ch AS (SELECT entity.id FROM entity WHERE entity.work_id = w AND entity.kind = 'character')
  SELECT u.x || '~' || u.y, u.x, u.y, sum(u.n)::numeric FROM u
   WHERE u.x IN (SELECT ch.id FROM ch) AND u.y IN (SELECT ch.id FROM ch)
   GROUP BY u.x, u.y ORDER BY 1 $$;
