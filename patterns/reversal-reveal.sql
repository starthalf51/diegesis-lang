-- pattern: reversal-reveal
-- layer: information
-- summary: 독자가 믿던 사실이 반대로 뒤집히고, 그 사실이 복선(setup)이나 행동의 근거(because)였다
--
-- 독자 믿음의 변화점 중 앞 값이 있는 것(= 뒤집힘)마다: 그 기저 사실이 어떤 setup 의 명제이거나 어떤 행위의 because 다
-- (정규화한 기저 사실이 같으면 부정·별칭도 같은 사실). 진리값은 보지 않는다(참→거짓, 거짓→참 모두 반전이다).
-- 대상 = 기저 사실. span = 앞 믿음의 위치 ~ 뒤집힌 위치.
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
