-- pattern: recognition
-- layer: identity
-- summary: 정체 사실(fact … tag identity)을 믿는 인물 집합이 서술 순서대로 커지고, 적어도 한 번은 sees 로 늘어난다 — 인지(anagnorisis)
--
-- 정체 사실 f(tag identity, 정규화한 (기저, 값)) 마다: 그것을 믿는 believes 행(값까지 같은 것) 중 의심으로 거둬지지 않은 것
-- (until_pt 없음)으로 인물마다 처음 믿게 된 서술 위치를 구한다(배경 지식 = 0, 보이지 않는 장면의 출처는 세지 않는다).
-- 위치 > 0 인 단계(같은 위치에서 새로 믿게 된 인물들)가 둘 이상이고 그중 하나라도 출처가 sees 인 것이 있으면 매치.
-- 대상 = 정체 사실. span = 첫 단계 ~ 마지막 단계. data.steps = 단계마다 (위치, 새 인물, 경로 동사).
WITH idf AS (
  SELECT f.id, z.base, z.val FROM fact f CROSS JOIN LATERAL prop_norm($1, f.id) z WHERE f.work_id = $1 AND f.tag = 'identity'),
b AS (
  SELECT i.id AS fact, x.entity_id,
         CASE WHEN x.from_pt IS NULL THEN 0 ELSE point_ord($1, x.from_pt) END AS ord,
         coalesce(y.verb, CASE WHEN x.declared THEN 'believes' END) AS verb
    FROM believes x CROSS JOIN LATERAL prop_norm($1, x.prop_id) z
    JOIN idf i ON i.base = z.base AND i.val = (z.val = x.value)
    LEFT JOIN act y ON y.id = x.cause_act
   WHERE x.work_id = $1 AND x.until_pt IS NULL),
firsts AS (
  SELECT DISTINCT ON (fact, entity_id) fact, entity_id, ord, verb FROM b WHERE ord IS NOT NULL
   ORDER BY fact, entity_id, ord, (verb = 'sees') DESC),
steps AS (
  SELECT fact, ord, array_agg(entity_id ORDER BY entity_id) AS added, array_agg(DISTINCT verb) AS via,
         bool_or(verb = 'sees') AS by_sight
    FROM firsts WHERE ord > 0 GROUP BY fact, ord)
SELECT 'fact' AS target_kind, fact AS target_id, min(ord) AS span_from, max(ord) AS span_to,
       jsonb_build_object('steps', jsonb_agg(jsonb_build_object('ord', ord, 'added', added, 'via', via) ORDER BY ord)) AS data
  FROM steps GROUP BY fact
HAVING count(*) >= 2 AND bool_or(by_sight)
