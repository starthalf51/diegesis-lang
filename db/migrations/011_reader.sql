-- 011: reader model (Plan 2 Task 7, Review Focus 2). Everything about when the reader believes what, and at what value, is decided here.
--   Disclosure: contents of shown scenes' assertions (assertive: tells says whispers hails lies shows decrees sends), perceptions (sees; reads =
--     the propositions the document holds), asides (aside) and thoughts (thinks) are disclosed to the reader at that scene's discourse_ord.
--     asks, orders, writes and does disclose nothing. Acts in unseen events and accounts disclose nothing either.
--   The reader believes what was heard: value = the asserted value (a lie is believed as is). A proposition is normalized to (base fact, value)
--     (hearing `fact np = not p` gives (p, false)). If the opposite value is disclosed later, it is updated at that position.
--     Within one scene the last disclosure wins (scene granularity — a position is one scene slot).
--   `withhold P until X`: blocks disclosure of P from the position of the scene containing the withhold until X's narration position (the visible scene containing X).
--     A blocked disclosure does not vanish; it is deferred to X's position (the very start of that scene). If X is not in a visible scene, it is never disclosed.
--   `reveal P [at T]`: discloses P at that position (the visible scene containing T, else the scene containing the reveal). withhold does not block it.
-- ponytail: position = scene-level discourse_ord. Order within a scene is used only to pick the last disclosure. If sentence-level positions are needed, export sub.
SET LOCAL search_path = public, ag_catalog;

-- Proposition p ≡ (base = val): follow aliases, negation flips the value. A `w/not x` with no row is also a negation.
CREATE FUNCTION prop_norm(w text, p text, OUT base text, OUT val bool)
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb;
BEGIN
  base := p; val := true;
  FOR i IN 1..32 LOOP -- FOR i IN 1..32 LOOP -- guard against alias cycles
    SELECT x.def INTO d FROM fact x WHERE x.work_id = w AND x.id = base;
    IF NOT FOUND AND base LIKE w || '/not %' THEN
      base := substr(base, length(w) + 6); val := NOT val;
    ELSIF d ? 'alias' THEN
      base := d->>'alias';
    ELSIF d ? 'not' THEN
      base := d->>'not'; val := NOT val;
    ELSE
      RETURN;
    END IF;
  END LOOP;
END $$;

