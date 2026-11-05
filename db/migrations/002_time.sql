-- 002: STN 풀이(solve_time), 시점 간 최단 거리(point_dist), 선후 판정(precedes).
--
-- time_edge 한 행 (from, to, min, max) 은 min ≤ t_to − t_from ≤ max 다. 거리 그래프로 바꾸면
--   from → to  가중치 max   (t_to − t_from ≤ max)
--   to → from  가중치 −min  (t_from − t_to ≤ −min)
-- 이고, 최단 거리 d(u,v) 는 t_v − t_u 의 가장 빡빡한 상한이다. NULL 경계는 간선이 없다(무한).
--   latest(p) = d(origin, p),  earliest(p) = −d(p, origin)
SET LOCAL search_path = public, ag_catalog;

-- 풀이 간선 집합(억제 간선과 순환 간선을 뺀 것)에서의 최단 거리. 닿지 않는 쌍은 행이 없다(무한).
-- 자기 자신(d = 0) 행도 둔다. 린트가 gap_max = d(a,b), gap_min = −d(b,a) 로 쓸 수 있다.
CREATE TABLE point_dist (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  src text NOT NULL, dst text NOT NULL, d int8 NOT NULL, PRIMARY KEY (src, dst));

-- 거리 그래프를 임시 테이블 _stn_e 에 싣는다. skip 은 빼고 싶은 time_edge.ord 들.
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

-- 음수 사이클 검출: 모든 점을 0 에서 시작하는(가상 출발점) Bellman-Ford over _stn_e. 결과는 _stn_d.
-- 반환: NULL = 수렴(음수 사이클 없음). 아니면 V 번째 반복에서도 줄어든 점.
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

-- stn_negcycle 이 돌려준 점 x 에서 선행자를 V 번 거슬러 사이클에 들어간 뒤, 한 바퀴의 time_edge.ord 를 모은다.
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

-- 순서 순환: min ≥ 0 간선이 원래 방향으로 도는 강연결요소마다 그 안의 간선 ord 목록.
-- ponytail: 닫힘은 O(V²) 쌍이다. 시점이 수만 개면 PL/pgSQL Tarjan 으로 바꾼다.
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

-- 간선 묶음 하나로 time-cycle 진단 한 건. 그 중 하나라도 억제됐으면 waived.
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

-- 모든 점에서의 최단 거리를 _stn_e 로 point_dist 에 채운다(변한 행만 다시 완화하는 집합 단위 Bellman-Ford,
-- 모든 출발점을 한꺼번에). 반복 횟수는 최단 경로의 최대 간선 수. 음수 사이클이면 false 이고 행을 남기지 않는다.
-- ponytail: 행 수가 O(V²) 다(사슬 500 시점 ≈ 12.5만 행). 수천 시점을 넘으면 필요한 쌍만 계산한다.
-- enable_hashjoin off: 계획기가 NOT EXISTS 를 표 전체 해시 반조인으로 잡아 반복마다 전부 읽는다. 색인 조회가 맞다.
CREATE FUNCTION stn_apsp(w text) RETURNS bool
LANGUAGE plpgsql SET search_path = public, ag_catalog SET enable_hashjoin = off AS $$
DECLARE nv int; cnt int;
BEGIN
  DELETE FROM point_dist WHERE work_id = w;
  -- 계산은 정수 번호로 한다(긴 텍스트 ID 를 비교·색인하면 몇 배 느리다).
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
  -- 쌍마다 개선될 때마다 행을 덧붙인다(it = 반복 번호). 다음 반복은 방금 붙은 행(델타)만 완화한다.
  -- 수정·삭제가 없어서 죽은 튜플이 쌓이지 않는다. 끝나면 쌍마다 최솟값을 옮긴다.
  FOR i IN 1..nv + 1 LOOP
    -- EXECUTE: 표가 자라므로 반복마다 다시 계획한다.
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
-- (1) 순환 검출. 순서 순환은 먼저 억제되지 않은 간선만으로 강연결요소를 구해 미억제 진단을 낸다. 그다음 억제 간선까지
--     넣어 구한 요소에서 이미 보고한 간선을 뺀 나머지를 억제된 진단으로 낸다(억제 하나가 같은 요소의 진짜 순환을 덮지 않게).
--     그 밖의 음수 사이클은 Bellman-Ford 로 한 건씩(최대 20건), 찾은 간선을 빼고 다시.
-- (2) 풀이 간선 = 억제되지 않았고 (1) 의 미억제 순환(억제 간선이 하나도 없는 순환)에 속하지 않은 간선. 이것으로 point_dist 와 earliest/latest 를 채운다.
--     그래서 한 곳의 모순이 무관한 시점의 경계를 지우지 않는다. 그래도 음수 사이클이 남으면(20건 초과) 전부 NULL.
CREATE FUNCTION solve_time(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE seen int[] := '{}'; skip int[] := '{}'; cyc int[]; rest int[]; x text; origin text;
BEGIN
  UPDATE point SET earliest = NULL, latest = NULL WHERE work_id = w;
  DELETE FROM diagnostic WHERE work_id = w AND run_id = 'derive' AND rule_id = 'time-cycle';

  -- seen: 이미 보고한 간선(검출에서 뺀다). skip: 풀이에서 뺄 간선 = 억제 간선이 없는 순환의 간선.
  -- 억제 간선이 낀 순환은 그 억제 간선만 빠지면 끊어지므로 나머지 간선은 풀이에 남는다(R1).
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

-- precedes(work, a, b): a ≤ b 가 모든 해에서 성립하면 'yes', b ≤ a 가 그러면 'no', 나머지 'maybe'.
-- 풀이 간선 집합의 거리로 판정한다: d(b, a) ≤ 0 이면 t_a − t_b ≤ 0 이 보장된다(yes), d(a, b) ≤ 0 이면 no.
-- STN 에서 이 판정은 완전하다(min ≥ 0 경로도, latest(a) ≤ earliest(b) 도 여기 포함된다). 풀어 둔 경계 비교를 먼저 한다.
-- 읽기만 한다(STABLE): derive 가 point_dist 를 미리 채운다. 거리가 없으면(모순으로 풀지 못함) 'maybe'.
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

-- 시점 ID 만으로 부르는 꼴(checks.md 의 규칙 예시). 작품은 a 의 행에서 찾는다.
CREATE FUNCTION precedes(a text, b text) RETURNS text
LANGUAGE sql STABLE STRICT SET search_path = public, ag_catalog AS $$
  SELECT precedes((SELECT work_id FROM point WHERE id = a), a, b)
$$;
