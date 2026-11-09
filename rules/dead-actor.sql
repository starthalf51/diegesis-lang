-- rule: dead-actor
-- severity: warn
-- layer: lint
-- target: act
-- summary: 죽은 인물이 말하거나 행동한다
--
-- Target: acts whose actor (the resolved speaker; the audience is not looked at) X has a `dies X` point @X.died.
-- Violation: the death is necessarily before the act — precedes(@X.died, act) = yes and precedes(act, @X.died) ≠ yes
--   (if they may be at the same instant it is read as a dying word). maybe is not emitted.
-- Exemption: lint_exempt — knows oracle|omniscient (ghosts etc.) and members of such groups. Other cases are handled by suppression.
SELECT 'act' AS target_kind, a.id AS target_id,
       format('%s %s after dying', lint_short(a.resolved_actor_id), a.verb) AS message,
       jsonb_build_object('entity', a.resolved_actor_id, 'died', d.id) AS data
  FROM act a JOIN point d ON d.work_id = a.work_id AND d.id = a.resolved_actor_id || '.died' AND d.kind = 'died'
 WHERE a.work_id = $1 AND NOT lint_exempt($1, a.resolved_actor_id)
   AND precedes($1, d.id, a.pt) = 'yes' AND precedes($1, a.pt, d.id) <> 'yes'
