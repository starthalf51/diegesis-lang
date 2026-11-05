-- rule: inferred-deception
-- severity: info
-- layer: lint
-- target: act
-- summary: 쓴 사람이 거짓인 줄 알고 쓴 것으로 판정된 문서(007) — 작가의 실수인지 확인하라
--
-- 대상: `X writes art` 중 deception 행이 있는 것(007: 쓴 사람이 쓰는 시점에 ¬P 를 확정적으로 안다). 문서 하나에 한 건,
--   거짓으로 본 명제를 모두 적는다. lies 와 달리 작가가 거짓말이라고 쓰지 않았는데 엔진이 추론한 것이라 보여 준다.
--   진짜 거짓 서신이면 그대로 두면 된다(info, 오탐이 아니다). 작가의 실수(쓴 사람이 ¬P 를 알 리 없음)면 원문을 고친다.
SELECT 'act' AS target_kind, a.id AS target_id,
       format('%s writes %s knowing it false (inferred deception: %s)', lint_short(a.resolved_actor_id),
              string_agg(DISTINCT lint_short(art.id), ', '), string_agg(DISTINCT lint_short(d.prop_id), ', ')) AS message,
       jsonb_build_object('entity', a.resolved_actor_id, 'props', jsonb_agg(DISTINCT d.prop_id)) AS data
  FROM act a JOIN deception d ON d.act_id = a.id
  JOIN artifact art ON art.id = ANY (a.objects) AND d.prop_id = ANY (art.contains)
 WHERE a.work_id = $1 AND a.verb = 'writes'
 GROUP BY a.id, a.resolved_actor_id
