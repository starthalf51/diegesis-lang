-- rule: undelivered-message
-- severity: warn
-- layer: lint
-- target: act
-- summary: `sends m to t with P` 의 전갈이 t 에게 전해지지 않았다
--
-- 대상: sends 행위의 받는 사람 t(to 목록)와 내용 명제 P(prop_id ∪ with) 한 쌍씩.
--   (무대 밖 sends 의 청중은 전령뿐이다. t 는 전달로만 안다.)
-- 전달: 전령(objects 의 누구든)이 단언 발화(sends 제외, lies 포함 — 거짓이어도 전해지긴 했다)로 P 를 말하고
--   t 가 그 청중에 있는 다른 행위. 별칭은 lint_canon 으로 맞춘다.
-- 판정: precedes(sends, 전달) = yes 인 전달이 있으면 정상, maybe 뿐이면 info, 없으면 warn.
WITH
msg AS (
  SELECT s.id AS act_id, s.pt, s.objects AS messengers, t.id AS recipient, x.prop_id, lint_canon($1, x.prop_id) AS c
    FROM act s CROSS JOIN LATERAL unnest(s."to") t(id) JOIN act_content x ON x.act_id = s.id
   WHERE s.work_id = $1 AND s.verb = 'sends'),
told AS (
  SELECT r.act_id, r.pt, r.actor_id, lint_canon($1, r.prop_id) AS c, au.entity_id AS hearer
    FROM act_content r JOIN audience au ON au.act_id = r.act_id
   WHERE r.work_id = $1 AND assertive(r.verb) AND r.verb <> 'sends'),
graded AS (
  SELECT m.act_id, m.recipient, m.prop_id,
         max(CASE WHEN d.act_id IS NULL THEN 0
                  ELSE CASE precedes($1, m.pt, d.pt) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END) AS g
    FROM msg m
    LEFT JOIN told d ON d.actor_id = ANY (m.messengers) AND d.c = m.c AND d.hearer = m.recipient AND d.act_id <> m.act_id
   GROUP BY 1, 2, 3)
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN 'message %s to %s may be delivered only before it was sent (order not determined)'
                     ELSE 'message %s to %s is never delivered' END,
              lint_short(prop_id), lint_short(recipient)) AS message,
       jsonb_build_object('recipient', recipient, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
