-- rule: attribute-conflict
-- severity: warn
-- layer: lint
-- target: attribute
-- summary: 같은 엔티티의 같은 속성이 서로 맞지 않는 값으로 단언된다
--
-- 대상: account 밖의 attr 단언(account 안은 account-variance 몫). 시점 = at_pt, 없으면(배경) origin.
--   값 비교는 (kind, 정규값): id 는 lint_canon(별칭 해소), int 는 sema 가 정규화(07 ≡ 7), str 은 그대로.
-- 순서: 같은 (엔티티, 키)의 두 단언 p, q 에서 q 가 나중 = precedes(p, q) = yes 이고 precedes(q, p) ≠ yes,
--   또는 어느 쪽도 엄격히 앞서지 않으면 원문 순서(src_id)가 뒤. 대상은 나중 단언 q 다.
-- 위반(키 ≠ age, 또는 값이 정수가 아닌 age): 정규값이 다르다. `attr` 재단언은 값을 바꾸는 사건이 아니고
--   지금 언어에는 값을 바꾸는 사건이 없으므로, 다른 값은 시점과 무관하게 충돌이다(바뀐 것이 맞으면 억제).
-- 위반(키 = age, 둘 다 int): da = 나이(q) − 나이(p), 경과 t = t_q − t_p (년, 1y = 365d).
--   t 의 경계 [lo, hi] = [−d(q, p), d(p, q)] (point_dist, 없으면 그쪽은 무한). 두 검사는 따로 한다(fix 1):
--   hi 가 있고 hi < da − 1(경과가 너무 짧다), 또는 lo 가 있고 lo > da + 1(경과가 너무 길다) — 어느 쪽이든
--   [lo, hi] 의 모든 t 에서 |da − t| > 1 이다. 그리고 경계와 무관하게 precedes(p, q) = yes 이고 da < 0(나이가 준다).
-- 한 대상에 맞서는 단언이 여럿이면 원문에서 가장 앞선 것 하나를 data.other 로 싣는다.
WITH
a AS (
  SELECT x.id, x.entity_id, x.key, x.kind, x.value, x.src_id,
         CASE x.kind WHEN 'id' THEN lint_canon($1, x.value) ELSE x.value END AS v,
         coalesce(x.at_pt, (SELECT id FROM point WHERE work_id = $1 AND kind = 'origin' ORDER BY id LIMIT 1)) AS pt
    FROM attribute x WHERE x.work_id = $1 AND x.account_id IS NULL),
pair AS (
  SELECT q.*, p.id AS p_id, p.kind AS p_kind, p.value AS p_value, p.v AS p_v, p.src_id AS p_src, p.pt AS p_pt,
         q.key = 'age' AND q.kind = 'int' AND p.kind = 'int' AS age,
         precedes($1, p.pt, q.pt) AS fwd, precedes($1, q.pt, p.pt) AS bwd
    FROM a p JOIN a q ON q.entity_id = p.entity_id AND q.key = p.key AND q.id <> p.id),
later AS (
  SELECT * FROM pair
   WHERE (fwd = 'yes' AND bwd <> 'yes') OR (NOT (bwd = 'yes' AND fwd <> 'yes') AND p_src < src_id)),
conflict AS (
  SELECT l.* FROM later l
    LEFT JOIN point_dist hi ON hi.work_id = $1 AND hi.src = l.p_pt AND hi.dst = l.pt
    LEFT JOIN point_dist lo ON lo.work_id = $1 AND lo.src = l.pt AND lo.dst = l.p_pt
   CROSS JOIN LATERAL (SELECT CASE WHEN l.age THEN l.v::numeric - l.p_v::numeric END AS da) g
   WHERE (NOT l.age AND (l.kind, l.v) <> (l.p_kind, l.p_v))
      OR (l.age AND hi.d / 31536000.0 < g.da - 1)
      OR (l.age AND -lo.d / 31536000.0 > g.da + 1)
      OR (l.age AND l.fwd = 'yes' AND g.da < 0))
SELECT DISTINCT ON (c.id) 'attribute' AS target_kind, c.id AS target_id, c.src_id,
       format('%s.%s = %s conflicts with %s', lint_short(c.entity_id), c.key, lint_short(c.value), lint_short(c.p_value))
         AS message,
       jsonb_build_object('entity', c.entity_id, 'key', c.key, 'value', c.value, 'kind', c.kind, 'src_id', c.src_id,
                          'other', c.p_id, 'other_value', c.p_value, 'other_kind', c.p_kind, 'other_src_id', c.p_src) AS data
  FROM conflict c
 ORDER BY c.id, c.p_src
