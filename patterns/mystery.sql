-- pattern: mystery
-- layer: information
-- summary: 참인 P 가 독자에게 withhold 되어 있고, 그동안 P 를 묻거나 의심하는 장면이 있다
--
-- withhold 마다: P ≡ (b = v) 가 참(truth_at(b, withhold 장면 시작) = v)이고, 막힌 구간 [withhold 장면 위치, until 위치)
-- 안의 보이는 장면에 그 기저 사실을 내용으로 하는 asks 행위나 그 사실에 대한 doubts 문장이 있다.
-- 대상 = 기저 사실. span = withhold 위치 ~ until 위치(until 이 보이는 장면에 없으면 NULL: 끝내 풀리지 않는다).
WITH h AS (
  SELECT z.base, z.val, coalesce(sd.discourse_ord, 0) AS d_ord, point_ord($1, wh.until_pt) AS u_ord
    FROM withhold wh LEFT JOIN scene sd ON sd.id = wh.scene_id AND sd.shown
    CROSS JOIN LATERAL prop_norm($1, wh.prop_id) z
   WHERE wh.work_id = $1 AND wh.prop_id IS NOT NULL AND truth_at($1, z.base, sd.start_pt) = z.val),
q AS (
  SELECT a.base, a.ord, a.act_id AS ref, 'asks' AS kind FROM act_props($1) a WHERE a.verb = 'asks' AND a.ord IS NOT NULL
  UNION ALL
  SELECT z.base, s.discourse_ord, b.entity_id, 'doubts'
    FROM belief b JOIN scene s ON s.id = b.scene_id AND s.shown CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
   WHERE b.work_id = $1 AND b.verb = 'doubts')
SELECT 'fact' AS target_kind, h.base AS target_id, h.d_ord AS span_from, h.u_ord AS span_to,
       jsonb_build_object('value', h.val,
                          'questions', jsonb_agg(jsonb_build_object('kind', q.kind, 'ord', q.ord, 'ref', q.ref)
                                                 ORDER BY q.ord, q.ref)) AS data
  FROM h JOIN q ON q.base = h.base AND q.ord >= h.d_ord AND (h.u_ord IS NULL OR q.ord < h.u_ord)
 GROUP BY h.base, h.val, h.d_ord, h.u_ord
