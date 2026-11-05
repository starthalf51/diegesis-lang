-- 004: AGE 그래프 투영(project_graph) 과 derive. 그래프는 테이블에서 매번 새로 만드는 투영이다(ADR-001).
SET LOCAL search_path = public, ag_catalog;

-- rows(jsonb 배열)를 $rows 로 넘겨 `UNWIND $rows AS r <q>` 를 돈다. 문자열을 Cypher 에 이어 붙이지 않는다.
CREATE FUNCTION graph_rows(g text, q text, rows jsonb) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  IF rows IS NULL OR jsonb_array_length(rows) = 0 THEN
    RETURN;
  END IF;
  EXECUTE format('SELECT * FROM cypher(%L, $c$ UNWIND $rows AS r %s $c$, $1) AS (a agtype)', g, q)
    USING jsonb_build_object('rows', rows)::text::agtype;
END $$;

-- 정점 Entity/Point/Fact/Setup, 간선 BEFORE/TOLD/BELIEVES/PRESENT_AT/CAUSES/FORESHADOWS/PARENT_OF.
-- ponytail: 간선마다 MATCH 가 id 로 정점을 훑는다(O(V·E)). 큰 작품에서 느리면 id 속성 인덱스를 건다.
-- ponytail: BEFORE 는 추이 축소하지 않는다(storage.md 는 축소를 적었다). BEFORE* 가 느려지면 축소한다.
CREATE FUNCTION project_graph(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE g text := graph_name(w);
BEGIN
  IF EXISTS (SELECT 1 FROM ag_graph WHERE name = g) THEN
    PERFORM drop_graph(g, true);
  END IF;
  PERFORM create_graph(g);

  PERFORM graph_rows(g, 'CREATE (:Entity {id: r.id, kind: r.kind, name: r.name})',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT id, kind, name FROM entity WHERE work_id = w) x));
  PERFORM graph_rows(g, 'CREATE (:Point {id: r.id, kind: r.kind, earliest: r.earliest, latest: r.latest})',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT id, kind, earliest, latest FROM point WHERE work_id = w) x));
  PERFORM graph_rows(g, 'CREATE (:Fact {id: r.id, text: r.text, truth: r.truth})',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT id, text, truth FROM fact WHERE work_id = w) x));
  PERFORM graph_rows(g, 'CREATE (:Setup {id: r.id, kind: r.kind, prop: r.prop_id})',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT id, kind, prop_id FROM setup WHERE work_id = w) x));

  -- 순서: 억제되지 않은 min ≥ 0 제약. 같은 쌍은 한 간선(가장 빡빡한 경계).
  PERFORM graph_rows(g, 'MATCH (a:Point {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:BEFORE {min: r.min, max: r.max}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT from_pt AS a, to_pt AS b, max(min) AS min, min(max) AS max FROM time_edge
        WHERE work_id = w AND min >= 0 AND NOT waived AND from_pt <> to_pt GROUP BY from_pt, to_pt) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Entity {id: r.b}) CREATE (a)-[:TOLD {act: r.act, prop: r.prop, pt: r.pt}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT c.actor_id AS a, au.entity_id AS b, c.act_id AS act, c.prop_id AS prop, c.pt
         FROM act_content c JOIN audience au ON au.act_id = c.act_id
        WHERE c.work_id = w AND assertive(c.verb) AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Fact {id: r.b}) CREATE (a)-[:BELIEVES {from: r.from_pt, cause: r.cause, value: r.value}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT entity_id AS a, prop_id AS b, from_pt, cause_act AS cause, value FROM believes WHERE work_id = w) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:PRESENT_AT {scene: r.scene, from: r.f, to: r.t}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT p.entity_id AS a, s.start_pt AS b, p.scene_id AS scene, p.from_pt AS f, p.to_pt AS t
         FROM presence p JOIN scene s ON s.id = p.scene_id WHERE p.work_id = w) x));
  -- 인과: 명시적 `believes … because @x` (because_pt → at_pt) 와, `… because P` 행위의 화자가 P 를 믿게 된 행위 → 그 행위.
  PERFORM graph_rows(g, 'MATCH (a:Point {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:CAUSES {prop: r.prop}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT because_pt AS a, at_pt AS b, prop_id AS prop FROM belief
        WHERE work_id = w AND because_pt IS NOT NULL AND at_pt IS NOT NULL
       UNION
       SELECT b.from_pt, a.pt, a.because_id FROM act a
         JOIN believes b ON b.work_id = w AND b.entity_id = a.resolved_actor_id AND b.prop_id = a.because_id AND b.value
        WHERE a.work_id = w AND b.from_pt <> a.pt) x));
  PERFORM graph_rows(g, 'MATCH (a:Setup {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:FORESHADOWS]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT setup_id AS a, at_pt AS b FROM payoff WHERE work_id = w AND at_pt IS NOT NULL) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Entity {id: r.b}) CREATE (a)-[:PARENT_OF]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT parent_id AS a, child_id AS b FROM parent WHERE work_id = w) x));
END $$;

-- 적재 트랜잭션의 마지막 단계. 순서가 중요하다: 역할 해석은 풀린 시간을, 믿음은 해석된 화자를 쓴다.
CREATE OR REPLACE FUNCTION derive(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  PERFORM solve_time(w);
  PERFORM resolve_roles(w);
  PERFORM derive_beliefs(w);
  PERFORM project_graph(w);
END $$;
