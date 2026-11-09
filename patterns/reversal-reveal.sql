-- pattern: reversal-reveal
-- layer: information
-- summary: a fact the reader believed is reversed, and that fact was a setup or the basis (because) of an action
--
-- For each reader-belief change point that has a previous value (= a reversal): that base fact is the proposition of some setup or the because of some act
-- (if the normalized base facts are equal, negations and aliases are the same fact). The truth value is not checked (true→false and false→true are both reversals).
-- Target = base fact. span = position of the previous belief ~ position of the reversal.
WITH k AS (
  SELECT r.*, lag(r.from_ord) OVER (PARTITION BY r.fact_id ORDER BY r.from_ord) AS prev_ord FROM reader_knows($1) r),
used AS (
  SELECT z.base, 'setup' AS kind, s.id AS ref FROM setup s CROSS JOIN LATERAL prop_norm($1, s.prop_id) z
   WHERE s.work_id = $1 AND s.prop_id IS NOT NULL
  UNION ALL
  SELECT z.base, 'because', a.id FROM act a CROSS JOIN LATERAL prop_norm($1, a.because_id) z
   WHERE a.work_id = $1 AND a.because_id IS NOT NULL)
SELECT 'fact' AS target_kind, k.fact_id AS target_id, k.prev_ord AS span_from, k.from_ord AS span_to,
       jsonb_build_object('now', k.value,
                          'used', jsonb_agg(jsonb_build_object('kind', u.kind, 'ref', u.ref) ORDER BY u.ref)) AS data
  FROM k JOIN used u ON u.base = k.fact_id
 WHERE k.prev_ord IS NOT NULL
 GROUP BY k.fact_id, k.prev_ord, k.from_ord, k.value
