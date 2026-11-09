-- pattern: deception-chain
-- layer: deception
-- summary: deception (a lie or a false document) → the recipient believes (and may pass it on) → an action that uses that belief as its because
--
-- Deception D: a lies act (the audience are recipients), or a reads act on a document holding a false proposition (truth_at ≠ the held value) (the reader is the recipient).
-- Passing on: when a recipient tells the proposition to someone else as an assertion (not a lie), or writes a document holding it and someone else
-- reads that document, that audience or reader is a recipient too (definitely after the earlier receipt, precedes = yes). Closed by recursion.
-- Match: the actor of an act Y (because P) is a recipient and the receipt is definitely before Y. The propositions must be equal as normalized (base, value).
-- Target = action Y. span = the earliest deception's position ~ Y's position. data.sources = the deception acts of that chain.
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
