-- rule: info-flow
-- severity: warn
-- layer: lint
-- target: act
-- summary: 인물이 그 전에 알게 된 적 없는 명제를 말한다
--
-- 대상: 진실한 단언 발화(assertive 동사 중 lies·decrees 제외)의 내용 명제마다 uses(화자, P, 행위 시점) 한 줄.
--   writes 는 letter-unknown-info, `because P` 는 because-unbelieved 가 맡는다(겹쳐 내지 않는다).
--   decrees 는 수행문이라 뺀다(선포가 P 를 만든다).
-- 면제: lint_exempt — 화자 또는 화자가 속한 그룹이 knows oracle|omniscient.
-- 판정: lint_known(화자, 정규 P, 행위 시점) (db/migrations/005_lint_common.sql):
--   출처 = 화자·속한 그룹의 believes(P, true) 와 decrees P. 배경 지식(시점 없음)은 처음부터 안다.
--   같은 행위의 출처는 declared 일 때만. 확정(precedes yes) → 정상, maybe 뿐 → info(Review Focus 2), 없음 → warn.
WITH graded AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id,
         lint_known($1, u.entity_id, lint_canon($1, u.prop_id), u.at_pt) AS g
    FROM uses u JOIN act a ON a.id = u.act_id
   WHERE u.work_id = $1 AND assertive(a.verb) AND a.verb NOT IN ('lies', 'decrees')
     AND u.prop_id = ANY (a.prop_id || a."with") AND NOT lint_exempt($1, u.entity_id))
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s says %s; the only source may come after this act (order not determined)'
                     ELSE '%s says %s without having learned it before' END,
              lint_short(entity_id), lint_short(prop_id)) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
