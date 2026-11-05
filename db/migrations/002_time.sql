-- 002: STN 풀이(solve_time) 와 선후 판정(precedes).
--
-- time_edge 한 행 (from, to, min, max) 은 min ≤ t_to − t_from ≤ max 다. 거리 그래프로 바꾸면
--   from → to  가중치 max   (t_to − t_from ≤ max)
--   to → from  가중치 −min  (t_from − t_to ≤ −min)
-- 이고, 최단 거리 d(u,v) 는 t_v − t_u 의 가장 빡빡한 상한이다. NULL 경계는 간선이 없다(무한).
--   latest(p)   =  d(origin, p)       정방향 Bellman-Ford
--   earliest(p) = −d(p, origin)       역방향(간선을 뒤집은) Bellman-Ford
SET LOCAL search_path = public, ag_catalog;

-- 거리 그래프를 임시 테이블 _stn_e 에 싣는다. skip 은 빼고 싶은 time_edge.ord 들.
CREATE FUNCTION stn_edges(w text, with_waived bool, skip int[] DEFAULT '{}') RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  IF to_regclass('pg_temp._stn_e') IS NULL THEN
    CREATE TEMP TABLE _stn_e(u text NOT NULL, v text NOT NULL, wt int8 NOT NULL, ord int NOT NULL);
  END IF;
  TRUNCATE _stn_e;
  INSERT INTO _stn_e
    SELECT from_pt, to_pt, max, ord FROM time_edge
     WHERE work_id = w AND max IS NOT NULL AND (with_waived OR NOT waived) AND ord <> ALL (skip)
    UNION ALL
    SELECT to_pt, from_pt, -min, ord FROM time_edge
     WHERE work_id = w AND min IS NOT NULL AND (with_waived OR NOT waived) AND ord <> ALL (skip);
END $$;

-- 단일 출발점 Bellman-Ford(집합 단위 완화) over _stn_e. 결과는 _stn_d(pt, dist, pred, pred_pt).
-- source NULL 이면 모든 점을 0 에서 시작한다(가상 출발점 = 음수 사이클 검출용).
-- backward 면 간선을 뒤집어 d(p, source) 를 구한다.
-- 반환: NULL = 수렴. 아니면 V 번째 반복에서도 줄어든 점(음수 사이클 위이거나 거기서 닿는 점).
CREATE FUNCTION stn_bf(source text, backward bool) RETURNS text
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE nv int; n int; x text;
BEGIN
  IF to_regclass('pg_temp._stn_d') IS NULL THEN
    CREATE TEMP TABLE _stn_d(pt text PRIMARY KEY, dist int8, pred int, pred_pt text);
  END IF;
  TRUNCATE _stn_d;
  INSERT INTO _stn_d(pt, dist)
    SELECT p, CASE WHEN source IS NULL OR p = source THEN 0 END
      FROM (SELECT u FROM _stn_e UNION SELECT v FROM _stn_e UNION SELECT source WHERE source IS NOT NULL) s(p);
  SELECT count(*) INTO nv FROM _stn_d;
  FOR i IN 1..nv LOOP
    WITH e AS (
      SELECT CASE WHEN backward THEN v ELSE u END AS a, CASE WHEN backward THEN u ELSE v END AS b, wt, ord FROM _stn_e),
    c AS (
      SELECT DISTINCT ON (e.b) e.b, d.dist + e.wt AS nd, e.ord, e.a
        FROM e JOIN _stn_d d ON d.pt = e.a
       WHERE d.dist IS NOT NULL
       ORDER BY e.b, d.dist + e.wt, e.ord),
    up AS (
      UPDATE _stn_d t SET dist = c.nd, pred = c.ord, pred_pt = c.a
        FROM c WHERE t.pt = c.b AND (t.dist IS NULL OR c.nd < t.dist)
      RETURNING t.pt)
    SELECT count(*), min(pt) INTO n, x FROM up;
    IF n = 0 THEN
      RETURN NULL;
    END IF;
  END LOOP;
  RETURN x;
END $$;

-- 정방향 stn_bf 가 돌려준 점 x 에서 선행자를 V 번 거슬러 사이클에 들어간 뒤, 한 바퀴의 time_edge.ord 를 모은다.
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

-- 사이클을 이루는 간선들로 time-cycle 진단 한 건. 그 중 하나라도 억제됐으면 waived (R1).
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

