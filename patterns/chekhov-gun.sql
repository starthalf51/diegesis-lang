-- pattern: chekhov-gun
-- layer: foreshadow
-- summary: 약속 복선(setup promise)이 회수(payoff)된다 — 체호프의 총
--
-- 매치: kind = promise 인 setup 마다 그 setup 을 가리키는 payoff 가 하나라도 있으면 한 행. 대상 = setup.
-- span = setup 의 서술 위치 ~ 가장 이른 payoff 의 서술 위치(보이지 않는 장면·최상위면 NULL).
-- 회수가 복선보다 앞서는지는 payoff-without-setup 규칙이 본다(여기서는 둘 다 있는지만).
SELECT 'setup' AS target_kind, s.id AS target_id,
       (SELECT x.discourse_ord FROM scene x WHERE x.id = s.scene_id AND x.shown) AS span_from,
       min(sp.discourse_ord) AS span_to,
       jsonb_build_object('prop', s.prop_id, 'text', s.text,
                          'payoffs', jsonb_agg(DISTINCT p.scene_id)) AS data
  FROM setup s
  JOIN payoff p ON p.work_id = s.work_id AND p.setup_id = s.id
  LEFT JOIN scene sp ON sp.id = p.scene_id AND sp.shown
 WHERE s.work_id = $1 AND s.kind = 'promise'
 GROUP BY s.id, s.scene_id, s.prop_id, s.text