-- Narration position of the visible scene containing the point (scene start/end, beat, act). NULL outside visible scenes.
CREATE FUNCTION point_ord(w text, pt text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.discourse_ord FROM scene s
   WHERE s.work_id = w AND s.shown AND (pt IN (s.start_pt, s.end_pt) OR starts_with(pt, s.id || '#'))
   ORDER BY s.discourse_ord LIMIT 1 $$;

-- State predicates: base facts whose truth is fixed per point (holds(role, x) = holds the role, alive(x) = birth/death points).
CREATE FUNCTION fact_is_state(w text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT coalesce((SELECT def->>'pred' IN ('holds', 'alive') FROM fact WHERE work_id = w AND id = b), false) $$;

-- Whether base fact b is true at point at: true / false / NULL (undetermined).
--   holds(r, x): true if one of x's holding intervals for r (role_span, half-open [from, until)) certainly covers at; false if every interval
--     certainly misses (even with no intervals). alive(x): true if x.born ≤ at < x.died is certain; false if at < born or died ≤ at is certain.
--   Any other fact has its declared truth value (true/false) regardless of point. derived and unknown are NULL.
CREATE FUNCTION truth_at(w text, b text, at text) RETURNS bool
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb; t text; x text; born text; died text;
BEGIN
  SELECT f.def, f.truth INTO d, t FROM fact f WHERE f.work_id = w AND f.id = b;
  IF d->>'pred' = 'holds' THEN
    IF at IS NULL THEN RETURN NULL; END IF;
    IF EXISTS (SELECT 1 FROM role_span s WHERE s.work_id = w AND s.role_id = d->'args'->>0 AND s.entity_id = d->'args'->>1
                  AND (s.from_pt IS NULL OR precedes(w, s.from_pt, at) = 'yes')
                  AND (s.until_pt IS NULL OR (precedes(w, at, s.until_pt) = 'yes' AND precedes(w, s.until_pt, at) <> 'yes'))) THEN
      RETURN true;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM role_span s WHERE s.work_id = w AND s.role_id = d->'args'->>0 AND s.entity_id = d->'args'->>1
                      AND NOT ((s.from_pt IS NOT NULL AND precedes(w, at, s.from_pt) = 'yes' AND precedes(w, s.from_pt, at) <> 'yes')
                            OR (s.until_pt IS NOT NULL AND precedes(w, s.until_pt, at) = 'yes'))) THEN
      RETURN false;
    END IF;
    RETURN NULL;
  ELSIF d->>'pred' = 'alive' THEN
    IF at IS NULL THEN RETURN NULL; END IF;
    x := d->'args'->>0;
    SELECT p.id INTO born FROM point p WHERE p.work_id = w AND p.id = x || '.born';
    SELECT p.id INTO died FROM point p WHERE p.work_id = w AND p.id = x || '.died';
    IF (born IS NOT NULL AND precedes(w, at, born) = 'yes' AND precedes(w, born, at) <> 'yes')
       OR (died IS NOT NULL AND precedes(w, died, at) = 'yes') THEN
      RETURN false;
    END IF;
    IF (born IS NULL OR precedes(w, born, at) = 'yes')
       AND (died IS NULL OR (precedes(w, at, died) = 'yes' AND precedes(w, died, at) <> 'yes')) THEN
      RETURN true;
    END IF;
    RETURN NULL;
  END IF;
  RETURN CASE t WHEN 'true' THEN true WHEN 'false' THEN false END;
END $$;

-- Each reader disclosure: (base fact, value, narration position, index within scene, path, act). sub is used only to pick the last one at the same position.
CREATE FUNCTION reader_events(w text) RETURNS TABLE (fact_id text, value bool, ord int, sub int, via text, act_id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH told AS (
    SELECT c.prop_id, s.discourse_ord o, 2 * a.ord sub, a.verb, a.id
      FROM act a JOIN scene s ON s.id = a.scene_id
      CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id)
     WHERE a.work_id = w AND s.shown AND (assertive(a.verb) OR a.verb IN ('sees', 'aside', 'thinks'))
    UNION ALL
    SELECT p, s.discourse_ord, 2 * a.ord, a.verb, a.id
      FROM act a JOIN scene s ON s.id = a.scene_id JOIN artifact art ON art.id = ANY (a.objects)
      CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND s.shown AND a.verb = 'reads'),
  held AS (
    SELECT z.base, coalesce(sd.discourse_ord, 0) d_ord, point_ord(w, wh.until_pt) u_ord
      FROM withhold wh LEFT JOIN scene sd ON sd.id = wh.scene_id AND sd.shown
      CROSS JOIN LATERAL prop_norm(w, wh.prop_id) z
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL),
  acts AS (
    SELECT z.base, z.val, t.o, t.sub, t.verb, t.id, h.blocked, h.u_ord
      FROM told t CROSS JOIN LATERAL prop_norm(w, t.prop_id) z
      LEFT JOIN LATERAL (SELECT true blocked, h.u_ord FROM held h
                          WHERE h.base = z.base AND h.d_ord <= t.o AND (h.u_ord IS NULL OR h.u_ord > t.o)
                          ORDER BY h.u_ord DESC NULLS FIRST LIMIT 1) h ON true)
  SELECT base, val, CASE WHEN blocked THEN u_ord ELSE o END, CASE WHEN blocked THEN -1 ELSE sub END, verb, id
    FROM acts WHERE NOT coalesce(blocked, false) OR u_ord IS NOT NULL
  UNION ALL
  SELECT z.base, z.val, coalesce(point_ord(w, r.at_pt), sr.discourse_ord),
         coalesce(2 * (SELECT a.ord FROM act a WHERE a.id = r.at_pt) + 1, 1), 'reveal', NULL
    FROM reveal r LEFT JOIN scene sr ON sr.id = r.scene_id AND sr.shown
    CROSS JOIN LATERAL prop_norm(w, r.prop_id) z
   WHERE r.work_id = w AND r.prop_id IS NOT NULL AND coalesce(point_ord(w, r.at_pt), sr.discourse_ord) IS NOT NULL $$;

-- Change points of reader belief: from from_ord the reader believes fact_id = value (the first disclosure, then one row each time the value changes).
-- If f is given, only that proposition's base fact (fact_id is the base fact, value is also the base fact's value). NULL means all facts.
CREATE FUNCTION reader_knows(w text, f text DEFAULT NULL) RETURNS TABLE (fact_id text, from_ord int, value bool)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH e AS (SELECT * FROM reader_events(w) x WHERE f IS NULL OR x.fact_id = (SELECT base FROM prop_norm(w, f))),
  last AS (SELECT DISTINCT ON (e.fact_id, e.ord) e.fact_id, e.ord, e.value FROM e
            ORDER BY e.fact_id, e.ord, e.sub DESC, e.act_id DESC NULLS LAST),
  ch AS (SELECT l.*, lag(l.value) OVER (PARTITION BY l.fact_id ORDER BY l.ord) prev FROM last l)
  SELECT ch.fact_id, ch.ord, ch.value FROM ch WHERE ch.prev IS DISTINCT FROM ch.value ORDER BY 1, 2 $$;

