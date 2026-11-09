-- rule: object-location
-- severity: warn
-- layer: lint
-- target: act
-- summary: an object is handled in a scene at a place other than where it is declared to be
--
-- Convention: an object's location is `attr <thing>.at = <place>`, a scene's place is scene.place_id.
-- Target: acts whose object (including the target of does) X is a thing or artifact and whose scene has a place.
-- Value valid at the point: among X.at assertions outside an account, those necessarily before the act (precedes = yes; background uses origin),
--   with no other X.at assertion coming between them and the act (the same "immediately before" rule as attribute-conflict).
-- If several values are valid (assertions with undetermined order) all are candidates: it is quiet if at least one equals the scene's place.
-- Violation: there are valid values and none of them equals the scene's place. Not emitted when there is no valid value.
WITH origin AS (SELECT id FROM point WHERE work_id = $1 AND kind = 'origin' ORDER BY id LIMIT 1),
loc AS (
  SELECT x.entity_id, x.value, lint_canon($1, x.value) AS v, x.src_id,
         coalesce(x.at_pt, (SELECT id FROM origin)) AS pt
    FROM attribute x
   WHERE x.work_id = $1 AND x.key = 'at' AND x.account_id IS NULL),
cur AS (
  SELECT a.id AS act_id, s.place_id, o.x AS entity_id, l.value, l.v
    FROM act a JOIN scene s ON s.work_id = a.work_id AND s.id = a.scene_id AND s.place_id IS NOT NULL
         CROSS JOIN LATERAL unnest(a.objects) AS o(x)
         JOIN entity e ON e.work_id = a.work_id AND e.id = o.x AND e.kind IN ('thing', 'artifact')
         JOIN loc l ON l.entity_id = o.x AND precedes($1, l.pt, a.pt) = 'yes'
   WHERE a.work_id = $1
     AND NOT EXISTS (SELECT 1 FROM loc q WHERE q.entity_id = l.entity_id AND q.pt <> l.pt
                        AND precedes($1, l.pt, q.pt) = 'yes' AND precedes($1, q.pt, a.pt) = 'yes')),
bad AS (
  SELECT act_id, place_id, entity_id, min(value) AS value FROM cur c
   GROUP BY act_id, place_id, entity_id HAVING bool_and(v <> place_id))
SELECT 'act' AS target_kind, b.act_id AS target_id,
       format('%s %s %s in %s, but %s is at %s', lint_short(a.resolved_actor_id), coalesce(nullif(a.action, ''), a.verb), lint_short(b.entity_id),
              lint_short(b.place_id), lint_short(b.entity_id), lint_short(b.value)) AS message,
       jsonb_build_object('entity', b.entity_id, 'place', b.place_id, 'at', b.value) AS data
  FROM bad b JOIN act a ON a.work_id = $1 AND a.id = b.act_id
