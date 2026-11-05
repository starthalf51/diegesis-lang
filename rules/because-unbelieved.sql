-- rule: because-unbelieved
-- severity: warn
-- layer: lint
-- target: act
-- summary: `… because P` 인데 그 시점에 행위자가 P 를 믿지 않는다
--
-- 대상: because P 가 붙은 행위(동사 무관). 행위자 = 해석된 화자(role: 화자가 모호하면 뺀다. role-ambiguous 가 따로 있다).
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
  SELECT a.id AS act_id, a.resolved_actor_id AS entity_id, a.because_id AS prop_id,
         coalesce(k.c, a.because_id) AS c, a.pt AS at_pt
    FROM act a LEFT JOIN canon k ON k.id = a.because_id
   WHERE a.work_id = $1 AND a.because_id IS NOT NULL AND a.resolved_actor_id IS NOT NULL
     AND a.resolved_actor_id NOT IN (SELECT id FROM exempt)),
graded AS (
  SELECT u.act_id, u.entity_id, u.prop_id,
         max(CASE WHEN l.entity_id IS NULL THEN 0 WHEN l.pt IS NULL THEN 2
                  ELSE CASE precedes($1, l.pt, u.at_pt) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END) AS g
    FROM used u
    LEFT JOIN learned l ON l.entity_id = u.entity_id AND l.c = u.c AND (l.pt IS DISTINCT FROM u.at_pt OR l.declared)
   GROUP BY 1, 2, 3)
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s acts because %s, but may only come to believe it after this act (order not determined)'
                     ELSE '%s acts because %s without believing it at this point' END,
              regexp_replace(entity_id, '[^(), ]*/', '', 'g'), regexp_replace(prop_id, '[^(), ]*/', '', 'g')) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
