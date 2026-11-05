-- rule: belief-contradiction
-- severity: warn
-- layer: lint
-- target: act
-- summary: 직접 지각한 사실과 반대되는 명제를 진실하게 주장한다
--
-- 대상: 행위자 X 가 시점 t 에 P 를 진실하게 주장하는 것. 진실한 주장 = 단언 발화(assertive 동사 중 lies·decrees 제외)의
--   내용 명제, 또는 `writes art` 에 담긴 명제. `because P` 는 주장이 아니다.
--   ¬P 를 알고 쓴 명제(deception 행, 007)는 거짓말이지 주장이 아니다.
-- 반대: lint_opposite — `not P` 와 P(별칭은 lint_canon 으로 정규화).
-- 위반: X 자신이 sees 로 얻은 believes(X, ¬P, true)(`sees: P` 와 `sees P` 둘 다)가 있고 그 시점이 t 와 다르며
--   precedes(지각, t) = yes. 들은 것(tells 등)과의 모순, 그룹이 본 것, 선후가 maybe 인 지각은 세지 않는다.
WITH
asserted AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, lint_canon($1, u.prop_id) AS c, u.at_pt
    FROM uses u JOIN act a ON a.id = u.act_id
   WHERE u.work_id = $1
     AND (a.verb = 'writes'
          OR (assertive(a.verb) AND a.verb NOT IN ('lies', 'decrees') AND u.prop_id = ANY (a.prop_id || a."with")))
     AND NOT EXISTS (SELECT 1 FROM deception d WHERE d.act_id = u.act_id AND d.prop_id = u.prop_id)),
perceived AS (
  SELECT b.entity_id, lint_canon($1, b.prop_id) AS c, b.prop_id, b.from_pt, b.cause_act
    FROM believes b JOIN act s ON s.id = b.cause_act AND s.verb = 'sees'
   WHERE b.work_id = $1 AND b.value AND b.from_pt IS NOT NULL)
SELECT DISTINCT ON (a.act_id, a.prop_id)
       'act' AS target_kind, a.act_id AS target_id,
       format('%s asserts %s after seeing %s', lint_short(a.entity_id), lint_short(a.prop_id), lint_short(p.prop_id)) AS message,
       jsonb_build_object('entity', a.entity_id, 'prop', a.prop_id, 'perceived', p.prop_id, 'seen_at', p.cause_act) AS data
  FROM asserted a JOIN perceived p ON p.entity_id = a.entity_id AND lint_opposite($1, a.c, p.c)
 WHERE p.from_pt <> a.at_pt AND precedes($1, p.from_pt, a.at_pt) = 'yes'
 ORDER BY a.act_id, a.prop_id, p.cause_act