-- The (fact, value) the reader believes at narration position o (including that scene's disclosures).
CREATE FUNCTION reader_state(w text, o int) RETURNS TABLE (fact_id text, value bool)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT DISTINCT ON (k.fact_id) k.fact_id, k.value FROM reader_knows(w) k WHERE k.from_ord <= o
   ORDER BY k.fact_id, k.from_ord DESC $$;

-- Whether character e knows base fact fb as fv at point at: 'yes' / 'opp' (believes the opposite) / 'maybe' (only sources with undetermined order) / 'no'.
--   Sources = believes rows of e (or a group e belongs to) (rows withdrawn by doubt drop out from until_pt), and what e itself asserted (not lies), asided or thought.
--   The source point is definitely before at (the same point only for declared beliefs). Background knowledge (no point) counts from the start.
CREATE FUNCTION char_knows(w text, ent text, fb text, fv bool, at text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src AS (
    SELECT (z.val = b.value) v, b.from_pt pt, b.until_pt upt, b.declared
      FROM believes b JOIN lint_self_and_groups(w, ent) u ON u.id = b.entity_id
      CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
     WHERE b.work_id = w AND z.base = fb
    UNION ALL
    SELECT z.val, c.pt, NULL, false
      FROM act_content c CROSS JOIN LATERAL prop_norm(w, c.prop_id) z
     WHERE c.work_id = w AND c.actor_id = ent AND z.base = fb
       AND (assertive(c.verb) AND c.verb <> 'lies' OR c.verb IN ('aside', 'thinks'))),
  held AS (
    SELECT v, (pt IS NULL OR ((pt <> at OR declared) AND precedes(w, pt, at) = 'yes'))
              AND (upt IS NULL OR (precedes(w, at, upt) = 'yes' AND precedes(w, upt, at) <> 'yes')) AS sure
      FROM src WHERE (pt IS NULL OR ((pt <> at OR declared) AND precedes(w, pt, at) <> 'no'))
                 AND (upt IS NULL OR precedes(w, upt, at) <> 'yes')) -- AND (upt IS NULL OR precedes(w, upt, at) <> 'yes')) -- a definitely withdrawn belief is not even maybe
  SELECT CASE WHEN bool_or(sure AND v = fv) THEN 'yes' WHEN bool_or(sure AND v <> fv) THEN 'opp'
              WHEN bool_or(v = fv) THEN 'maybe' ELSE 'no' END FROM held $$;
