-- rule: info-flow
-- severity: warn
-- layer: lint
-- target: act
-- summary: 인물이 그 전에 알게 된 적 없는 명제를 말한다
--
-- 대상: 진실한 단언 발화(assertive 동사 중 lies·decrees 제외)의 내용 명제마다 uses(화자, P, 행위 시점) 한 줄.
--   writes 는 letter-unknown-info, `because P` 는 because-unbelieved 가 맡는다(겹쳐 내지 않는다).
--   decrees 는 수행문이라 뺀다(선포가 P 를 만든다).
-- 면제: 화자가 knows oracle|omniscient 이거나 그런 그룹의 (간접) 구성원.
-- 출처: believes(화자, P, true) 그리고 화자 자신의 앞선 decrees P. 별칭(fact x = …)은 정규 명제로 맞춘다.
--   출처 시점이 행위 시점과 같으면(같은 행위) 작가가 선언한 믿음(declared)일 때만 센다. 출처 시점이 NULL 이면 처음부터 안다.
-- 판정: precedes(출처, 행위) = yes 인 출처가 있으면 정상. maybe 뿐이면 info(Review Focus 2). 없으면 warn.
WITH RECURSIVE
exempt(id) AS (
  SELECT id FROM entity WHERE work_id = $1 AND knows IN ('oracle', 'omniscient')
  UNION SELECT g.member_id FROM group_member g JOIN exempt e ON e.id = g.group_id WHERE g.work_id = $1),
canon(id, c) AS (SELECT id, coalesce(def->>'alias', id) FROM fact WHERE work_id = $1),
learned(entity_id, c, pt, declared) AS (
  SELECT b.entity_id, coalesce(k.c, b.prop_id), b.from_pt, b.declared
    FROM believes b LEFT JOIN canon k ON k.id = b.prop_id WHERE b.work_id = $1 AND b.value
  UNION ALL
  SELECT x.actor_id, coalesce(k.c, x.prop_id), x.pt, false
    FROM act_content x LEFT JOIN canon k ON k.id = x.prop_id
   WHERE x.work_id = $1 AND x.verb = 'decrees' AND x.actor_id IS NOT NULL),
used AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, coalesce(k.c, u.prop_id) AS c, u.at_pt
    FROM uses u JOIN act a ON a.id = u.act_id LEFT JOIN canon k ON k.id = u.prop_id
   WHERE u.work_id = $1 AND assertive(a.verb) AND a.verb NOT IN ('lies', 'decrees')
     AND u.prop_id = ANY (a.prop_id || a."with")
     AND u.entity_id NOT IN (SELECT id FROM exempt)),
graded AS (
  SELECT u.act_id, u.entity_id, u.prop_id,
         max(CASE WHEN l.entity_id IS NULL THEN 0 WHEN l.pt IS NULL THEN 2
                  ELSE CASE precedes($1, l.pt, u.at_pt) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END) AS g
    FROM used u
    LEFT JOIN learned l ON l.entity_id = u.entity_id AND l.c = u.c AND (l.pt IS DISTINCT FROM u.at_pt OR l.declared)
   GROUP BY 1, 2, 3)
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s says %s; the only source may come after this act (order not determined)'
                     ELSE '%s says %s without having learned it before' END,
              regexp_replace(entity_id, '[^(), ]*/', '', 'g'), regexp_replace(prop_id, '[^(), ]*/', '', 'g')) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
