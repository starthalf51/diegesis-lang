-- rule: letter-unknown-info
-- severity: warn
-- layer: lint
-- target: act
-- summary: 쓴 사람이 쓰기 전에 믿지 않은 명제가 문서에 담겼다
--
-- 대상: `X writes art` 마다 art 에 담긴 명제 P 하나씩(uses(X, P, 쓴 시점)).
-- 면제·판정은 info-flow 와 같다: lint_exempt, lint_known(X, 정규 P, 쓴 시점) → 2 정상, 1 info, 0 warn.
WITH graded AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, art.id AS artifact,
         lint_known($1, u.entity_id, lint_canon($1, u.prop_id), u.at_pt) AS g
    FROM uses u JOIN act a ON a.id = u.act_id
    JOIN artifact art ON art.id = ANY (a.objects) AND u.prop_id = ANY (art.contains)
   WHERE u.work_id = $1 AND a.verb = 'writes' AND NOT lint_exempt($1, u.entity_id))
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s writes %s into %s; the only source may come after the writing (order not determined)'
                     ELSE '%s writes %s into %s without believing it beforehand' END,
              lint_short(entity_id), lint_short(prop_id), lint_short(artifact)) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'artifact', artifact,
                          'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
