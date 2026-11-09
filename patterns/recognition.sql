-- pattern: recognition
-- layer: identity
-- summary: the set of characters who believe an identity fact (fact … tag identity) grows in narration order, and grows via sees at least once — recognition (anagnorisis)
--
-- For each identity fact f (tag identity, normalized (base, value)): from the believes rows that believe it (value included) and were not withdrawn by doubt
-- (no until_pt), find the narration position where each character first came to believe it (background knowledge = 0; sources in unseen scenes are not counted).
-- Match if there are two or more stages at position > 0 (characters who newly came to believe at the same position) and at least one has a sees source.
-- Target = identity fact. span = first stage ~ last stage. data.steps = per stage (position, new characters, route verb).
WITH idf AS (
  SELECT f.id, z.base, z.val FROM fact f CROSS JOIN LATERAL prop_norm($1, f.id) z WHERE f.work_id = $1 AND f.tag = 'identity'),
b AS (
  SELECT i.id AS fact, x.entity_id,
         CASE WHEN x.from_pt IS NULL THEN 0 ELSE point_ord($1, x.from_pt) END AS ord,
         coalesce(y.verb, CASE WHEN x.declared THEN 'believes' END) AS verb
    FROM believes x CROSS JOIN LATERAL prop_norm($1, x.prop_id) z
    JOIN idf i ON i.base = z.base AND i.val = (z.val = x.value)
    LEFT JOIN act y ON y.id = x.cause_act
   WHERE x.work_id = $1 AND x.until_pt IS NULL),
firsts AS (
  SELECT DISTINCT ON (fact, entity_id) fact, entity_id, ord, verb FROM b WHERE ord IS NOT NULL
   ORDER BY fact, entity_id, ord, (verb = 'sees') DESC),
steps AS (
  SELECT fact, ord, array_agg(entity_id ORDER BY entity_id) AS added, array_agg(DISTINCT verb) AS via,
         bool_or(verb = 'sees') AS by_sight
    FROM firsts WHERE ord > 0 GROUP BY fact, ord)
SELECT 'fact' AS target_kind, fact AS target_id, min(ord) AS span_from, max(ord) AS span_to,
       jsonb_build_object('steps', jsonb_agg(jsonb_build_object('ord', ord, 'added', added, 'via', via) ORDER BY ord)) AS data
  FROM steps GROUP BY fact
HAVING count(*) >= 2 AND bool_or(by_sight)
