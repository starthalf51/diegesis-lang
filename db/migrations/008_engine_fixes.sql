-- 008: 오디세이 전 엔진 수정(Task 11).
--   a. lint_canon 이 부정 안의 별칭도 푼다: `not q`(q = p) ≡ `not p`, `not r`(r = not p) ≡ p. 별칭의 별칭도 푼다.
--      (Task 10 성경: `not peter_disciple` 과 `not peter_with_jesus` 가 다른 명제로 보였다.)
--   b. derive 진단의 대상: time-cycle 은 순환 간선을 만든 `when`/`born`/`dies` 문장의 waiver 대상(time_edge.target_*)을
--      가진다. role-overlap·role-ambiguous 의 억제는 규칙·대상 종류·대상 ID 가 모두 같은 waiver 만.
--   c. 그래프 BELIEVES 간선에 until(believes.until_pt, 006 의 의심으로 거둔 시점).
SET LOCAL search_path = public, ag_catalog;

-- ── a. 정규 명제 ──
-- 정규형: 별칭은 끝까지 따라가고, `not X` 는 lint_neg(정규 X). 그 밖(이름·술어)은 그대로.
-- ponytail: 술어 인자(holds(r, alias))는 풀지 않는다. 인자에 별칭을 쓰는 작품이 생기면 인자도 정규화한다.
CREATE OR REPLACE FUNCTION lint_canon(w text, p text) RETURNS text
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb;
BEGIN
  FOR i IN 1..32 LOOP -- 별칭 순환 방지
    SELECT def INTO d FROM fact WHERE work_id = w AND id = p;
    IF d ? 'alias' THEN
      p := d->>'alias';
    ELSIF d ? 'not' THEN
      RETURN lint_neg(w, lint_canon(w, d->>'not'));
    ELSE
      RETURN p;
    END IF;
  END LOOP;
  RETURN p;
END $$;

