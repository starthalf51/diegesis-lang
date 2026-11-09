-- pattern: counter-deception
-- layer: deception
-- summary: the recipient of a deception doubts the proposition or already believes the opposite — is not fooled
--
-- Deception D (same as deception-chain): the audience of a lies act, or the person who read a document holding a false proposition, is recipient r.
-- Match: r doubts that base fact after D (same point included, precedes = yes), or
--       already knows the opposite at D's point (char_knows(r, base, ¬value, D) = yes).
-- Target = deception act D. span = D's position ~ the latest doubt's position (D if no doubt). data.receivers = (recipient, manner).
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
