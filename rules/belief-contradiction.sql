-- rule: belief-contradiction
-- severity: warn
-- layer: lint
-- target: act
-- summary: 직접 지각한 사실과 반대되는 명제를 진실하게 주장한다
--
-- Target: actor X truthfully asserting P at point t. A truthful assertion = the content proposition of an assertive utterance (assertive verbs except lies and decrees),
--   or a proposition held in `writes art`. `because P` is not an assertion.
--   A proposition written while knowing ¬P (deception row, 007) is a lie, not an assertion.
-- Opposite: lint_opposite — `not P` and P (aliases normalized with lint_canon).
-- Violation: X has a believes(X, ¬P, true) obtained by X's own sees (both `sees: P` and `sees P`) whose point differs from t and
--   precedes(perception, t) = yes. Contradiction with what was heard (tells etc.), what a group saw, and perceptions with maybe order are not counted.
WITH
asserted AS (
  SELECT DISTINCT u.act_id, u.entity_id, u.prop_id, lint_canon($1, u.prop_id) AS c, u.at_pt
    FROM uses u JOIN act a ON a.id = u.act_id
   WHERE u.work_id = $1
     AND (a.verb = 'writes'
          OR (assertive(a.verb) AND a.verb NOT IN ('lies', 'decrees') AND u.prop_id = ANY (a.prop_id || a."with")))
     AND NOT EXISTS (SELECT 1 FROM deception d WHERE d.act_id = u.act_id AND d.prop_id = u.prop_id)),
perceived AS (
  SELECT b.entity_id, lint_canon($1, b.prop_id) AS c, b.prop_id, b.from_pt, b.cause_act
    FROM believes b JOIN act s ON s.id = b.cause_act AND s.verb = 'sees'
   WHERE b.work_id = $1 AND b.value AND b.from_pt IS NOT NULL)
SELECT DISTINCT ON (a.act_id, a.prop_id)
       'act' AS target_kind, a.act_id AS target_id,
       format('%s asserts %s after seeing %s', lint_short(a.entity_id), lint_short(a.prop_id), lint_short(p.prop_id)) AS message,
       jsonb_build_object('entity', a.entity_id, 'prop', a.prop_id, 'perceived', p.prop_id, 'seen_at', p.cause_act) AS data
  FROM asserted a JOIN perceived p ON p.entity_id = a.entity_id AND lint_opposite($1, a.c, p.c)
 WHERE p.from_pt <> a.at_pt AND precedes($1, p.from_pt, a.at_pt) = 'yes'
 ORDER BY a.act_id, a.prop_id, p.cause_act
