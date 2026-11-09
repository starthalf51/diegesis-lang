-- rule: letter-unknown-info
-- severity: warn
-- layer: lint
-- target: act
-- summary: a document holds a proposition the writer did not believe before writing
--
-- Target: for each `X writes art`, one row per proposition P held in art (uses(X, P, time of writing)).
-- Excluded: propositions written while knowing ¬P (deception row, 007 — forged or false letters are lies told in writing).
-- Exemption and verdict are the same as info-flow: lint_exempt, lint_known(X, canonical P, time of writing) → 2 fine, 1 info, 0 warn.
WITH graded AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, art.id AS artifact,
         lint_known($1, u.entity_id, lint_canon($1, u.prop_id), u.at_pt) AS g
    FROM uses u JOIN act a ON a.id = u.act_id
    JOIN artifact art ON art.id = ANY (a.objects) AND u.prop_id = ANY (art.contains)
   WHERE u.work_id = $1 AND a.verb = 'writes' AND NOT lint_exempt($1, u.entity_id)
     AND NOT EXISTS (SELECT 1 FROM deception d WHERE d.act_id = u.act_id AND d.prop_id = u.prop_id))
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s writes %s into %s; the only source may come after the writing (order not determined)'
                     ELSE '%s writes %s into %s without believing it beforehand' END,
              lint_short(entity_id), lint_short(prop_id), lint_short(artifact)) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'artifact', artifact,
                          'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
