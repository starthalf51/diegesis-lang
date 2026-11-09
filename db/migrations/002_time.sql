-- 002: STN solving (solve_time), shortest distance between points (point_dist), ordering verdicts (precedes).
--
-- A time_edge row (from, to, min, max) means min ≤ t_to − t_from ≤ max. As a distance graph:
--   from → to  weight max   (t_to − t_from ≤ max)
--   to → from  weight −min  (t_from − t_to ≤ −min)
-- and the shortest distance d(u,v) is the tightest upper bound on t_v − t_u. A NULL bound has no edge (unbounded).
--   latest(p) = d(origin, p),  earliest(p) = −d(p, origin)
SET LOCAL search_path = public, ag_catalog;

-- Shortest distance over the solving edge set (without suppressed edges and cycle edges). Unreachable pairs have no row (unbounded).
-- The self row (d = 0) is kept too. Lint can use gap_max = d(a,b), gap_min = −d(b,a).
CREATE TABLE point_dist (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  src text NOT NULL, dst text NOT NULL, d int8 NOT NULL, PRIMARY KEY (src, dst));

-- Load the distance graph into the temp table _stn_e. skip lists the time_edge.ord values to leave out.
CREATE FUNCTION stn_edges(w text, with_waived bool, skip int[] DEFAULT '{}') RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  IF to_regclass('pg_temp._stn_e') IS NULL THEN
    CREATE TEMP TABLE _stn_e(u text NOT NULL, v text NOT NULL, wt int8 NOT NULL, ord int NOT NULL);
    CREATE INDEX ON _stn_e(u);
  END IF;
  TRUNCATE _stn_e;
  INSERT INTO _stn_e
    SELECT from_pt, to_pt, max, ord FROM time_edge
     WHERE work_id = w AND max IS NOT NULL AND (with_waived OR NOT waived) AND ord <> ALL (skip)
    UNION ALL
    SELECT to_pt, from_pt, -min, ord FROM time_edge
     WHERE work_id = w AND min IS NOT NULL AND (with_waived OR NOT waived) AND ord <> ALL (skip);
  ANALYZE _stn_e;
END $$;

-- Negative-cycle detection: Bellman-Ford over _stn_e with every point starting at 0 (virtual source). The result goes to _stn_d.
-- Returns NULL = converged (no negative cycle); otherwise a point that still shrank on the V-th iteration.
CREATE FUNCTION stn_negcycle() RETURNS text
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE nv int; n int; x text;
BEGIN
  IF to_regclass('pg_temp._stn_d') IS NULL THEN
    CREATE TEMP TABLE _stn_d(pt text PRIMARY KEY, dist int8, pred int, pred_pt text);
  END IF;
  TRUNCATE _stn_d;
  INSERT INTO _stn_d(pt, dist) SELECT p, 0 FROM (SELECT u FROM _stn_e UNION SELECT v FROM _stn_e) s(p);
  SELECT count(*) INTO nv FROM _stn_d;
  FOR i IN 1..nv LOOP
    WITH c AS (
      SELECT DISTINCT ON (e.v) e.v, d.dist + e.wt AS nd, e.ord, e.u
        FROM _stn_e e JOIN _stn_d d ON d.pt = e.u
       ORDER BY e.v, d.dist + e.wt, e.ord),
    up AS (
      UPDATE _stn_d t SET dist = c.nd, pred = c.ord, pred_pt = c.u
        FROM c WHERE t.pt = c.v AND c.nd < t.dist
      RETURNING t.pt)
    SELECT count(*), min(pt) INTO n, x FROM up;
    IF n = 0 THEN
      RETURN NULL;
    END IF;
  END LOOP;
  RETURN x;
END $$;

-- From the point x returned by stn_negcycle, walk predecessors V times to land on the cycle, then collect the time_edge.ord values of one lap.
CREATE FUNCTION stn_cycle(x text) RETURNS int[]
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE nv int; y text; o int; out int[] := '{}';
BEGIN
  SELECT count(*) INTO nv FROM _stn_d;
  FOR i IN 1..nv LOOP
    SELECT pred_pt INTO y FROM _stn_d WHERE pt = x;
    EXIT WHEN y IS NULL;
    x := y;
  END LOOP;
  y := x;
  LOOP
    SELECT pred, pred_pt INTO o, y FROM _stn_d WHERE pt = y;
    EXIT WHEN o IS NULL;
    out := out || o;
    EXIT WHEN y = x OR cardinality(out) > nv;
  END LOOP;
  RETURN out;
END $$;

