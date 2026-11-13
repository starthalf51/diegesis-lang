-- rule: scene-no-turn
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 설계(design) 블록이 있는 모듈에서 turn 이 없는 보이는 장면
--
-- Only in a module (file) where at least one scene has a design block: a shown scene there with no `design.turn` has no value change to build toward.
-- Silent in modules that never use design (a work adopts design file by file).
SELECT 'scene' AS target_kind, s.id AS target_id,
       'scene has no design turn; the module uses design blocks' AS message,
       '{}'::jsonb AS data
  FROM scene s
 WHERE s.work_id = $1 AND s.shown
   AND EXISTS (SELECT 1 FROM directive d JOIN scene m ON m.id = d.scene_id WHERE d.work_id = $1 AND m.module_id IS NOT DISTINCT FROM s.module_id AND d.key LIKE 'design.%')
   AND NOT EXISTS (SELECT 1 FROM directive d WHERE d.work_id = $1 AND d.scene_id = s.id AND d.key = 'design.turn');
