-- rule: because-unbelieved
-- severity: warn
-- layer: lint
-- target: act
-- summary: `… because P`, but the actor does not believe P at that point
--
-- Target: acts carrying because P (any verb). Actor = the resolved speaker (skipped if role: speaker is ambiguous; role-ambiguous covers that separately).
-- Exemption and verdict are the same as info-flow: lint_exempt, lint_known(actor, canonical P, act point) → 2 fine, 1 info, 0 warn.
WITH graded AS (
  SELECT a.id AS act_id, a.resolved_actor_id AS entity_id, a.because_id AS prop_id,
         lint_known($1, a.resolved_actor_id, lint_canon($1, a.because_id), a.pt) AS g
    FROM act a
   WHERE a.work_id = $1 AND a.because_id IS NOT NULL AND a.resolved_actor_id IS NOT NULL
     AND NOT lint_exempt($1, a.resolved_actor_id))
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN '%s acts because %s, but may only come to believe it after this act (order not determined)'
                     ELSE '%s acts because %s without believing it at this point' END,
              lint_short(entity_id), lint_short(prop_id)) AS message,
       jsonb_build_object('entity', entity_id, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
