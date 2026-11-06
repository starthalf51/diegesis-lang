-- pattern: counter-deception
-- layer: deception
-- summary: 기만의 수신자가 그 명제를 의심하거나 이미 반대를 믿는다 — 속지 않는다
--
-- 기만 D(deception-chain 과 같다): lies 행위의 청중, 또는 거짓 명제를 담은 문서를 읽은 사람이 수신자 r.
-- 매치: r 이 D 뒤(같은 시점 포함, precedes = yes)에 그 기저 사실을 doubts 하거나,
--       D 의 시점에 그 반대를 이미 안다(char_knows(r, 기저, ¬값, D) = yes).
-- 대상 = 기만 행위 D. span = D 의 위치 ~ 가장 늦은 의심의 위치(의심이 없으면 D). data.receivers = (수신자, 방식).
WITH ap AS (SELECT * FROM act_props($1)),
d AS (
  SELECT a.act_id, a.ord, a.pt, a.base, a.val, au.entity_id AS e
    FROM ap a JOIN audience au ON au.act_id = a.act_id
   WHERE a.verb = 'lies' AND au.entity_id IS DISTINCT FROM a.actor_id
  UNION
  SELECT a.act_id, a.ord, a.pt, a.base, a.val, a.actor_id
    FROM ap a WHERE a.verb = 'reads' AND a.actor_id IS NOT NULL AND truth_at($1, a.base, a.pt) = NOT a.val),
c AS (
  SELECT d.act_id, d.ord, d.e, 'doubts' AS how, point_ord($1, b.at_pt) AS at_ord
    FROM d JOIN belief b ON b.work_id = $1 AND b.verb = 'doubts' AND b.entity_id = d.e AND b.at_pt IS NOT NULL
    CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
   WHERE z.base = d.base AND precedes($1, d.pt, b.at_pt) = 'yes'
  UNION
  SELECT d.act_id, d.ord, d.e, 'knows-opposite', d.ord
    FROM d WHERE char_knows($1, d.e, d.base, NOT d.val, d.pt) = 'yes')
SELECT 'act' AS target_kind, c.act_id AS target_id, c.ord AS span_from, coalesce(max(c.at_ord), c.ord) AS span_to,
       jsonb_build_object('receivers', jsonb_agg(DISTINCT jsonb_build_object('receiver', c.e, 'how', c.how))) AS data
  FROM c GROUP BY c.act_id, c.ord
