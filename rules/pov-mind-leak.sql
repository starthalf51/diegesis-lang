-- rule: pov-mind-leak
-- severity: error
-- layer: lint
-- target: act
-- summary: 1인칭 장면에서 화자가 남의 속마음을 안다
--
-- Violation: the scene is `render narration first-person` with `render pov X`, and a `thinks` act belongs to someone other than X.
--   A first-person narrator cannot know another character's thoughts. Third-person scenes and scenes with no pov are never reported.
--   pov is stored as written (short or `module/id`); it is shortened with lint_short and compared with lint_short(actor). An actor that is a role
--   resolve_roles could not resolve is skipped (it may be X).
SELECT 'act' AS target_kind, a.id AS target_id,
       format('first-person narrator "%s" cannot know the thoughts of "%s"', pv.v, lint_short(coalesce(a.resolved_actor_id, a.actor_id))) AS message,
       jsonb_build_object('pov', pv.v, 'actor', coalesce(a.resolved_actor_id, a.actor_id)) AS data
  FROM act a
  JOIN LATERAL (SELECT lint_short(d."values"[1]) AS v FROM directive d
                 WHERE d.work_id = $1 AND d.scene_id = a.scene_id AND d.key = 'pov' LIMIT 1) pv ON pv.v IS NOT NULL
 WHERE a.work_id = $1 AND a.verb = 'thinks'
   AND NOT (a.resolved_actor_id IS NULL AND a.actor_is_role)
   AND EXISTS (SELECT 1 FROM directive n WHERE n.work_id = $1 AND n.scene_id = a.scene_id
                  AND n.key = 'narration' AND n."values"[1] = 'first-person')
   AND lint_short(coalesce(a.resolved_actor_id, a.actor_id)) <> pv.v
