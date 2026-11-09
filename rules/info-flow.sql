-- rule: info-flow
-- severity: warn
-- layer: lint
-- target: act
-- summary: a character says a proposition they have never come to know before
--
-- Target: for each content proposition of a truthful assertive utterance (assertive verbs except lies and decrees), one uses(speaker, P, act point) line.
--   writes is covered by letter-unknown-info and `because P` by because-unbelieved (not reported twice).
--   decrees is excluded as a performative (the proclamation makes P).
-- Exemption: lint_exempt — the speaker or a group the speaker belongs to has knows oracle|omniscient.
-- Verdict: lint_known(speaker, canonical P, act point) (db/migrations/005_lint_common.sql):
--   Sources = believes(P, true) of the speaker and their groups, and decrees P. Background knowledge (no point) is known from the start.
--   A source from the same act counts only when declared. Definite (precedes yes) → fine, only maybe → info (Review Focus 2), none → warn.
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
