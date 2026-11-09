-- rule: fact-conflict
-- severity: warn
-- layer: lint
-- target: fact
-- summary: two true facts with the same predicate and first argument but different remaining arguments (e.g. broke(lens, A) and broke(lens, B)), or two text facts with the same leading token in different files (candidate)
--
-- True fact = a predicate row declared as `fact x = pred(args)` (the one an alias points at). `not`, alias rows, and predicate rows made during inference are not counted.
-- Emitted once per pair (on the smaller id side). If both really are true (a relation that allows several), waive it.
WITH pf AS (
  SELECT f.id, f.src_id, f.def->>'pred' AS pred, f.def->'args' AS args
    FROM fact f
   WHERE f.work_id = $1 AND f.def ? 'pred' AND jsonb_array_length(f.def->'args') >= 2
     AND EXISTS (SELECT 1 FROM fact x WHERE x.work_id = $1 AND x.def->>'alias' = f.id))
SELECT 'fact' AS target_kind, a.id AS target_id, a.src_id,
       format('%s conflicts with %s (same predicate and first argument, different rest)', lint_short(a.id), lint_short(b.id)) AS message,
       jsonb_build_object('other', b.id) AS data
  FROM pf a JOIN pf b ON a.pred = b.pred AND a.args->0 = b.args->0 AND a.id < b.id AND a.args <> b.args
UNION ALL
-- Text-fact heuristic (candidate): generated stories use only `fact x "text" true` with no predicate. Two true facts with no predicate definition
--   that are in different files, whose ids share the leading token (before the first `_`, e.g. lens_smashed → lens), and that are both narrated (act content or reader disclosure) are emitted as candidates.
--   The same token inside one file is a bundle of facts written at once, so it is not counted.
--   The leading token alone fired 150 times in one run3 B piece (with 3 shared, 21 times; with 4, 6 of 7 were false positives), so narrow it to pairs that tell the same event differently:
--   the two texts share five or more two-character word prefixes (subjects, actions, agents like lens, smashed, Do Gyeongtae). A cheap approximation meant to be insensitive to particles and conjugation.
SELECT 'fact', a.id, a.src_id,
       format('candidate conflict, check: %s and %s are both true text facts about %s in different files', lint_short(a.id), lint_short(b.id), split_part(lint_short(a.id), '_', 1)),
       jsonb_build_object('other', b.id, 'heuristic', 'text-fact')
  FROM (SELECT f.id, f.src_id, g.file, split_part(lint_short(f.id), '_', 1) AS tok
          FROM fact f JOIN src g ON g.work_id = $1 AND g.id = f.src_id
         WHERE f.work_id = $1 AND f.def IS NULL AND f.truth = 'true' AND position('_' IN lint_short(f.id)) > 1
           AND (EXISTS (SELECT 1 FROM act_content c WHERE c.work_id = $1 AND lint_canon($1, c.prop_id) = f.id)
             OR EXISTS (SELECT 1 FROM reader_events($1) r WHERE r.fact_id = f.id))) a
  JOIN (SELECT f.id, f.src_id, g.file, split_part(lint_short(f.id), '_', 1) AS tok
          FROM fact f JOIN src g ON g.work_id = $1 AND g.id = f.src_id
         WHERE f.work_id = $1 AND f.def IS NULL AND f.truth = 'true' AND position('_' IN lint_short(f.id)) > 1
           AND (EXISTS (SELECT 1 FROM act_content c WHERE c.work_id = $1 AND lint_canon($1, c.prop_id) = f.id)
             OR EXISTS (SELECT 1 FROM reader_events($1) r WHERE r.fact_id = f.id))) b
    ON a.tok = b.tok AND a.id < b.id AND a.file <> b.file
 WHERE (SELECT count(*) FROM (SELECT left(w, 2) FROM regexp_split_to_table((SELECT text FROM fact WHERE id = a.id), '[\s,.''"]+') w WHERE length(w) >= 2
                              INTERSECT
                              SELECT left(w, 2) FROM regexp_split_to_table((SELECT text FROM fact WHERE id = b.id), '[\s,.''"]+') w WHERE length(w) >= 2) z) >= 5
