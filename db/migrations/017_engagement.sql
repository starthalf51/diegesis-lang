-- 017: per-scene engagement metrics (Plan 14 Task 3). Same conventions as 014: every function takes (work text),
--   x axis = discourse_ord of visible scenes, 0 rows for an empty work, no division by zero.
--   novelty = facts first disclosed to the reader (reader_events), payoff = payoffs + released withholds + exposed deceptions,
--   drag = a scene with no acts or nobody on stage. engagement combines them with metric_tension.
SET LOCAL search_path = public, ag_catalog;

INSERT INTO metric_weights VALUES ('eng_tension', 1), ('eng_novelty', 1), ('eng_payoff', 1), ('eng_drag', 1);

-- Facts whose first disclosure to the reader falls in that scene.
CREATE FUNCTION metric_novelty(w text) RETURNS TABLE (ord int, scene text, new_facts int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH f AS (SELECT fact_id, min(e.ord) o FROM reader_events(w) e GROUP BY fact_id)
  SELECT s.discourse_ord, s.id, (SELECT count(*)::int FROM f WHERE f.o = s.discourse_ord)
    FROM scene s WHERE s.work_id = w AND s.shown ORDER BY 1, 2 $$;

-- Payoffs in the scene + withholds whose `until` lands there + deceptions the reader first sees contradicted there.
-- ponytail: an exposed deception = first reader event of the opposite value on the same proposition at or after the lie's scene.
CREATE FUNCTION metric_payoff(w text) RETURNS TABLE (ord int, scene text, payoffs int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH pay AS (
    SELECT coalesce((SELECT x.discourse_ord FROM scene x WHERE x.id = p.scene_id AND x.shown), point_ord(w, p.at_pt)) o
      FROM payoff p WHERE p.work_id = w
    UNION ALL
    SELECT point_ord(w, h.until_pt) FROM withhold h WHERE h.work_id = w AND h.prop_id IS NOT NULL
    UNION ALL
    SELECT (SELECT min(e.ord) FROM reader_events(w) e CROSS JOIN LATERAL prop_norm(w, d.prop_id) z
             WHERE e.fact_id = z.base AND e.value <> z.val AND e.ord >= l.ord)
      FROM deception d CROSS JOIN LATERAL
           (SELECT x.discourse_ord ord FROM act a JOIN scene x ON x.id = a.scene_id AND x.shown WHERE a.id = d.act_id) l
     WHERE d.work_id = w)
  SELECT s.discourse_ord, s.id, (SELECT count(*)::int FROM pay WHERE pay.o = s.discourse_ord)
    FROM scene s WHERE s.work_id = w AND s.shown ORDER BY 1, 2 $$;

CREATE FUNCTION metric_drag(w text) RETURNS TABLE (ord int, scene text, drag int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.discourse_ord, s.id,
         CASE WHEN NOT EXISTS (SELECT 1 FROM act a WHERE a.work_id = w AND a.scene_id = s.id)
                OR NOT EXISTS (SELECT 1 FROM presence p WHERE p.work_id = w AND p.scene_id = s.id) THEN 1 ELSE 0 END
    FROM scene s WHERE s.work_id = w AND s.shown ORDER BY 1, 2 $$;

-- score = w_t·tension_n + w_n·novelty_n + w_p·payoff_n − w_d·drag; *_n = value / that work's maximum (0 if the maximum is 0).
CREATE FUNCTION metric_engagement(w text) RETURNS TABLE (ord int, scene text, module text, score numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH b AS (
    SELECT n.ord, n.scene, m.name module, coalesce(t.value, 0) tn, n.new_facts nn, p.payoffs pn, d.drag dn
      FROM metric_novelty(w) n
      JOIN metric_payoff(w) p ON p.scene = n.scene
      JOIN metric_drag(w) d ON d.scene = n.scene
      JOIN scene s ON s.id = n.scene
      LEFT JOIN module m ON m.id = s.module_id
      LEFT JOIN metric_tension(w) t ON t.ord = n.ord),
  wt AS (SELECT coalesce(max(weight) FILTER (WHERE term = 'eng_tension'), 1) t, coalesce(max(weight) FILTER (WHERE term = 'eng_novelty'), 1) n,
                coalesce(max(weight) FILTER (WHERE term = 'eng_payoff'), 1) p, coalesce(max(weight) FILTER (WHERE term = 'eng_drag'), 1) d
           FROM metric_weights)
  SELECT b.ord, b.scene, b.module,
         round(wt.t * coalesce(b.tn / nullif(max(b.tn) OVER (), 0), 0)
             + wt.n * coalesce(b.nn::numeric / nullif(max(b.nn) OVER (), 0), 0)
             + wt.p * coalesce(b.pn::numeric / nullif(max(b.pn) OVER (), 0), 0) - wt.d * b.dn, 4)
    FROM b CROSS JOIN wt ORDER BY 1, 2 $$;

CREATE FUNCTION metric_module_engagement(w text) RETURNS TABLE (module text, first_ord int, n int, mean numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT e.module, min(e.ord), count(*)::int, round(avg(e.score), 4)
    FROM metric_engagement(w) e GROUP BY e.module ORDER BY 2 $$;
