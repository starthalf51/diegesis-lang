-- 007: lies told in writing and doubts about documents read (Task 9, Red Cliffs).
--   1. A document read is also testimony: `X doubts P` also withdraws a P obtained from a document read (reads) before it (same as 006's heard beliefs).
--      Basis: Romance of the Three Kingdoms 47.14-16, Cao Cao reads Huang Gai's letter and promptly suspects it as a "personal injury trick".
--   2. If the writer definitely knows ¬P at the time of writing (lint_known = 2), P in that document is a lie: a deception row.
--      letter-unknown-info and belief-contradiction do not emit such (act, proposition) pairs (the same way lies drops out of info-flow).
--      Basis: 45.115 the forged letters of Cai Mao and Zhang Yun that Zhou Yu fabricated, 47.14 Huang Gai's false surrender letter.
SET LOCAL search_path = public, ag_catalog;

-- Whether e definitely knows the opposite of P (¬P, aliases included) at point at.
CREATE FUNCTION lint_knows_false(w text, e text, p text, at text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM fact f WHERE f.work_id = w
                    AND lint_opposite(w, lint_canon(w, f.id), lint_canon(w, p))
                    AND lint_known(w, e, lint_canon(w, f.id), at) = 2) $$;

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

  -- Doubt: for each heard row (audience row, not the speaker, not declared) and read row, the earliest doubt after it (precedes yes, same point included).
  UPDATE believes bl SET until_pt = (
      SELECT b.at_pt FROM belief b JOIN point p ON p.id = b.at_pt
       WHERE b.work_id = w AND b.verb = 'doubts' AND b.entity_id = bl.entity_id AND b.at_pt IS NOT NULL
         AND lint_canon(w, b.prop_id) = lint_canon(w, bl.prop_id) AND precedes(w, bl.from_pt, b.at_pt) = 'yes'
       ORDER BY p.earliest, b.src_id LIMIT 1)
    FROM act s
   WHERE bl.work_id = w AND bl.value AND NOT bl.declared AND s.id = bl.cause_act
     AND ((assertive(s.verb) AND s.verb <> 'shows' AND s.resolved_actor_id IS DISTINCT FROM bl.entity_id)
          OR s.verb = 'reads');

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
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.id, a.resolved_actor_id, p
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'writes' AND a.resolved_actor_id IS NOT NULL
     AND lint_knows_false(w, a.resolved_actor_id, p, a.pt);
END $$;
