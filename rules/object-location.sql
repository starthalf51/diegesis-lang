-- rule: object-location
-- severity: warn
-- layer: lint
-- target: act
-- summary: 사물이 있다고 선언된 곳과 다른 장소의 장면에서 그 사물을 다룬다
--
-- 규약: 사물의 위치는 `attr <thing>.at = <place>`, 장면의 장소는 scene.place_id.
-- 대상: 행위의 목적어(does 의 동작 대상 포함) X 가 thing·artifact 이고 장면에 장소가 있는 행위.
-- 시점에 유효한 값: account 밖의 X.at 단언 중 행위보다 반드시 앞서고(precedes = yes; 배경은 origin),
--   그 뒤에 또 다른 X.at 단언이 행위 전에 끼지 않은 것(attribute-conflict 와 같은 "바로 앞" 규칙).
-- 유효한 값이 여럿(순서가 정해지지 않은 단언)이면 모두 후보다: 장면 장소와 같은 것이 하나라도 있으면 조용하다.
-- 위반: 유효한 값이 있고 그중 장면 장소와 같은 것이 하나도 없다. 유효한 값이 없으면 내지 않는다.
WITH origin AS (SELECT id FROM point WHERE work_id = $1 AND kind = 'origin' ORDER BY id LIMIT 1),
loc AS (
  SELECT x.entity_id, x.value, lint_canon($1, x.value) AS v, x.src_id,
         coalesce(x.at_pt, (SELECT id FROM origin)) AS pt
    FROM attribute x
   WHERE x.work_id = $1 AND x.key = 'at' AND x.account_id IS NULL),
cur AS (
  SELECT a.id AS act_id, s.place_id, o.x AS entity_id, l.value, l.v
    FROM act a JOIN scene s ON s.work_id = a.work_id AND s.id = a.scene_id AND s.place_id IS NOT NULL
         CROSS JOIN LATERAL unnest(a.objects) AS o(x)
         JOIN entity e ON e.work_id = a.work_id AND e.id = o.x AND e.kind IN ('thing', 'artifact')
         JOIN loc l ON l.entity_id = o.x AND precedes($1, l.pt, a.pt) = 'yes'
   WHERE a.work_id = $1
     AND NOT EXISTS (SELECT 1 FROM loc q WHERE q.entity_id = l.entity_id AND q.pt <> l.pt
                        AND precedes($1, l.pt, q.pt) = 'yes' AND precedes($1, q.pt, a.pt) = 'yes')),
bad AS (
  SELECT act_id, place_id, entity_id, min(value) AS value FROM cur c
   GROUP BY act_id, place_id, entity_id HAVING bool_and(v <> place_id))
SELECT 'act' AS target_kind, b.act_id AS target_id,
       format('%s %s %s in %s, but %s is at %s', lint_short(a.resolved_actor_id), coalesce(nullif(a.action, ''), a.verb), lint_short(b.entity_id),
              lint_short(b.place_id), lint_short(b.entity_id), lint_short(b.value)) AS message,
       jsonb_build_object('entity', b.entity_id, 'place', b.place_id, 'at', b.value) AS data
  FROM bad b JOIN act a ON a.work_id = $1 AND a.id = b.act_id
