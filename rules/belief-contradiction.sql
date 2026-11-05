-- rule: belief-contradiction
-- severity: warn
-- layer: lint
-- target: act
-- summary: 직접 지각한 사실과 반대되는 명제를 진실하게 주장한다
--
-- 대상: 행위자 X 가 시점 t 에 P 를 진실하게 주장하는 것. 진실한 주장 = 단언 발화(assertive 동사 중 lies·decrees 제외)의
--   내용 명제, 또는 `writes art` 에 담긴 명제. `because P` 는 주장이 아니다.
-- 반대: P 의 부정. `not P` 와 P 는 서로 부정이다(별칭 `fact x = not P` 는 정규 명제로 맞춘다).
-- 위반: X 가 sees 로 얻은 believes(X, ¬P, true) 가 있고 그 시점이 t 와 다르며 precedes(지각, t) = yes.
--   들은 것(tells 등)과의 모순은 세지 않는다(남의 말은 틀릴 수 있다). 선후가 maybe 인 지각도 세지 않는다.
WITH
canon(id, c) AS (SELECT id, coalesce(def->>'alias', id) FROM fact WHERE work_id = $1),
neg(p, n) AS (
  SELECT coalesce(k.c, f.def->>'not'), f.id
    FROM fact f LEFT JOIN canon k ON k.id = f.def->>'not' WHERE f.work_id = $1 AND f.def ? 'not'),
opp(x, y) AS (SELECT p, n FROM neg UNION SELECT n, p FROM neg),
asserted AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, coalesce(k.c, u.prop_id) AS c, u.at_pt
    FROM uses u JOIN act a ON a.id = u.act_id LEFT JOIN canon k ON k.id = u.prop_id
   WHERE u.work_id = $1
     AND (a.verb = 'writes'
          OR (assertive(a.verb) AND a.verb NOT IN ('lies', 'decrees') AND u.prop_id = ANY (a.prop_id || a."with")))),
perceived AS (
  SELECT b.entity_id, coalesce(k.c, b.prop_id) AS c, b.prop_id, b.from_pt, b.cause_act
    FROM believes b JOIN act s ON s.id = b.cause_act AND s.verb = 'sees' LEFT JOIN canon k ON k.id = b.prop_id
   WHERE b.work_id = $1 AND b.value AND b.from_pt IS NOT NULL)
SELECT DISTINCT ON (a.act_id, a.prop_id)
       'act' AS target_kind, a.act_id AS target_id,
       format('%s asserts %s after seeing %s', regexp_replace(a.entity_id, '[^(), ]*/', '', 'g'),
              regexp_replace(a.prop_id, '[^(), ]*/', '', 'g'), regexp_replace(p.prop_id, '[^(), ]*/', '', 'g')) AS message,
       jsonb_build_object('entity', a.entity_id, 'prop', a.prop_id, 'perceived', p.prop_id, 'seen_at', p.cause_act) AS data
  FROM asserted a JOIN opp o ON o.x = a.c JOIN perceived p ON p.entity_id = a.entity_id AND p.c = o.y
 WHERE p.from_pt <> a.at_pt AND precedes($1, p.from_pt, a.at_pt) = 'yes'
 ORDER BY a.act_id, a.prop_id, p.cause_act
