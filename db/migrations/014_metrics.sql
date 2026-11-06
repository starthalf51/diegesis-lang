-- 014: 분석 지표(플랜 2 Task 8). Grafana 가 읽는 시계열과 인물 그래프. 전부 PG 안에서 계산한다(ADR-010).
--   x 축 = 보이는 장면의 discourse_ord(장면 하나가 한 칸). 모든 함수는 (work text) 를 받고, 빈 작품이면 0행이다.
--   독자 모델은 011·013 의 reader_knows 를, 패턴은 013 의 pattern_result 뷰를 그대로 쓴다. 여기서 다시 정하지 않는다.
SET LOCAL search_path = public, ag_catalog;

-- 보이는 장면의 서술 위치들(지표의 x 축).
CREATE FUNCTION metric_ords(w text) RETURNS TABLE (ord int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT DISTINCT s.discourse_ord FROM scene s WHERE s.work_id = w AND s.shown $$;

-- 장면별 출석과 행위 수(보이는 장면만; 말없이 선 인물은 acts = 0).
CREATE FUNCTION metric_presence(w text) RETURNS TABLE (ord int, scene text, entity text, acts int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.discourse_ord, s.id, p.entity_id,
         (SELECT count(*)::int FROM act a WHERE a.work_id = w AND a.scene_id = s.id
             AND coalesce(a.resolved_actor_id, a.actor_id) = p.entity_id)
    FROM scene s JOIN (SELECT DISTINCT scene_id, entity_id FROM presence WHERE work_id = w) p ON p.scene_id = s.id
   WHERE s.work_id = w AND s.shown
   ORDER BY 1, 2, 3 $$;

-- 그 장면 끝에서 독자가 믿는 참인 명제(reader_state 와 같은 규칙: 그 장면의 공개까지 포함) 중
-- 장면에 있는 인물이 모르는(char_knows = no | opp; maybe 는 세지 않는다 — dramatic-irony 패턴과 같다) 수.
-- 예언자·전지(lint_exempt)는 뺀다. ponytail: 참 = truth_at 가 정해진 것만(derived·unknown 사실은 NULL 이라 빠진다).
-- ponytail: char_knows 를 (장면 x 출석 인물 x 참인 사실)마다 부른다. 큰 작품이 느려지면 머티리얼라이즈한다.
CREATE FUNCTION metric_asymmetry(w text) RETURNS TABLE (ord int, entity text, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH kn AS (SELECT * FROM reader_knows(w)),
  sc AS (SELECT s.id, s.discourse_ord o, coalesce(s.end_pt, s.start_pt) pt FROM scene s
          WHERE s.work_id = w AND s.shown AND coalesce(s.end_pt, s.start_pt) IS NOT NULL),
  st AS (
    SELECT sc.id, sc.o, sc.pt, r.fact_id, r.value
      FROM sc CROSS JOIN LATERAL (SELECT DISTINCT ON (kn.fact_id) kn.fact_id, kn.value FROM kn
                                   WHERE kn.from_ord <= sc.o ORDER BY kn.fact_id, kn.from_ord DESC) r
     WHERE truth_at(w, r.fact_id, sc.pt) = r.value),
  pe AS (SELECT DISTINCT scene_id, entity_id FROM presence WHERE work_id = w)
  SELECT st.o, pe.entity_id,
         (count(*) FILTER (WHERE char_knows(w, pe.entity_id, st.fact_id, st.value, st.pt) IN ('no', 'opp')))::int
    FROM st JOIN pe ON pe.scene_id = st.id
   WHERE NOT lint_exempt(w, pe.entity_id)
   GROUP BY st.o, pe.entity_id
   ORDER BY 1, 2 $$;

-- 열린 setup 수: 그 위치까지 나온(setup 의 장면 또는 시점이 든 보이는 장면) setup 중 아직 payoff 가 그 위치까지 오지 않은 것.
-- (setup 누적 − payoff 누적과 같다. payoff 가 setup 보다 앞서도 음수가 되지 않게 setup 마다 센다.)
CREATE FUNCTION metric_open_setups(w text) RETURNS TABLE (ord int, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH su AS (
    SELECT s.id, coalesce((SELECT x.discourse_ord FROM scene x WHERE x.id = s.scene_id AND x.shown), point_ord(w, s.at_pt)) o
      FROM setup s WHERE s.work_id = w),
  pa AS (
    SELECT p.setup_id, min(coalesce((SELECT x.discourse_ord FROM scene x WHERE x.id = p.scene_id AND x.shown), point_ord(w, p.at_pt))) o
      FROM payoff p WHERE p.work_id = w GROUP BY p.setup_id)
  SELECT m.ord, (SELECT count(*)::int FROM su LEFT JOIN pa ON pa.setup_id = su.id
                  WHERE su.o <= m.ord AND (pa.o IS NULL OR pa.o > m.ord))
    FROM metric_ords(w) m ORDER BY 1 $$;

-- 활성 기만 수: 기만(deception 행: lies, 거짓 문서 writes)을 받은 사람 중 누군가가 그 위치에 아직 거짓을 믿는 것.
-- 믿음 행 = believes(value, cause_act = 그 lies 행위 또는 그 문서를 읽은 reads). until_pt(의심)가 서술 위치로 지났으면 거둔다.
-- 세는 것은 사람이 아니라 기만 (행위, 명제)다: 수신자 중 한 명이라도 아직 믿으면 1.
CREATE FUNCTION metric_active_deceptions(w text) RETURNS TABLE (ord int, n int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH live AS (  -- (기만 행위, 명제, 수신자) 마다 믿음이 서는 구간 [from, until)
    SELECT d.act_id, d.prop_id,
           (SELECT x.discourse_ord FROM act c JOIN scene x ON x.id = c.scene_id AND x.shown WHERE c.id = b.cause_act) f,
           point_ord(w, b.until_pt) u
      FROM deception d
      JOIN believes b ON b.work_id = w AND b.value AND b.entity_id <> d.liar_id
                     AND lint_canon(w, b.prop_id) = lint_canon(w, d.prop_id)
                     AND (b.cause_act = d.act_id
                          OR EXISTS (SELECT 1 FROM act r JOIN act wr ON wr.id = d.act_id
                                      WHERE r.id = b.cause_act AND r.verb = 'reads' AND wr.objects && r.objects))
     WHERE d.work_id = w)
  SELECT m.ord, (SELECT count(DISTINCT (l.act_id, l.prop_id))::int FROM live l
                  WHERE l.f <= m.ord AND (l.u IS NULL OR l.u > m.ord))
    FROM metric_ords(w) m ORDER BY 1 $$;

-- 가중치(작품과 무관). 합이 1 일 필요는 없다.
CREATE TABLE metric_weights (term text PRIMARY KEY, weight numeric NOT NULL);
INSERT INTO metric_weights VALUES ('promise', 0.4), ('deception', 0.3), ('pattern', 0.3);

-- 긴장 = Σ 가중치 × (항 / 작품 안 그 항의 최댓값). 최댓값이 0 이면 그 항은 0(0 나누기 없음).
--   열린 promise = 열린 setup 수(metric_open_setups), 활성 기만, 그 위치를 덮는 dramatic-irony·mystery 패턴 매치 수(pattern_result).
CREATE FUNCTION metric_tension(w text) RETURNS TABLE (ord int, value numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH t AS (
    SELECT s.ord, s.n a, d.n b,
           (SELECT count(*) FROM pattern_result m WHERE m.work_id = w AND m.pattern_id IN ('dramatic-irony', 'mystery')
               AND s.ord BETWEEN m.span_from AND m.span_to) c
      FROM metric_open_setups(w) s JOIN metric_active_deceptions(w) d USING (ord)),
  wt AS (SELECT coalesce(max(weight) FILTER (WHERE term = 'promise'), 0) p, coalesce(max(weight) FILTER (WHERE term = 'deception'), 0) d,
                coalesce(max(weight) FILTER (WHERE term = 'pattern'), 0) q FROM metric_weights)
  SELECT t.ord, round(wt.p * coalesce(t.a::numeric / nullif(max(t.a) OVER (), 0), 0)
                    + wt.d * coalesce(t.b::numeric / nullif(max(t.b) OVER (), 0), 0)
                    + wt.q * coalesce(t.c::numeric / nullif(max(t.c) OVER (), 0), 0), 4)
    FROM t CROSS JOIN wt ORDER BY 1 $$;

-- 시간 착행: x = 서술 위치, y = 장면 시작의 earliest(이야기 시간). 시간 제약이 없는 점(earliest = 0 이고 latest 없음: 풀이의 기본값)은 뺀다.
CREATE FUNCTION metric_anachrony(w text) RETURNS TABLE (scene text, ord int, story_t int8)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.id, s.discourse_ord, p.earliest FROM scene s JOIN point p ON p.id = s.start_pt
   WHERE s.work_id = w AND s.shown AND p.earliest IS NOT NULL AND (p.earliest > 0 OR p.latest IS NOT NULL) ORDER BY 2, 1 $$;

-- 속도: 장면의 이야기 시간 폭(끝 earliest − 시작 earliest, 초) 대 행위 수.
-- 시간 제약이 없는 점(earliest = 0 이고 latest 없음: 풀이의 기본값)이 한쪽이라도 끝이면 span_s = NULL.
CREATE FUNCTION metric_pace(w text) RETURNS TABLE (scene text, ord int, span_s int8, acts int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.id, s.discourse_ord,
         CASE WHEN (b.earliest > 0 OR b.latest IS NOT NULL) AND (e.earliest > 0 OR e.latest IS NOT NULL)
              THEN e.earliest - b.earliest END,
         (SELECT count(*)::int FROM act a WHERE a.work_id = w AND a.scene_id = s.id)
    FROM scene s LEFT JOIN point b ON b.id = s.start_pt LEFT JOIN point e ON e.id = s.end_pt
   WHERE s.work_id = w AND s.shown ORDER BY 2, 1 $$;

-- Grafana 노드 그래프: nodes(id, title, "mainStat") / edges(id, source, target, "mainStat"). 인물만. 노드 mainStat = 행위 수.
CREATE FUNCTION graph_nodes(w text) RETURNS TABLE (id text, title text, "mainStat" numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT e.id, e.name, (SELECT count(*) FROM act a WHERE a.work_id = w AND coalesce(a.resolved_actor_id, a.actor_id) = e.id)::numeric
    FROM entity e WHERE e.work_id = w AND e.kind = 'character' ORDER BY 1 $$;

-- 간선 = 함께 출석한 보이는 장면 수 + TOLD 수(어느 쪽이 누구에게든). 쌍마다 하나(source < target).
CREATE FUNCTION graph_edges(w text) RETURNS TABLE (id text, source text, target text, "mainStat" numeric)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH pe AS (SELECT DISTINCT p.scene_id, p.entity_id FROM presence p JOIN scene s ON s.id = p.scene_id AND s.shown
               WHERE p.work_id = w),
  co AS (SELECT least(a.entity_id, b.entity_id) x, greatest(a.entity_id, b.entity_id) y, count(*) n
           FROM pe a JOIN pe b ON a.scene_id = b.scene_id AND a.entity_id < b.entity_id GROUP BY 1, 2),
  told AS (SELECT least(c.actor_id, au.entity_id) x, greatest(c.actor_id, au.entity_id) y, count(*) n
             FROM act_content c JOIN audience au ON au.act_id = c.act_id
            WHERE c.work_id = w AND assertive(c.verb) AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id GROUP BY 1, 2),
  u AS (SELECT * FROM co UNION ALL SELECT * FROM told),
  ch AS (SELECT entity.id FROM entity WHERE entity.work_id = w AND entity.kind = 'character')
  SELECT u.x || '~' || u.y, u.x, u.y, sum(u.n)::numeric FROM u
   WHERE u.x IN (SELECT ch.id FROM ch) AND u.y IN (SELECT ch.id FROM ch)
   GROUP BY u.x, u.y ORDER BY 1 $$;
