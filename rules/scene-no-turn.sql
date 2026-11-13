-- rule: scene-no-turn
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 설계(design) 블록이 있는 작품에서 turn 이 없는 보이는 장면
--
-- Only in a work where at least one scene has a design block: a shown scene with no `design.turn` has no value change to build toward.
-- Silent in works that never use design.
SELECT 'scene' AS target_kind, s.id AS target_id,
       'scene has no design turn; the work uses design blocks' AS message,
       '{}'::jsonb AS data
  FROM scene s
 WHERE s.work_id = $1 AND s.shown
   AND EXISTS (SELECT 1 FROM directive d WHERE d.work_id = $1 AND d.key LIKE 'design.%')
   AND NOT EXISTS (SELECT 1 FROM directive d WHERE d.work_id = $1 AND d.scene_id = s.id AND d.key = 'design.turn');
