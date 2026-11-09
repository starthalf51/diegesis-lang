-- 006: doubt withdraws a belief that was heard (Task 9). Changes 003's derive_beliefs and 005's lint_known.
--   `X doubts P` (point t) — X's belief row for P, acquired by hearing it before t (including at the same point as t), is withdrawn from t:
--   that row's until_pt = t (the earliest such doubt). Heard beliefs = audience rows of assertive utterances (excluding shows).
--   Perception (sees), reading (reads), explicit believes and background knowledge are not withdrawn. A new source after t is a new row, so it is believed again.
--   A background doubts (no point) creates no row and withdraws nothing — it just means not believing from the start.
--   ponytail: only the doubter's own rows are withdrawn (what a group heard stays). If group doubt is needed, widen it with lint_self_and_groups.
SET LOCAL search_path = public, ag_catalog;

ALTER TABLE believes ADD COLUMN until_pt text;

CREATE OR REPLACE FUNCTION derive_beliefs(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  DELETE FROM believes WHERE work_id = w;
  DELETE FROM uses WHERE work_id = w;
  DELETE FROM deception WHERE work_id = w;

  INSERT INTO believes(work_id, entity_id, prop_id, value, from_pt, cause_act, declared, src_id)
  SELECT w, au.entity_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = w AND assertive(c.verb) AND au.entity_id IS DISTINCT FROM c.actor_id
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, false, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'sees' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, true, a.pt, a.id, false, a.src_id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'reads' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, b.entity_id, b.prop_id, true, b.at_pt, (SELECT id FROM act WHERE id = b.because_pt), true, b.src_id
    FROM belief b WHERE b.work_id = w AND b.verb = 'believes' AND b.prop_id IS NOT NULL;

  -- Doubt: for each heard row (audience row, not the speaker, not declared), the earliest doubt after it (precedes yes, same point included).
  UPDATE believes bl SET until_pt = (
      SELECT b.at_pt FROM belief b JOIN point p ON p.id = b.at_pt
       WHERE b.work_id = w AND b.verb = 'doubts' AND b.entity_id = bl.entity_id AND b.at_pt IS NOT NULL
         AND lint_canon(w, b.prop_id) = lint_canon(w, bl.prop_id) AND precedes(w, bl.from_pt, b.at_pt) = 'yes'
       ORDER BY p.earliest, b.src_id LIMIT 1)
    FROM act s
   WHERE bl.work_id = w AND bl.value AND NOT bl.declared AND s.id = bl.cause_act
     AND assertive(s.verb) AND s.verb <> 'shows' AND s.resolved_actor_id IS DISTINCT FROM bl.entity_id;

  INSERT INTO uses(work_id, entity_id, prop_id, at_pt, act_id)
  SELECT w, c.actor_id, c.prop_id, c.pt, c.act_id
    FROM act_content c WHERE c.work_id = w AND assertive(c.verb) AND c.verb <> 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, a.pt, a.id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'writes' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, a.because_id, a.pt, a.id
    FROM act a WHERE a.work_id = w AND a.because_id IS NOT NULL AND a.resolved_actor_id IS NOT NULL;

  INSERT INTO deception(work_id, act_id, liar_id, prop_id)
  SELECT w, c.act_id, c.actor_id, c.prop_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL;
END $$;

-- Add withdrawal to lint_known (005): if until_pt is definitely before at (not the same point), that row is not a source.
-- The same point is the case where the doubt was written after that act (sentence point = previous act's point) → they still believed at the time of that act.
-- If the order is maybe it is not definite, so that row counts at most 1 (info).
CREATE OR REPLACE FUNCTION lint_known(w text, e text, c text, at text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src(pt, declared, until_pt) AS (
    SELECT b.from_pt, b.declared, b.until_pt FROM believes b JOIN lint_self_and_groups(w, e) u ON u.id = b.entity_id
     WHERE b.work_id = w AND b.value AND lint_canon(w, b.prop_id) = c
    UNION ALL
    SELECT x.pt, false, NULL FROM act_content x JOIN lint_self_and_groups(w, e) u ON u.id = x.actor_id
     WHERE x.work_id = w AND x.verb = 'decrees' AND lint_canon(w, x.prop_id) = c),
  graded(g, cut) AS (
    SELECT CASE WHEN pt IS NULL THEN 2
                ELSE CASE precedes(w, pt, at) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END,
           CASE WHEN until_pt IS NULL OR until_pt = at THEN 2
                ELSE CASE precedes(w, until_pt, at) WHEN 'yes' THEN 0 WHEN 'maybe' THEN 1 ELSE 2 END END
      FROM src WHERE pt IS DISTINCT FROM at OR declared)
  SELECT coalesce(max(least(g, cut)), 0) FROM graded $$;
