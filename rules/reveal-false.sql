-- rule: reveal-false
-- severity: warn
-- layer: lint
-- target: reveal
-- summary: 거짓으로 선언된 사실을 독자에게 공개한다
--
-- Target: reveal statements (target kind reveal = that scene ID, suppression uses the same key). The same fact in one scene is emitted once.
-- Violation: the revealed proposition is false within the work — render_fact_true judges it following aliases and negation
--   (a fact declared with value false, the negation of a negation, etc.). derived and unknown count as true.
SELECT DISTINCT ON (r.scene_id, r.prop_id) 'reveal' AS target_kind, r.scene_id AS target_id, r.src_id,
       format('reveal of false fact %s', lint_short(r.prop_id)) AS message,
       jsonb_build_object('prop', r.prop_id, 'scene', r.scene_id) AS data
  FROM reveal r
 WHERE r.work_id = $1 AND r.prop_id IS NOT NULL AND NOT render_fact_true($1, r.prop_id)
