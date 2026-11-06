-- pattern: prophecy-fulfilled
-- layer: foreshadow
-- summary: 예언자(knows oracle)가 단언한 명제가 그때는 참이 아니었거나 미정이었고, 나중에 참이 된다
--
-- 예언 = 예언자(자신 또는 그룹이 knows oracle)의 단언(거짓말 제외)이 전하는 명제 P ≡ (기저 b = v).
-- 그때 참이 아님(또는 미정): 다음 중 하나. 시점이 없는 사실(상태 술어가 아님)은 둘째 조건으로만 판정한다
--   (선언 진리값은 시점과 무관해서 "그때 참이 아님" 을 말해 주지 못한다).
--   - b 가 상태 술어(holds·alive)이고 그 시점에 truth_at(b) ≠ v(거짓 또는 정해지지 않음)
--   - 청중 누구도 그 시점에 P 를 알지 못한다(char_knows ≠ yes: 청자에게는 미정. 코더형 — 이미 참이지만 아무도 모른다)
-- 나중에 참이 됨(경로 route, 하나라도):
--   state     상태 술어가 예언 뒤의 보이는 장면 끝에서 확실히 v(역할 보유 구간 / 출생·사망 시점)
--   confirmed 예언자가 아닌 인물이 예언 뒤에 P 를 단언하고, 그 시점에 truth_at = v 이며, 그 인물의 P 믿음 중
--             예언자의 행위에서 온 것이 없고, 그 단언의 청중이 예언의 청중과 겹친다(예언을 들은 사람에게 확인해 준다 —
--             예언을 되풀이한 것, 예언과 무관한 사람들 사이에서 지난 사실을 밝히는 것은 확인이 아니다)
--   payoff    예언과 같은 장면에서 P 를 promise 복선으로 걸고 예언 뒤에 회수한다(작가가 적은 실현)
-- 대상 = 예언 행위. span = 예언 위치 ~ 가장 이른 실현 위치(보이지 않는 장면이면 NULL).
WITH proph AS (
  SELECT a.* FROM act_props($1) a
   WHERE assertive(a.verb) AND a.verb <> 'lies' AND is_oracle($1, a.actor_id)),
notyet AS (
  SELECT p.* FROM proph p
   WHERE (fact_is_state($1, p.base) AND truth_at($1, p.base, p.pt) IS DISTINCT FROM p.val)
      OR NOT EXISTS (SELECT 1 FROM audience au WHERE au.act_id = p.act_id AND au.entity_id IS DISTINCT FROM p.actor_id
                        AND char_knows($1, au.entity_id, p.base, p.val, p.pt) = 'yes')),
f AS (
  SELECT n.act_id, n.prop_id, 'state' AS route, min(s.discourse_ord) AS ord, NULL::text AS by_act
    FROM notyet n JOIN scene s ON s.work_id = $1 AND s.shown
   WHERE fact_is_state($1, n.base) AND truth_at($1, n.base, n.pt) IS DISTINCT FROM n.val
     AND precedes($1, n.pt, s.end_pt) = 'yes' AND truth_at($1, n.base, s.end_pt) = n.val
   GROUP BY n.act_id, n.prop_id
  UNION ALL
  SELECT n.act_id, n.prop_id, 'confirmed', b.ord, b.act_id
    FROM notyet n JOIN act_props($1) b ON b.base = n.base AND b.val = n.val AND b.act_id <> n.act_id
   WHERE assertive(b.verb) AND b.verb <> 'lies' AND NOT is_oracle($1, b.actor_id)
     AND precedes($1, n.pt, b.pt) = 'yes' AND precedes($1, b.pt, n.pt) <> 'yes'
     AND truth_at($1, b.base, b.pt) = b.val
     AND NOT EXISTS (SELECT 1 FROM believes x JOIN act y ON y.id = x.cause_act
                       CROSS JOIN LATERAL prop_norm($1, x.prop_id) z
                      WHERE x.work_id = $1 AND x.entity_id = b.actor_id AND z.base = b.base
                        AND is_oracle($1, y.resolved_actor_id))
     AND EXISTS (SELECT 1 FROM audience pa JOIN audience ca ON ca.entity_id = pa.entity_id
                  WHERE pa.act_id = n.act_id AND ca.act_id = b.act_id)
  UNION ALL
  SELECT n.act_id, n.prop_id, 'payoff', sp.discourse_ord, NULL
    FROM notyet n
    JOIN setup st ON st.work_id = $1 AND st.kind = 'promise' AND st.scene_id = n.scene_id
    CROSS JOIN LATERAL prop_norm($1, st.prop_id) z
    JOIN payoff po ON po.work_id = $1 AND po.setup_id = st.id
    LEFT JOIN scene sp ON sp.id = po.scene_id AND sp.shown
   WHERE z.base = n.base AND z.val = n.val AND po.at_pt IS NOT NULL AND precedes($1, n.pt, po.at_pt) = 'yes')
SELECT 'act' AS target_kind, n.act_id AS target_id, n.ord AS span_from, min(f.ord) AS span_to,
       jsonb_build_object('prop', n.prop_id, 'oracle', n.actor_id,
                          'fulfilled', jsonb_agg(jsonb_build_object('route', f.route, 'ord', f.ord, 'act', f.by_act)
                                                 ORDER BY f.ord NULLS LAST, f.route)) AS data
  FROM notyet n JOIN f ON f.act_id = n.act_id AND f.prop_id = n.prop_id
 GROUP BY n.act_id, n.prop_id, n.ord, n.actor_id
