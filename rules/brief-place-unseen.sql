-- rule: brief-place-unseen
-- severity: info
-- layer: lint
-- target: scene
-- summary: 장면의 장소에 감각 낱말(look)이 없다
--
-- Data gap for `dieg brief`: a scene's place has no `attr <place>.look`, so the brief gives the writer nothing to
--   see there. Reported only in a work that gives at least one place a look.
SELECT 'scene' AS target_kind, s.id AS target_id,
       format('place %s has no look', render_name($1, s.place_id, 'en')) AS message,
       jsonb_build_object('place', s.place_id) AS data
  FROM scene s
 WHERE s.work_id = $1 AND s.place_id IS NOT NULL
   AND EXISTS (SELECT 1 FROM attribute x JOIN entity xe ON xe.id = x.entity_id AND xe.kind = 'place'
                WHERE x.work_id = $1 AND x.key = 'look')
   AND NOT EXISTS (SELECT 1 FROM attribute a WHERE a.work_id = $1 AND a.entity_id = s.place_id AND a.key = 'look')