-- solve_time: (1) 모든 간선(억제 포함)에서 순환을 찾아 진단한다. 순서 순환(min ≥ 0 간선이 도는 것, 길이 0
-- 이어도)은 강연결요소마다 한 건, 그 밖의 음수 사이클은 Bellman-Ford 로 한 건씩.
-- (2) 억제된 간선을 빼고 earliest/latest 를 푼다. 남은 간선에 음수 사이클이 있으면 원점 성분 전체가 NULL 이다
-- (모든 시점이 원점에 묶이므로 "사이클 없는 성분만 푼다" 와 같다. 원점에 안 닿는 점은 어차피 NULL).
CREATE FUNCTION solve_time(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE skip int[]; comp record; x text; cyc int[]; origin text; ok bool;
BEGIN
  UPDATE point SET earliest = NULL, latest = NULL WHERE work_id = w;
  DELETE FROM diagnostic WHERE work_id = w AND run_id = 'derive' AND rule_id = 'time-cycle';

  -- ponytail: 닫힘은 O(V²) 쌍이다. 시점이 수만 개면 PL/pgSQL Tarjan 으로 바꾼다.
  skip := '{}';
  FOR comp IN
    WITH RECURSIVE pe AS (
      SELECT from_pt AS a, to_pt AS b FROM time_edge WHERE work_id = w AND min >= 0 AND from_pt <> to_pt),
    r(a, b) AS (SELECT a, b FROM pe UNION SELECT r.a, pe.b FROM r JOIN pe ON pe.a = r.b),
    scc AS (SELECT r1.a AS pt, min(r1.b) AS c FROM r r1 JOIN r r2 ON r2.a = r1.b AND r2.b = r1.a GROUP BY r1.a)
    SELECT array_agg(e.ord ORDER BY e.ord) AS ords
      FROM time_edge e JOIN scc s1 ON s1.pt = e.from_pt JOIN scc s2 ON s2.pt = e.to_pt AND s2.c = s1.c
     WHERE e.work_id = w AND e.min >= 0
     GROUP BY s1.c
  LOOP
    PERFORM time_cycle_diag(w, comp.ords);
    skip := skip || comp.ords;
  END LOOP;

  -- 음수 사이클: 찾으면 진단하고 그 간선을 빼고 다시 찾는다(최대 20건).
  FOR k IN 1..20 LOOP
    PERFORM stn_edges(w, true, skip);
    x := stn_bf(NULL, false);
    EXIT WHEN x IS NULL;
    cyc := stn_cycle(x);
    EXIT WHEN cardinality(cyc) = 0;
    PERFORM time_cycle_diag(w, cyc);
    skip := skip || cyc;
  END LOOP;

  SELECT id INTO origin FROM point WHERE work_id = w AND kind = 'origin' ORDER BY id LIMIT 1;
  IF origin IS NULL THEN
    RETURN;
  END IF;
  PERFORM stn_edges(w, false);
  ok := stn_bf(origin, false) IS NULL;
  IF ok THEN
    UPDATE point p SET latest = d.dist FROM _stn_d d WHERE p.work_id = w AND p.id = d.pt;
    ok := stn_bf(origin, true) IS NULL;
    IF ok THEN
      UPDATE point p SET earliest = -d.dist FROM _stn_d d WHERE p.work_id = w AND p.id = d.pt;
    ELSE
      UPDATE point SET latest = NULL WHERE work_id = w;
    END IF;
  END IF;
END $$;

-- precedes(work, a, b): a ≤ b 가 모든 해에서 성립하면 'yes', b ≤ a 가 그러면 'no', 나머지 'maybe'.
-- 계산: 억제되지 않은 간선의 거리 그래프에서 d(b, a) ≤ 0 이면 t_a − t_b ≤ 0 이 보장된다(yes).
-- d(a, b) ≤ 0 이면 no. 이 판정은 STN 에서 완전하다(min ≥ 0 경로도, latest(a) ≤ earliest(b) 도 여기에 포함된다).
-- 먼저 풀어 둔 earliest/latest 로 싸게 확인하고, 안 되면 b(그다음 a)에서 Bellman-Ford 를 한 번씩 돈다.
-- 모순된 STN 이면 'maybe'. 인자가 NULL 이면 NULL (STRICT).
-- ponytail: 호출마다 BF 한두 번. 린트가 느려지면 (work, source) 단위로 거리를 캐시한다.
CREATE FUNCTION precedes(w text, a text, b text) RETURNS text
LANGUAGE plpgsql STRICT SET search_path = public, ag_catalog AS $$
DECLARE ea int8; la int8; eb int8; lb int8; d int8;
BEGIN
  IF a = b THEN
    RETURN 'yes';
  END IF;
  SELECT earliest, latest INTO ea, la FROM point WHERE work_id = w AND id = a;
  SELECT earliest, latest INTO eb, lb FROM point WHERE work_id = w AND id = b;
  IF la <= eb THEN
    RETURN 'yes';
  ELSIF lb <= ea THEN
    RETURN 'no';
  END IF;
  PERFORM stn_edges(w, false);
  IF stn_bf(b, false) IS NOT NULL THEN
    RETURN 'maybe';
  END IF;
  SELECT dist INTO d FROM _stn_d WHERE pt = a;
  IF d <= 0 THEN
    RETURN 'yes';
  END IF;
  PERFORM stn_bf(a, false);
  SELECT dist INTO d FROM _stn_d WHERE pt = b;
  IF d <= 0 THEN
    RETURN 'no';
  END IF;
  RETURN 'maybe';
END $$;

-- 시점 ID 만으로 부르는 꼴(checks.md 의 규칙 예시). 작품은 a 의 행에서 찾는다.
CREATE FUNCTION precedes(a text, b text) RETURNS text
LANGUAGE sql STRICT SET search_path = public, ag_catalog AS $$
  SELECT precedes((SELECT work_id FROM point WHERE id = a), a, b)
$$;
