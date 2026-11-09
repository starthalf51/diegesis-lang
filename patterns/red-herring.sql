-- pattern: red-herring
-- layer: information
-- summary: the reader believes a false fact for a while, then later receives the opposite (true) — a red herring
--
-- For each change point of reader belief (reader_knows): base fact b is not a state predicate (a state that changes per point is a change, not a correction),
-- its declared truth value is the opposite of the reader's value (truth_at ≠ value), and after it there is a next change point (= the opposite value, i.e. the truth).
-- Target = base fact. span = position where the false belief started ~ the corrected position. Several times for one fact gives several rows.
WITH k AS (
  SELECT r.*, lead(r.from_ord) OVER (PARTITION BY r.fact_id ORDER BY r.from_ord) AS next_ord FROM reader_knows($1) r),
ev AS (SELECT * FROM reader_events($1))
SELECT 'fact' AS target_kind, k.fact_id AS target_id, k.from_ord AS span_from, k.next_ord AS span_to,
       jsonb_build_object('believed', k.value,
                          'told_by', (SELECT jsonb_agg(DISTINCT e.act_id) FROM ev e
                                       WHERE e.fact_id = k.fact_id AND e.ord = k.from_ord AND e.value = k.value),
                          'corrected_by', (SELECT jsonb_agg(DISTINCT e.act_id) FROM ev e
                                            WHERE e.fact_id = k.fact_id AND e.ord = k.next_ord AND e.value <> k.value)) AS data
  FROM k
 WHERE k.next_ord IS NOT NULL AND NOT fact_is_state($1, k.fact_id) AND truth_at($1, k.fact_id, NULL) = NOT k.value
