-- rule: pov-offstage
-- severity: error
-- layer: lint
-- target: scene
-- summary: 시점 인물이 장면에 한 번도 등장하지 않는다
--
-- Violation: a scene has `render pov X` but X never enters it (no presence row; `mention` does not count, and a later exit is not considered).
--   pov is stored as written (short or `module/id`); it is shortened with lint_short and matched against lint_short(entity id). A scene with no pov is never reported.
SELECT 'scene' AS target_kind, d.scene_id AS target_id,
       format('pov character "%s" never enters this scene', d."values"[1]) AS message,
       jsonb_build_object('pov', d."values"[1]) AS data
  FROM directive d
 WHERE d.work_id = $1 AND d.key = 'pov' AND d."values"[1] IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM presence p
                    WHERE p.work_id = $1 AND p.scene_id = d.scene_id AND lint_short(p.entity_id) = lint_short(d."values"[1]))
