-- rule: unpaid-promise
-- severity: warn
-- layer: lint
-- target: setup
-- summary: promise 복선이 회수되지 않았다
--
-- 대상: kind = promise 인 setup 중 같은 작품에 그 setup 을 가리키는 payoff 가 하나도 없는 것.
-- hint·symbol 은 회수를 요구하지 않는다. payoff 의 순서는 payoff-without-setup 이 본다.
SELECT 'setup' AS target_kind, s.id AS target_id,
       format('promise %s is never paid off', lint_short(s.id)) AS message,
       jsonb_build_object('setup', s.id, 'prop', s.prop_id) AS data
  FROM setup s
 WHERE s.work_id = $1 AND s.kind = 'promise'
   AND NOT EXISTS (SELECT 1 FROM payoff p WHERE p.work_id = $1 AND p.setup_id = s.id)
