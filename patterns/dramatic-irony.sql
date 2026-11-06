-- pattern: dramatic-irony
-- layer: information
-- summary: 독자는 참인 P 를 알고, 같은 장면에 있는 인물은 P 를 모르거나 반대로 믿는다(장면 단위)
--
-- 보이는 장면 s 마다: 독자가 s 앞(서술 위치 < s)까지 믿게 된 (b, v) 중 s 의 시작에 참인 것(truth_at = v),
-- 그 사실이 s 에서 다뤄지고(s 의 행위가 전하거나 묻거나, s 안의 믿음·의심 문장이 그 사실이다 — 무관한 사실은 세지 않는다),
-- s 에 등장하는(presence) 인물 중 s 의 시작에 그것을 확실히 알지 못하는(char_knows = no | opp: 출처가 없거나 반대로 믿는다.
-- 순서가 정해지지 않은 출처만 있으면(maybe) 세지 않는다) 사람이 있다.
-- 예언자·전지(lint_exempt)는 빼고, 그 사실을 앞서 말하거나 생각한 인물은 안다(char_knows 의 출처).
-- 대상 = 장면. span = 그 장면 한 칸. data.irony = (인물, 사실, 값, 인물의 앎).
WITH kn AS (SELECT * FROM reader_knows($1)),
sc AS (SELECT s.id, s.discourse_ord AS ord, s.start_pt FROM scene s WHERE s.work_id = $1 AND s.shown),
k AS (
  SELECT sc.id AS scene_id, sc.ord, sc.start_pt, r.fact_id, r.value
    FROM sc CROSS JOIN LATERAL (SELECT DISTINCT ON (kn.fact_id) kn.fact_id, kn.value FROM kn
                                 WHERE kn.from_ord < sc.ord ORDER BY kn.fact_id, kn.from_ord DESC) r
   WHERE truth_at($1, r.fact_id, sc.start_pt) = r.value),
inplay AS (
  SELECT a.scene_id, a.base FROM act_props($1) a
  UNION
  SELECT b.scene_id, z.base FROM belief b CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
   WHERE b.work_id = $1 AND b.scene_id IS NOT NULL AND b.prop_id IS NOT NULL),
ir AS (
  SELECT k.scene_id, k.ord, k.fact_id, k.value, p.entity_id,
         char_knows($1, p.entity_id, k.fact_id, k.value, k.start_pt) AS knows
    FROM k JOIN inplay i ON i.scene_id = k.scene_id AND i.base = k.fact_id
    JOIN (SELECT DISTINCT entity_id, scene_id FROM presence WHERE work_id = $1) p ON p.scene_id = k.scene_id
   WHERE NOT lint_exempt($1, p.entity_id))
SELECT 'scene' AS target_kind, scene_id AS target_id, ord AS span_from, ord AS span_to,
       jsonb_build_object('irony', jsonb_agg(jsonb_build_object('character', entity_id, 'fact', fact_id, 'value', value,
                                                                'character_knows', knows) ORDER BY entity_id, fact_id)) AS data
  FROM ir WHERE knows IN ('no', 'opp')
 GROUP BY scene_id, ord
