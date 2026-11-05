-- rule: dead-actor
-- severity: warn
-- layer: lint
-- target: act
-- summary: 죽은 인물이 말하거나 행동한다
--
-- 대상: 행위자(해석된 화자. 청중은 보지 않는다) X 에게 `dies X` 시점 @X.died 가 있는 행위.
-- 위반: 죽음이 행위보다 반드시 앞선다 — precedes(@X.died, 행위) = yes 이고 precedes(행위, @X.died) ≠ yes
--   (같은 시각일 수 있으면 임종의 말로 본다). maybe 는 내지 않는다.
-- 면제: lint_exempt — knows oracle|omniscient (유령 등) 와 그런 그룹의 구성원. 그 밖의 경우는 억제로 처리한다.
SELECT 'act' AS target_kind, a.id AS target_id,
       format('%s %s after dying', lint_short(a.resolved_actor_id), a.verb) AS message,
       jsonb_build_object('entity', a.resolved_actor_id, 'died', d.id) AS data
  FROM act a JOIN point d ON d.work_id = a.work_id AND d.id = a.resolved_actor_id || '.died' AND d.kind = 'died'
 WHERE a.work_id = $1 AND NOT lint_exempt($1, a.resolved_actor_id)
   AND precedes($1, d.id, a.pt) = 'yes' AND precedes($1, a.pt, d.id) <> 'yes'
