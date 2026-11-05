-- rule: payoff-without-setup
-- severity: warn
-- layer: lint
-- target: payoff
-- summary: 복선(setup)이 없거나 회수보다 앞서지 않는다
--
-- 대상: payoff 문장 하나하나(억제 대상은 sema 대로 (payoff, setup ID), 위치는 그 payoff 문장).
-- 위반: setup 이 없으면 warn. 둘 다 시점이 있으면 precedes(setup, payoff):
--   no → warn(회수가 먼저다), maybe → info(Review Focus 2), yes → 정상. 최상위(시점 없는) setup 은 처음부터 있다.
SELECT 'payoff' AS target_kind, p.setup_id AS target_id, p.src_id,
       CASE WHEN s.id IS NULL OR x.o = 'no' THEN 'warn' ELSE 'info' END AS severity,
       format(CASE WHEN s.id IS NULL THEN 'payoff of %s has no setup'
                   WHEN x.o = 'no' THEN 'payoff of %s comes before its setup'
                   ELSE 'payoff of %s may come before its setup (order not determined)' END,
              lint_short(p.setup_id)) AS message,
       jsonb_build_object('setup', p.setup_id, 'payoff_at', p.at_pt, 'order', coalesce(x.o, 'missing')) AS data
  FROM payoff p
  LEFT JOIN setup s ON s.work_id = p.work_id AND s.id = p.setup_id
  CROSS JOIN LATERAL (SELECT CASE WHEN s.id IS NULL THEN NULL
                                  WHEN s.at_pt IS NULL OR p.at_pt IS NULL THEN 'yes'
                                  ELSE precedes($1, s.at_pt, p.at_pt) END AS o) x
 WHERE p.work_id = $1 AND (s.id IS NULL OR x.o <> 'yes')
