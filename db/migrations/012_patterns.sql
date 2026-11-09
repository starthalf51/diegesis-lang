-- 012: narrative patterns (Plan 2 Task 7). A pattern definition is patterns/*.sql (one file per pattern; the body is a SELECT with $1 = work) and
--   the runner (internal/pg/patterns.go) only copies the results into pattern_match. The verdict is made by the pattern SQL and the 011 reader model.
--   The author declaration `pattern <id> [at <scene>]` is pattern_decl. Declared but no match: lint rule pattern-missing (warn).
--   Matched but not declared: only reported (not a diagnostic).
--   fact.tag: `fact … tag <name>` (R3). tag identity = the identity fact (picked by the recognition pattern).
SET LOCAL search_path = public, ag_catalog;

ALTER TABLE fact ADD COLUMN tag text;

CREATE TABLE pattern_decl (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text NOT NULL, pattern_id text NOT NULL, scene_id text, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- span = discourse_ord range (NULL for a match that touches only unseen scenes). Only the rows of the last run per work are kept.
CREATE TABLE pattern_match (
  run_id text NOT NULL, work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  pattern_id text NOT NULL, target_kind text NOT NULL, target_id text NOT NULL,
  span_from int, span_to int, data jsonb);
CREATE INDEX ON pattern_match (work_id, pattern_id);

-- Whether a declaration covers a match. A declaration with no scene = any match of the same pattern. With a scene: the target is that scene or something inside it
-- (an act or beat `scene#…`), or the match range covers that scene's narration position.
CREATE FUNCTION pattern_covers(w text, scene_id text, target_id text, span_from int, span_to int) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT scene_id IS NULL OR target_id = scene_id OR starts_with(target_id, scene_id || '#')
      OR EXISTS (SELECT 1 FROM scene s WHERE s.work_id = w AND s.id = scene_id AND s.shown
                    AND s.discourse_ord BETWEEN span_from AND span_to) $$;

-- Prophet: self or a group it belongs to has knows oracle (omniscient is not a prophet).
CREATE FUNCTION is_oracle(w text, e text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM entity x JOIN lint_self_and_groups(w, e) u ON u.id = x.id
                  WHERE x.work_id = w AND x.knows = 'oracle') $$;

-- Propositions an act conveys, normalized (including the scene's narration position). reads = propositions the document holds. via = verb.
CREATE FUNCTION act_props(w text) RETURNS TABLE (act_id text, scene_id text, pt text, actor_id text, verb text,
  prop_id text, base text, val bool, ord int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH c AS (
    SELECT a.id, a.scene_id, a.pt, a.resolved_actor_id, a.verb, p
      FROM act a CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") p WHERE a.work_id = w
    UNION ALL
    SELECT a.id, a.scene_id, a.pt, a.resolved_actor_id, a.verb, p
      FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND a.verb IN ('reads', 'writes'))
  SELECT c.id, c.scene_id, c.pt, c.resolved_actor_id, c.verb, c.p, z.base, z.val,
         (SELECT s.discourse_ord FROM scene s WHERE s.id = c.scene_id AND s.shown)
    FROM c CROSS JOIN LATERAL prop_norm(w, c.p) z $$;
