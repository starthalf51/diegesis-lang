-- pattern: deception-chain
-- layer: deception
-- summary: 기만(거짓말 또는 거짓 문서) → 수신자가 믿는다(전해질 수 있다) → 그 믿음을 because 로 쓴 행동
--
-- 기만 D: lies 행위(청중이 수신자), 또는 거짓 명제(truth_at ≠ 담긴 값)를 담은 문서를 읽는 reads 행위(읽은 사람이 수신자).
-- 전달: 수신자가 그 명제를 (거짓말이 아닌) 단언으로 다른 사람에게 말하거나, 그 명제를 담은 문서를 쓰고 다른 사람이
-- 그 문서를 읽으면 그 청중·독자도 수신자다(앞선 수신보다 확실히 뒤, precedes = yes). 재귀로 닫는다.
-- 매치: 행위 Y(because P)의 행위자가 수신자이고 수신이 Y 보다 확실히 앞이다. 명제는 정규화한 (기저, 값)으로 같아야 한다.
-- 대상 = 행동 Y. span = 가장 이른 기만의 위치 ~ Y 의 위치. data.sources = 그 사슬의 기만 행위들.
WITH RECURSIVE ap AS (SELECT * FROM act_props($1)),
src AS (
  SELECT a.act_id AS d_act, a.ord AS d_ord, a.base, a.val, au.entity_id AS e, a.pt AS t
    FROM ap a JOIN audience au ON au.act_id = a.act_id
   WHERE a.verb = 'lies' AND au.entity_id IS DISTINCT FROM a.actor_id
  UNION
  SELECT a.act_id, a.ord, a.base, a.val, a.actor_id, a.pt
    FROM ap a WHERE a.verb = 'reads' AND a.actor_id IS NOT NULL AND truth_at($1, a.base, a.pt) = NOT a.val),
hop AS (
  SELECT x.actor_id AS fe, x.base, x.val, x.pt AS wpt, au.entity_id AS te, x.pt AS tpt
    FROM ap x JOIN audience au ON au.act_id = x.act_id
   WHERE assertive(x.verb) AND x.verb <> 'lies' AND au.entity_id IS DISTINCT FROM x.actor_id
  UNION
  SELECT w.resolved_actor_id, z.base, z.val, w.pt, y.resolved_actor_id, y.pt
    FROM act w JOIN act y ON y.work_id = $1 AND y.verb = 'reads' AND y.objects && w.objects
    JOIN artifact art ON art.id = ANY (w.objects) CROSS JOIN LATERAL unnest(art.contains) p
    CROSS JOIN LATERAL prop_norm($1, p) z
   WHERE w.work_id = $1 AND w.verb = 'writes' AND art.id = ANY (y.objects)
     AND y.resolved_actor_id IS DISTINCT FROM w.resolved_actor_id AND precedes($1, w.pt, y.pt) = 'yes'),
reach AS (
  SELECT * FROM src
  UNION
  SELECT r.d_act, r.d_ord, r.base, r.val, h.te, h.tpt
    FROM reach r JOIN hop h ON h.fe = r.e AND h.base = r.base AND h.val = r.val
   WHERE h.wpt <> r.t AND precedes($1, r.t, h.wpt) = 'yes')
SELECT 'act' AS target_kind, y.id AS target_id, min(r.d_ord) AS span_from,
       (SELECT s.discourse_ord FROM scene s WHERE s.id = y.scene_id AND s.shown) AS span_to,
       jsonb_build_object('prop', y.because_id, 'actor', y.resolved_actor_id,
                          'sources', jsonb_agg(DISTINCT r.d_act)) AS data
  FROM act y CROSS JOIN LATERAL prop_norm($1, y.because_id) z
  JOIN reach r ON r.e = y.resolved_actor_id AND r.base = z.base AND r.val = z.val
 WHERE y.work_id = $1 AND y.because_id IS NOT NULL AND r.t <> y.pt AND precedes($1, r.t, y.pt) = 'yes'
 GROUP BY y.id, y.scene_id, y.because_id, y.resolved_actor_id
