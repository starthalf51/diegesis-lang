-- pattern: red-herring
-- layer: information
-- summary: 독자가 거짓 사실을 한동안 믿다가 나중에 반대(참)를 받는다 — 붉은 청어
--
-- 독자 믿음의 변화점(reader_knows)마다: 기저 사실 b 가 상태 술어가 아니고(시점마다 바뀌는 상태는 정정이 아니라 변화다)
-- 선언된 진리값이 독자의 값과 반대이며(truth_at ≠ value), 그 뒤에 다음 변화점(= 반대 값, 곧 참)이 있다.
-- 대상 = 기저 사실. span = 거짓을 믿기 시작한 위치 ~ 정정된 위치. 한 사실에 여러 번이면 여러 행.
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
