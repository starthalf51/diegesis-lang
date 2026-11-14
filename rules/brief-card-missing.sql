-- rule: brief-card-missing
-- severity: info
-- layer: lint
-- target: scene
-- summary: 장면에 나오거나 말하는 인물에게 인물 카드(role·want·look·voice)가 없다
--
-- Data gap for `dieg brief`: a character on stage (presence) or speaking into the scene (act actor) has none of
--   `attr <character>.role/want/look/voice`. Reported only in a work that gives at least one character a card.
WITH who AS (SELECT p.scene_id, p.entity_id FROM presence p WHERE p.work_id = $1
             UNION SELECT a.scene_id, a.actor_id FROM act a WHERE a.work_id = $1 AND a.scene_id IS NOT NULL)
SELECT DISTINCT 'scene' AS target_kind, w.scene_id AS target_id,
       format('%s has no card (role, want, look or voice)', render_name($1, w.entity_id, 'en')) AS message,
       jsonb_build_object('character', w.entity_id) AS data
  FROM who w JOIN entity e ON e.id = w.entity_id AND e.kind = 'character'
 WHERE EXISTS (SELECT 1 FROM attribute x JOIN entity xe ON xe.id = x.entity_id AND xe.kind = 'character'
                WHERE x.work_id = $1 AND x.key IN ('role', 'want', 'look', 'voice'))
   AND NOT EXISTS (SELECT 1 FROM attribute c WHERE c.work_id = $1 AND c.entity_id = w.entity_id
                    AND c.key IN ('role', 'want', 'look', 'voice'))
