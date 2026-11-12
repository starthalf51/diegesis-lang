-- 018: structural-beauty metrics (Plan 14 Task 5). Same conventions as 014/017: takes (work text), only visible scenes, no division by zero.
--   bookend = Jaccard(front, back) - Jaccard(front, middle) over the set of characters on stage, places and facts asserted
--   (shown scenes in discourse_ord order). n scenes, window w = max(1, floor(n*0.1)); front = ranks 1..w, back = n-w+1..n,
--   middle = w scenes centred at rank floor(n/2). If front and back overlap (n < 2w) no bookend row is emitted.
--   payoff_ratio = setups with at least one payoff / all setups (no row without setups).
SET LOCAL search_path = public, ag_catalog;

CREATE FUNCTION metric_structure(w text) RETURNS TABLE (term text, value numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH sc AS (SELECT s.id, s.place_id, row_number() OVER (ORDER BY s.discourse_ord, s.id)::int r, count(*) OVER ()::int n
                FROM scene s WHERE s.work_id = w AND s.shown),
  win AS (SELECT n, greatest(1, floor(n * 0.1)::int) k FROM sc LIMIT 1),
  -- stable identities: character, place, base fact (prop_norm) so a fact asserted twice counts once
  el AS (
    SELECT sc.r, 'c:' || p.entity_id e FROM sc JOIN presence p ON p.work_id = w AND p.scene_id = sc.id
    UNION SELECT sc.r, 'p:' || sc.place_id FROM sc WHERE sc.place_id IS NOT NULL
    UNION SELECT sc.r, 'f:' || z.base FROM sc JOIN act a ON a.work_id = w AND a.scene_id = sc.id
      JOIN act_content c ON c.act_id = a.id AND assertive(c.verb)
      CROSS JOIN LATERAL prop_norm(w, c.prop_id) z),
  part AS (SELECT 'front' g, sc.r FROM sc, win WHERE sc.r <= win.k
           UNION ALL SELECT 'back', sc.r FROM sc, win WHERE sc.r > win.n - win.k
           UNION ALL SELECT 'mid', sc.r FROM sc, win WHERE sc.r >= win.n / 2 - win.k / 2 AND sc.r < win.n / 2 - win.k / 2 + win.k),
  gs AS (SELECT p.g, el.e FROM part p JOIN el ON el.r = p.r GROUP BY p.g, el.e),
  -- Jaccard = |A and B| / |A or B| per pair; an empty union gives no row (read as 0 below)
  jac AS (SELECT v.b, count(*) FILTER (WHERE u.ina AND u.inb)::numeric / count(*) j
            FROM (VALUES ('front', 'back'), ('front', 'mid')) v(a, b)
            CROSS JOIN LATERAL (SELECT bool_or(g = v.a) ina, bool_or(g = v.b) inb FROM gs WHERE g IN (v.a, v.b) GROUP BY e) u
           GROUP BY v.b)
  SELECT 'bookend', round(coalesce((SELECT j FROM jac WHERE b = 'back'), 0) - coalesce((SELECT j FROM jac WHERE b = 'mid'), 0), 4)
    WHERE EXISTS (SELECT 1 FROM win WHERE n >= 2 * k)
  UNION ALL
  SELECT 'payoff_ratio', round(count(*) FILTER (WHERE EXISTS (SELECT 1 FROM payoff p WHERE p.work_id = w AND p.setup_id = s.id))::numeric / count(*), 4)
    FROM setup s WHERE s.work_id = w HAVING count(*) > 0 $$;
