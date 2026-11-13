-- rule: scene-ends-passive
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 설계(design)와 시점(pov)이 있는 장면이 시점 인물의 능동 행위로 끝나지 않는다
--
-- A shown scene with a design block and a pov whose last act in telling order (beats included) is not an active act by the pov character.
--   Active verbs: does, says, tells, asks, orders, writes, sends, shows, lies, whispers, hails, decrees, enacts. Anything else (sees, reads, thinks, ...) is passive.
--   pov is shortened with lint_short and compared with lint_short(actor). A role actor resolve_roles could not resolve is skipped (it may be the pov).
SELECT 'scene' AS target_kind, s.id AS target_id,
       format('scene ends on %s by %s; end it on the point-of-view character''s decision or action', la.verb, lint_short(coalesce(la.resolved_actor_id, la.actor_id))) AS message,
       jsonb_build_object('verb', la.verb, 'actor', coalesce(la.resolved_actor_id, la.actor_id)) AS data
  FROM scene s
  JOIN LATERAL (SELECT lint_short(d."values"[1]) AS v FROM directive d
                 WHERE d.work_id = $1 AND d.scene_id = s.id AND d.key = 'pov' LIMIT 1) pv ON pv.v IS NOT NULL
  JOIN LATERAL (SELECT a.* FROM act a WHERE a.work_id = $1 AND a.scene_id = s.id ORDER BY a.ord DESC LIMIT 1) la ON true
 WHERE s.work_id = $1 AND s.shown
   AND EXISTS (SELECT 1 FROM directive d WHERE d.work_id = $1 AND d.scene_id = s.id AND d.key LIKE 'design.%')
   AND NOT (la.resolved_actor_id IS NULL AND la.actor_is_role)
   AND NOT (la.verb IN ('does', 'says', 'tells', 'asks', 'orders', 'writes', 'sends', 'shows', 'lies', 'whispers', 'hails', 'decrees', 'enacts')
            AND lint_short(coalesce(la.resolved_actor_id, la.actor_id)) = pv.v);