-- Order cycles: for every strongly connected component where min ≥ 0 edges run in their original direction, the list of edge ords inside it.
-- ponytail: the closure is O(V²) pairs. With tens of thousands of points, switch to a PL/pgSQL Tarjan.
CREATE FUNCTION order_sccs(w text, with_waived bool) RETURNS SETOF int[]
LANGUAGE sql SET search_path = public, ag_catalog AS $$
  WITH RECURSIVE pe AS (
    SELECT ord, from_pt AS a, to_pt AS b FROM time_edge
     WHERE work_id = w AND min >= 0 AND from_pt <> to_pt AND (with_waived OR NOT waived)),
  r(a, b) AS (SELECT a, b FROM pe UNION SELECT r.a, pe.b FROM r JOIN pe ON pe.a = r.b),
  scc AS (SELECT r1.a AS pt, min(r1.b) AS c FROM r r1 JOIN r r2 ON r2.a = r1.b AND r2.b = r1.a GROUP BY r1.a)
  SELECT array_agg(e.ord ORDER BY e.ord)
    FROM pe e JOIN scc s1 ON s1.pt = e.a JOIN scc s2 ON s2.pt = e.b AND s2.c = s1.c
   GROUP BY s1.c
$$;

-- One time-cycle diagnostic per edge bundle. waived if any of its edges is suppressed.
CREATE FUNCTION time_cycle_diag(w text, ords int[]) RETURNS void
LANGUAGE sql SET search_path = public, ag_catalog AS $$
  INSERT INTO diagnostic(run_id, work_id, rule_id, severity, target_kind, src_id, message, data, waived)
  SELECT 'derive', w, 'time-cycle', 'warn', 'when', min(src_id),
         format('time constraints form a cycle (%s constraints)', count(*)),
         jsonb_build_object(
           'src_ids', coalesce(jsonb_agg(DISTINCT src_id ORDER BY src_id) FILTER (WHERE src_id IS NOT NULL), '[]'),
           'edges', jsonb_agg(ord ORDER BY ord),
           'points', (SELECT jsonb_agg(DISTINCT p ORDER BY p) FROM time_edge, LATERAL (VALUES (from_pt), (to_pt)) v(p)
                       WHERE work_id = w AND ord = ANY (ords))),
         bool_or(waived)
    FROM time_edge WHERE work_id = w AND ord = ANY (ords)
$$;

-- Fill point_dist from the shortest distances from all points over _stn_e (set-at-a-time Bellman-Ford that relaxes only changed rows,
-- all sources at once). The iteration count is the max edge count of a shortest path. On a negative cycle returns false and leaves no rows.
-- ponytail: the row count is O(V²) (a 500-point chain ≈ 125k rows). Past a few thousand points, compute only the needed pairs.
-- enable_hashjoin off: the planner turns NOT EXISTS into a whole-table hash anti-join and reads everything each iteration. An index lookup is right.
CREATE FUNCTION stn_apsp(w text) RETURNS bool
LANGUAGE plpgsql SET search_path = public, ag_catalog SET enable_hashjoin = off AS $$
DECLARE nv int; cnt int;
BEGIN
  DELETE FROM point_dist WHERE work_id = w;
  -- Compute on integer numbers (comparing and indexing long text IDs is several times slower).
  IF to_regclass('pg_temp._apsp') IS NULL THEN
    CREATE TEMP TABLE _apsp_n(n int PRIMARY KEY, id text UNIQUE);
    CREATE TEMP TABLE _apsp_e(u int, v int, wt int8);
    CREATE INDEX ON _apsp_e(u);
    CREATE TEMP TABLE _apsp(src int, dst int, d int8, it int);
    CREATE INDEX ON _apsp(src, dst, d);
    CREATE INDEX ON _apsp(it);
  END IF;
  TRUNCATE _apsp_n, _apsp_e, _apsp;
  INSERT INTO _apsp_n SELECT row_number() OVER (ORDER BY id), id FROM point WHERE work_id = w;
  INSERT INTO _apsp_e SELECT a.n, b.n, e.wt FROM _stn_e e JOIN _apsp_n a ON a.id = e.u JOIN _apsp_n b ON b.id = e.v;
  INSERT INTO _apsp SELECT n, n, 0, 0 FROM _apsp_n;
  SELECT count(*) INTO nv FROM _apsp_n;
  -- Append a row each time a pair improves (it = iteration number). The next iteration relaxes only the rows just added (the delta).
  -- No updates or deletes, so no dead tuples pile up. At the end, move each pair's minimum.
  FOR i IN 1..nv + 1 LOOP
    -- EXECUTE: the table grows, so re-plan every iteration.
    EXECUTE 'WITH c AS (
      SELECT x.src, e.v AS dst, min(x.d + e.wt) AS d
        FROM _apsp x JOIN _apsp_e e ON e.u = x.dst WHERE x.it = $1 - 1 GROUP BY 1, 2)
    INSERT INTO _apsp
    SELECT src, dst, d, $1 FROM c
     WHERE NOT EXISTS (SELECT 1 FROM _apsp q WHERE q.src = c.src AND q.dst = c.dst AND q.d <= c.d)' USING i;
    GET DIAGNOSTICS cnt = ROW_COUNT;
    IF cnt = 0 THEN
      INSERT INTO point_dist(work_id, src, dst, d)
      SELECT w, a.id, b.id, x.d FROM (SELECT src, dst, min(d) AS d FROM _apsp GROUP BY src, dst) x
        JOIN _apsp_n a ON a.n = x.src JOIN _apsp_n b ON b.n = x.dst;
      RETURN true;
    END IF;
  END LOOP;
  RETURN false;
