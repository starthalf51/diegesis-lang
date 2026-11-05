-- rule: letter-unknown-info
-- severity: warn
-- layer: lint
-- target: act
-- summary: 쓴 사람이 쓰기 전에 믿지 않은 명제가 문서에 담겼다
--
-- 대상: `X writes art` 마다 art 에 담긴 명제 P 하나씩(uses(X, P, 쓴 시점)).
-- 면제·출처·판정은 info-flow 와 같다: 별칭 정규화, believes(X, P, true) 와 X 의 앞선 decrees P,
--   같은 시점의 출처는 declared 일 때만, precedes yes → 정상, maybe 뿐 → info, 없음 → warn.
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
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, coalesce(k.c, u.prop_id) AS c, u.at_pt, art.id AS artifact
    FROM uses u JOIN act a ON a.id = u.act_id
    JOIN artifact art ON art.id = ANY (a.objects) AND u.prop_id = ANY (art.contains)
    LEFT JOIN canon k ON k.id = u.prop_id
   WHERE u.work_id = $1 AND a.verb = 'writes' AND u.entity_id NOT IN (SELECT id FROM exempt)),
graded AS (
  SELECT u.act_id, u.entity_id, u.prop_id, u.artifact,
         max(CASE WHEN l.entity_id IS NULL THEN 0 WHEN l.pt IS NULL THEN 2
                  ELSE CASE precedes($1, l.pt, u.at_pt) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END) AS g
    FROM used u
    LEFT JOIN learned l ON l.entity_id = u.entity_id AND l.c = u.c AND (l.pt IS DISTINCT FROM u.at_pt OR l.declared)
   GROUP BY 1, 2, 3, 4)
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s writes %s into %s; the only source may come after the writing (order not determined)'
                     ELSE '%s writes %s into %s without believing it beforehand' END,
              regexp_replace(entity_id, '[^(), ]*/', '', 'g'), regexp_replace(prop_id, '[^(), ]*/', '', 'g'),
              regexp_replace(artifact, '[^(), ]*/', '', 'g')) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'artifact', artifact,
                          'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
