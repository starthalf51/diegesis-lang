-- rule: pattern-missing
-- severity: warn
-- layer: lint
-- target: pattern
-- summary: a narrative pattern the author declared was not detected in the work
--
-- Target: each `pattern <id> [at <scene>]` declaration (suppression target = that declaration, ID `<scene or module>#<pattern>`).
-- Violation: none of the matches of the same pattern covers the declaration (pattern_covers: a declaration without a scene = any match; with a scene, a match touching that scene).
-- Lint puts only the declared patterns, before the rules, into pattern_match under its own run (lint-…) (internal/pg/lint.go) — only those rows are looked at.
-- A nonexistent pattern ID is caught here too.
SELECT 'pattern' AS target_kind, d.id AS target_id, d.src_id,
       format('pattern %s is declared%s but not found', d.pattern_id,
              CASE WHEN d.scene_id IS NULL THEN '' ELSE ' at ' || lint_short(d.scene_id) END) AS message,
       jsonb_build_object('pattern', d.pattern_id, 'scene', d.scene_id) AS data
  FROM pattern_decl d
 WHERE d.work_id = $1
   AND NOT EXISTS (SELECT 1 FROM pattern_match m
                    WHERE m.work_id = $1 AND m.run_id LIKE 'lint-%' AND m.pattern_id = d.pattern_id
                      AND pattern_covers($1, d.scene_id, m.target_id, m.span_from, m.span_to))