END $$;

-- solve_time:
-- (1) Cycle detection. For order cycles, first find strongly connected components from unsuppressed edges only and emit unsuppressed diagnostics. Then, from components
--     built with suppressed edges added, remove the edges already reported and emit the rest as suppressed diagnostics (so one suppression cannot hide a real cycle in the same component).
--     Other negative cycles: one at a time with Bellman-Ford (up to 20), removing the edges found and retrying.
-- (2) Solving edges = edges that are not suppressed and do not belong to an unsuppressed cycle from (1) (a cycle with no suppressed edge). These fill point_dist and earliest/latest.
--     So a contradiction in one place does not erase the bounds of unrelated points. If a negative cycle still remains (over 20), everything is NULL.
CREATE FUNCTION solve_time(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE seen int[] := '{}'; skip int[] := '{}'; cyc int[]; rest int[]; x text; origin text;
BEGIN
  UPDATE point SET earliest = NULL, latest = NULL WHERE work_id = w;
  DELETE FROM diagnostic WHERE work_id = w AND run_id = 'derive' AND rule_id = 'time-cycle';

  -- seen: edges already reported (excluded from detection). skip: edges to drop from solving = edges of cycles that have no suppressed edge.
  -- A cycle containing a suppressed edge breaks once that edge is removed, so the remaining edges stay in the solve (R1).
  FOR cyc IN SELECT order_sccs(w, false) LOOP
    PERFORM time_cycle_diag(w, cyc);
    seen := seen || cyc;
    skip := skip || cyc;
  END LOOP;
  FOR cyc IN SELECT order_sccs(w, true) LOOP
    rest := ARRAY(SELECT unnest(cyc) EXCEPT SELECT unnest(seen) ORDER BY 1);
    IF cardinality(rest) > 0 THEN
      PERFORM time_cycle_diag(w, rest);
      seen := seen || rest;
    END IF;
  END LOOP;

  FOR k IN 1..20 LOOP
    PERFORM stn_edges(w, true, seen);
    x := stn_negcycle();
    EXIT WHEN x IS NULL;
    cyc := stn_cycle(x);
    EXIT WHEN cardinality(cyc) = 0;
    PERFORM time_cycle_diag(w, cyc);
    seen := seen || cyc;
    IF NOT EXISTS (SELECT 1 FROM time_edge WHERE work_id = w AND ord = ANY (cyc) AND waived) THEN
      skip := skip || cyc;
    END IF;
  END LOOP;

  PERFORM stn_edges(w, false, skip);
  IF NOT stn_apsp(w) THEN
    RETURN;
  END IF;
  SELECT id INTO origin FROM point WHERE work_id = w AND kind = 'origin' ORDER BY id LIMIT 1;
  UPDATE point p SET latest = d.d FROM point_dist d WHERE p.work_id = w AND d.src = origin AND d.dst = p.id;
  UPDATE point p SET earliest = -d.d FROM point_dist d WHERE p.work_id = w AND d.dst = origin AND d.src = p.id;
END $$;

-- precedes(work, a, b): 'yes' if a ≤ b holds in every solution, 'no' if b ≤ a does, otherwise 'maybe'.
-- Decided from the distances of the solving edge set: d(b, a) ≤ 0 guarantees t_a − t_b ≤ 0 (yes); d(a, b) ≤ 0 gives no.
-- In an STN this verdict is complete (min ≥ 0 paths and latest(a) ≤ earliest(b) are included). The solved-bounds comparison goes first.
-- Read-only (STABLE): derive pre-fills point_dist. With no distance (unsolvable due to a contradiction) the answer is 'maybe'.
CREATE FUNCTION precedes(w text, a text, b text) RETURNS text
LANGUAGE sql STABLE STRICT SET search_path = public, ag_catalog AS $$
  SELECT CASE
    WHEN a = b THEN 'yes'
    WHEN EXISTS (SELECT 1 FROM point pa, point pb WHERE pa.id = a AND pb.id = b AND pa.work_id = w AND pb.work_id = w
                    AND pa.latest <= pb.earliest) THEN 'yes'
    WHEN EXISTS (SELECT 1 FROM point pa, point pb WHERE pa.id = a AND pb.id = b AND pa.work_id = w AND pb.work_id = w
                    AND pb.latest <= pa.earliest) THEN 'no'
    WHEN EXISTS (SELECT 1 FROM point_dist WHERE src = b AND dst = a AND work_id = w AND d <= 0) THEN 'yes'
    WHEN EXISTS (SELECT 1 FROM point_dist WHERE src = a AND dst = b AND work_id = w AND d <= 0) THEN 'no'
    ELSE 'maybe' END
$$;

-- Form called with point IDs only (the rule example in checks.md). The work is found from a's row.
CREATE FUNCTION precedes(a text, b text) RETURNS text
LANGUAGE sql STABLE STRICT SET search_path = public, ag_catalog AS $$
  SELECT precedes((SELECT work_id FROM point WHERE id = a), a, b)
$$;
