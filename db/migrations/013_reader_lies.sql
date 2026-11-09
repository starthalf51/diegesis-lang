-- 013: Task 7 fix round 1. 011 and 012 are not edited (migrations are append-only).
--   1) The reader does not believe a deception it already knows is false. Changes reader_events.
--      Deceptive disclosure = a lies act, or a reads act of a document written as deception (a writes with a deception row — a forged letter).
--      Disclosure by a deceived speaker = the speaker heard that (b, v) only from deceptive disclosures (all of that speaker's (b, v) believes rows came from deceptive acts).
--      The liar's own row (caused by their own lies) is not deceived. A row where someone reads the forged document themselves is deceived.
--      A disclosure (b, v) that is either one is dropped if an earlier position ((ord, sub) order) had a non-deceptive disclosure of (b, ¬v).
--      This includes when the liar first told the reader ¬v by aside or thought (that too is a non-deceptive disclosure).
--      Without such an earlier disclosure the reader still believes what was heard (011's default).
--      ponytail: a deceived speaker is followed one step only (words of someone who heard again from the deceived are believed as is). Widen with recursion if chains grow.
--   2) pattern_result: the last result of a full pattern-library run (dieg patterns / dieg test, run_id 'patterns-…').
--      lint inserts only declared patterns under its own run ('lint-…'), so it does not overwrite this view. The dashboard reads this view.
SET LOCAL search_path = public, ag_catalog;

-- Deceptive disclosure acts: lies, and reads of propositions in a document written as deception (per proposition).
CREATE FUNCTION deceptive_act(w text, x_act text, x_prop text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM act a WHERE a.work_id = w AND a.id = x_act AND a.verb = 'lies')
      OR EXISTS (SELECT 1 FROM act r JOIN act wr ON wr.work_id = w AND wr.verb = 'writes' AND wr.objects && r.objects
                   JOIN deception d ON d.act_id = wr.id AND lint_canon(w, d.prop_id) = lint_canon(w, x_prop)
                  WHERE r.work_id = w AND r.id = x_act AND r.verb = 'reads') $$;

CREATE OR REPLACE FUNCTION reader_events(w text) RETURNS TABLE (fact_id text, value bool, ord int, sub int, via text, act_id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH told AS (
    SELECT c.prop_id, s.discourse_ord o, 2 * a.ord sub, a.verb, a.id, a.resolved_actor_id actor
      FROM act a JOIN scene s ON s.id = a.scene_id
      CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id)
     WHERE a.work_id = w AND s.shown AND (assertive(a.verb) OR a.verb IN ('sees', 'aside', 'thinks'))
    UNION ALL
    SELECT p, s.discourse_ord, 2 * a.ord, a.verb, a.id, a.resolved_actor_id
      FROM act a JOIN scene s ON s.id = a.scene_id JOIN artifact art ON art.id = ANY (a.objects)
      CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND s.shown AND a.verb = 'reads'),
  held AS (
    SELECT z.base, coalesce(sd.discourse_ord, 0) d_ord, point_ord(w, wh.until_pt) u_ord
      FROM withhold wh LEFT JOIN scene sd ON sd.id = wh.scene_id AND sd.shown
      CROSS JOIN LATERAL prop_norm(w, wh.prop_id) z
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL),
  acts AS (
    SELECT z.base, z.val, t.o, t.sub, t.verb, t.id, t.actor, deceptive_act(w, t.id, t.prop_id) dec, h.blocked, h.u_ord
      FROM told t CROSS JOIN LATERAL prop_norm(w, t.prop_id) z
      LEFT JOIN LATERAL (SELECT true blocked, h.u_ord FROM held h
                          WHERE h.base = z.base AND h.d_ord <= t.o AND (h.u_ord IS NULL OR h.u_ord > t.o)
                          ORDER BY h.u_ord DESC NULLS FIRST LIMIT 1) h ON true),
  ev AS (
    SELECT base, val, CASE WHEN blocked THEN u_ord ELSE o END o, CASE WHEN blocked THEN -1 ELSE sub END sub, verb, id, actor, dec
      FROM acts WHERE NOT coalesce(blocked, false) OR u_ord IS NOT NULL
    UNION ALL
    SELECT z.base, z.val, coalesce(point_ord(w, r.at_pt), sr.discourse_ord),
           coalesce(2 * (SELECT a.ord FROM act a WHERE a.id = r.at_pt) + 1, 1), 'reveal', NULL, NULL, false
      FROM reveal r LEFT JOIN scene sr ON sr.id = r.scene_id AND sr.shown
      CROSS JOIN LATERAL prop_norm(w, r.prop_id) z
     WHERE r.work_id = w AND r.prop_id IS NOT NULL AND coalesce(point_ord(w, r.at_pt), sr.discourse_ord) IS NOT NULL),
  dupe AS (  -- deceived speaker: heard that (b, v) only from deceptive disclosures
    SELECT e.id, e.base, e.val FROM ev e
     WHERE NOT e.dec AND e.actor IS NOT NULL
       AND EXISTS (SELECT 1 FROM believes b CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
                    WHERE b.work_id = w AND b.entity_id = e.actor AND z.base = e.base AND (z.val = b.value) = e.val
                      AND b.cause_act IS NOT NULL AND deceptive_act(w, b.cause_act, b.prop_id)
                      AND NOT EXISTS (SELECT 1 FROM act y WHERE y.id = b.cause_act AND y.resolved_actor_id = b.entity_id AND y.verb = 'lies'))
       AND NOT EXISTS (SELECT 1 FROM believes b CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
                        WHERE b.work_id = w AND b.entity_id = e.actor AND z.base = e.base AND (z.val = b.value) = e.val
                          AND (b.cause_act IS NULL OR NOT deceptive_act(w, b.cause_act, b.prop_id)
                               OR EXISTS (SELECT 1 FROM act y WHERE y.id = b.cause_act AND y.resolved_actor_id = b.entity_id AND y.verb = 'lies'))))
  SELECT e.base, e.val, e.o, e.sub, e.verb, e.id FROM ev e
   WHERE NOT (e.dec OR EXISTS (SELECT 1 FROM dupe d WHERE d.id = e.id AND d.base = e.base AND d.val = e.val))
      OR NOT EXISTS (SELECT 1 FROM ev x WHERE NOT x.dec AND x.base = e.base AND x.val <> e.val
                        AND (x.o, x.sub) < (e.o, e.sub)
                        AND NOT EXISTS (SELECT 1 FROM dupe d WHERE d.id = x.id AND d.base = x.base AND d.val = x.val)) $$;

CREATE VIEW pattern_result AS
  SELECT m.* FROM pattern_match m
   WHERE m.run_id LIKE 'patterns-%'
     AND m.run_id = (SELECT max(x.run_id) FROM pattern_match x WHERE x.work_id = m.work_id AND x.run_id LIKE 'patterns-%');
