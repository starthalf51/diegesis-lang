-- rule: attribute-conflict
-- severity: warn
-- layer: lint
-- target: attribute
-- summary: the same attribute of the same entity is asserted with mutually inconsistent values
--
-- Target: attr assertions outside an account (inside an account is account-variance's job). Point = at_pt, or origin if none (background).
--   Values are compared as (kind, canonical value): id via lint_canon (alias resolution), int normalized by sema (07 ≡ 7), str as is.
-- Order: for two assertions p, q of the same (entity, key), q is later = precedes(p, q) = yes and precedes(q, p) ≠ yes,
--   or if neither strictly precedes, the later source order (src_id). The target is the later assertion q.
-- Violation (key ≠ age, or an age whose value is not an integer): the canonical values differ. A re-asserted `attr` is not an event that changes a value, and
--   the language has no value-changing event now, so a different value is a conflict regardless of point (suppress it if it really did change).
-- Violation (key = age, both int): da = age(q) − age(p), elapsed t = t_q − t_p (years, 1y = 365d).
--   The bounds of t [lo, hi] = [−d(q, p), d(p, q)] (point_dist; unbounded on that side if absent). The two checks are separate (fix 1):
--   hi exists and hi < da − 1 (elapsed too short), or lo exists and lo > da + 1 (elapsed too long) — either way
--   |da − t| > 1 for every t in [lo, hi]. And regardless of bounds, precedes(p, q) = yes and da < 0 (the age decreases).
-- The status key is excluded: gone ↔ present is a value-changing event, and stays-gone reads it per point.
-- The at key is excluded too: objects moving around is normal, and object-location uses the most recent value before the act.
-- When one target has several opposing assertions, the earliest in source is carried as data.other.
WITH
a AS (
  SELECT x.id, x.entity_id, x.key, x.kind, x.value, x.src_id,
         CASE x.kind WHEN 'id' THEN lint_canon($1, x.value) ELSE x.value END AS v,
         coalesce(x.at_pt, (SELECT id FROM point WHERE work_id = $1 AND kind = 'origin' ORDER BY id LIMIT 1)) AS pt
    FROM attribute x WHERE x.work_id = $1 AND x.account_id IS NULL AND x.key NOT IN ('status', 'at')),
pair AS (
  SELECT q.*, p.id AS p_id, p.kind AS p_kind, p.value AS p_value, p.v AS p_v, p.src_id AS p_src, p.pt AS p_pt,
         q.key = 'age' AND q.kind = 'int' AND p.kind = 'int' AS age,
         precedes($1, p.pt, q.pt) AS fwd, precedes($1, q.pt, p.pt) AS bwd
    FROM a p JOIN a q ON q.entity_id = p.entity_id AND q.key = p.key AND q.id <> p.id),
later AS (
  SELECT * FROM pair
   WHERE (fwd = 'yes' AND bwd <> 'yes') OR (NOT (bwd = 'yes' AND fwd <> 'yes') AND p_src < src_id)),
conflict AS (
  SELECT l.* FROM later l
    LEFT JOIN point_dist hi ON hi.work_id = $1 AND hi.src = l.p_pt AND hi.dst = l.pt
    LEFT JOIN point_dist lo ON lo.work_id = $1 AND lo.src = l.pt AND lo.dst = l.p_pt
   CROSS JOIN LATERAL (SELECT CASE WHEN l.age THEN l.v::numeric - l.p_v::numeric END AS da) g
   WHERE (NOT l.age AND (l.kind, l.v) <> (l.p_kind, l.p_v))
      OR (l.age AND hi.d / 31536000.0 < g.da - 1)
      OR (l.age AND -lo.d / 31536000.0 > g.da + 1)
      OR (l.age AND l.fwd = 'yes' AND g.da < 0))
SELECT DISTINCT ON (c.id) 'attribute' AS target_kind, c.id AS target_id, c.src_id,
       format('%s.%s = %s conflicts with %s', lint_short(c.entity_id), c.key, lint_short(c.value), lint_short(c.p_value))
         AS message,
       jsonb_build_object('entity', c.entity_id, 'key', c.key, 'value', c.value, 'kind', c.kind, 'src_id', c.src_id,
                          'other', c.p_id, 'other_value', c.p_value, 'other_kind', c.p_kind, 'other_src_id', c.p_src) AS data
  FROM conflict c
 ORDER BY c.id, c.p_src
