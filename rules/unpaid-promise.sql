-- rule: unpaid-promise
-- severity: warn
-- layer: lint
-- target: setup
-- summary: a promise setup was not paid off
--
-- Target: setups with kind = promise for which no payoff in the same work points at that setup.
-- hint and symbol do not require payoff. The order of payoffs is covered by payoff-without-setup.
SELECT 'setup' AS target_kind, s.id AS target_id,
       format('promise %s is never paid off', lint_short(s.id)) AS message,
       jsonb_build_object('setup', s.id, 'prop', s.prop_id) AS data
  FROM setup s
 WHERE s.work_id = $1 AND s.kind = 'promise'
   AND NOT EXISTS (SELECT 1 FROM payoff p WHERE p.work_id = $1 AND p.setup_id = s.id)