-- 정규 명제 c 의 부정(정규형). c 가 `not X` 면 X. 아니면 `not c` 사실(정의가 정확히 not c 인 것), 없으면 그 ID 꼴 문자열
-- (사실 행은 없지만 같은 명제끼리는 같은 문자열이 된다).
CREATE FUNCTION lint_neg(w text, c text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT coalesce(
    (SELECT lint_canon(w, def->>'not') FROM fact WHERE work_id = w AND id = c AND def ? 'not'),
    (SELECT min(id) FROM fact WHERE work_id = w AND def->>'not' = c),
    w || '/not ' || c) $$;

-- 두 정규 명제가 서로 부정인가. lint_neg 는 사실 행이 없는 부정에서 대합(involution)이 아니라서 양쪽을 본다.
CREATE OR REPLACE FUNCTION lint_opposite(w text, a text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT lint_neg(w, a) = b OR lint_neg(w, b) = a $$;

-- ── b. derive 진단의 대상 ──
ALTER TABLE time_edge ADD COLUMN target_kind text, ADD COLUMN target_id text;

-- 대상 = 억제된 간선이 있으면 그 문장(가장 앞선 것), 없으면 대상이 있는 간선 중 src 가 가장 앞선 문장.
-- waived 는 그대로 간선 단위다: 억제가 붙은 그 문장의 간선이 순환에 있어야 한다(다른 문장의 억제는 덮지 않는다).
CREATE OR REPLACE FUNCTION time_cycle_diag(w text, ords int[]) RETURNS void
LANGUAGE sql SET search_path = public, ag_catalog AS $$
  INSERT INTO diagnostic(run_id, work_id, rule_id, severity, target_kind, target_id, src_id, message, data, waived)
  SELECT 'derive', w, 'time-cycle', 'warn', coalesce(t.target_kind, 'when'), t.target_id, min(e.src_id),
         format('time constraints form a cycle (%s constraints)', count(*)),
         jsonb_build_object(
           'src_ids', coalesce(jsonb_agg(DISTINCT e.src_id ORDER BY e.src_id) FILTER (WHERE e.src_id IS NOT NULL), '[]'),
           'edges', jsonb_agg(e.ord ORDER BY e.ord),
           'points', (SELECT jsonb_agg(DISTINCT p ORDER BY p) FROM time_edge, LATERAL (VALUES (from_pt), (to_pt)) v(p)
                       WHERE work_id = w AND ord = ANY (ords))),
         bool_or(e.waived)
    FROM time_edge e
    LEFT JOIN LATERAL (
      SELECT x.target_kind, x.target_id FROM time_edge x
       WHERE x.work_id = w AND x.ord = ANY (ords) AND nullif(x.target_id, '') IS NOT NULL
       ORDER BY x.waived DESC, x.src_id NULLS LAST, x.ord LIMIT 1) t ON true
   WHERE e.work_id = w AND e.ord = ANY (ords)
   GROUP BY t.target_kind, t.target_id
$$;

-- 003 의 resolve_roles 와 같다. 억제 조건만 규칙·대상 종류·대상 ID 가 모두 같은 waiver 로 좁혔다.
CREATE OR REPLACE FUNCTION resolve_roles(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE a record; h record; o record; hs text[];
BEGIN
  DELETE FROM role_span WHERE work_id = w;
  DELETE FROM diagnostic WHERE work_id = w AND run_id = 'derive' AND rule_id IN ('role-ambiguous', 'role-overlap');
  FOR h IN
    SELECT rh.role_id, rh.entity_id, rh.from_pt, rh.until_pt, rh.src_id, rh.shared OR coalesce(r.shared, false) AS shared,
           CASE WHEN (rh.from_pt IS NOT NULL AND pf.earliest IS NULL) OR (rh.until_pt IS NOT NULL AND pu.earliest IS NULL)
                     OR pf.earliest > pu.earliest THEN 'empty'::int8range
                ELSE int8range(pf.earliest, pu.earliest, '[)') END AS span
      FROM role_holder rh
      LEFT JOIN role r ON r.id = rh.role_id
      LEFT JOIN point pf ON pf.id = rh.from_pt
      LEFT JOIN point pu ON pu.id = rh.until_pt
     WHERE rh.work_id = w
     ORDER BY rh.src_id NULLS LAST, rh.role_id, rh.entity_id
  LOOP
    IF NOT h.shared THEN
      SELECT * INTO o FROM role_span s
       WHERE s.work_id = w AND s.role_id = h.role_id AND NOT s.shared AND s.span && h.span LIMIT 1;
      IF FOUND AND ((h.until_pt IS NOT NULL AND o.from_pt IS NOT NULL AND precedes(w, h.until_pt, o.from_pt) <> 'no')
                 OR (o.until_pt IS NOT NULL AND h.from_pt IS NOT NULL AND precedes(w, o.until_pt, h.from_pt) <> 'no')) THEN
        INSERT INTO diagnostic(run_id, work_id, rule_id, severity, target_kind, target_id, src_id, message, data, waived)
        VALUES ('derive', w, 'role-overlap', 'warn', 'role', h.role_id, h.src_id,
                format('%s may hold %s while %s still does (order of the boundaries is not determined)', h.entity_id, h.role_id, o.entity_id),
                jsonb_build_object('role', h.role_id, 'holder', h.entity_id, 'other', o.entity_id, 'other_src_id', o.src_id),
                EXISTS (SELECT 1 FROM waiver v WHERE v.work_id = w AND v.rule = 'role-overlap'
                           AND v.target_kind = 'role' AND v.target_id = h.role_id));
        CONTINUE;
      END IF;
    END IF;
    INSERT INTO role_span(work_id, role_id, entity_id, from_pt, until_pt, span, shared, src_id)
    VALUES (w, h.role_id, h.entity_id, h.from_pt, h.until_pt, h.span, h.shared, h.src_id);
  END LOOP;

  UPDATE act SET resolved_actor_id = CASE WHEN actor_is_role THEN NULL ELSE actor_id END WHERE work_id = w;
  FOR a IN SELECT id, pt, actor_id, src_id FROM act WHERE work_id = w AND actor_is_role ORDER BY id LOOP
    SELECT array_agg(s.entity_id ORDER BY s.entity_id) INTO hs FROM role_span s
     WHERE s.work_id = w AND s.role_id = a.actor_id
       AND (s.from_pt IS NULL OR precedes(w, s.from_pt, a.pt) = 'yes')
       AND (s.until_pt IS NULL OR (precedes(w, a.pt, s.until_pt) = 'yes' AND precedes(w, s.until_pt, a.pt) <> 'yes'));
    IF cardinality(hs) = 1 THEN
      UPDATE act SET resolved_actor_id = hs[1] WHERE id = a.id;
    ELSE
      INSERT INTO diagnostic(run_id, work_id, rule_id, severity, target_kind, target_id, src_id, message, data, waived)
      VALUES ('derive', w, 'role-ambiguous', 'warn', 'act', a.id, a.src_id,
              format('holder of %s at this point is not determined (%s candidates)', a.actor_id, coalesce(cardinality(hs), 0)),
              jsonb_build_object('role', a.actor_id, 'holders', coalesce(to_jsonb(hs), '[]')),
              EXISTS (SELECT 1 FROM waiver v WHERE v.work_id = w AND v.rule = 'role-ambiguous'
                         AND v.target_kind = 'act' AND v.target_id = a.id));
    END IF;
  END LOOP;
END $$;

-- ── c. 그래프 ──
-- 004 의 project_graph 와 같다. BELIEVES 에 until 을 더했다. 설명 질의는 until 이 없거나 질의 시점 뒤인 믿음만 살아 있다고 본다.
CREATE OR REPLACE FUNCTION project_graph(w text) RETURNS void
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

  PERFORM graph_rows(g, 'MATCH (a:Point {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:BEFORE {min: r.min, max: r.max}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT from_pt AS a, to_pt AS b, max(min) AS min, min(max) AS max FROM time_edge
        WHERE work_id = w AND min >= 0 AND NOT waived AND from_pt <> to_pt GROUP BY from_pt, to_pt) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Entity {id: r.b}) CREATE (a)-[:TOLD {act: r.act, prop: r.prop, pt: r.pt}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT c.actor_id AS a, au.entity_id AS b, c.act_id AS act, c.prop_id AS prop, c.pt
         FROM act_content c JOIN audience au ON au.act_id = c.act_id
        WHERE c.work_id = w AND assertive(c.verb) AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Fact {id: r.b}) CREATE (a)-[:BELIEVES {from: r.from_pt, until: r.until_pt, cause: r.cause, value: r.value, declared: r.declared}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT entity_id AS a, prop_id AS b, from_pt, until_pt, cause_act AS cause, value, declared FROM believes WHERE work_id = w) x));
  PERFORM graph_rows(g, 'MATCH (a:Entity {id: r.a}), (b:Point {id: r.b}) CREATE (a)-[:PRESENT_AT {scene: r.scene, from: r.f, to: r.t}]->(b)',
    (SELECT jsonb_agg(to_jsonb(x)) FROM (
       SELECT p.entity_id AS a, s.start_pt AS b, p.scene_id AS scene, p.from_pt AS f, p.to_pt AS t
         FROM presence p JOIN scene s ON s.id = p.scene_id WHERE p.work_id = w) x));
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
