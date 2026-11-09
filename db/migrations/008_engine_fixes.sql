-- 008: Odyssey-wide engine fixes (Task 11).
--   a. lint_canon also resolves aliases inside a negation: `not q` (q = p) ≡ `not p`, `not r` (r = not p) ≡ p. Aliases of aliases too.
--      (Task 10 Bible: `not peter_disciple` and `not peter_with_jesus` looked like different propositions.)
--   b. Target of derive diagnostics: time-cycle carries the waiver target (time_edge.target_*) of the `when`/`born`/`dies` statement that made the cycle edge.
--      Suppression of role-overlap and role-ambiguous only by a waiver whose rule, target kind and target ID all match.
--   c. Add until (believes.until_pt, the point withdrawn by 006's doubt) to the graph BELIEVES edge.
SET LOCAL search_path = public, ag_catalog;

-- ── a. Canonical proposition ──
-- Canonical form: follow aliases to the end, `not X` is lint_neg(canonical X). Anything else (names, predicates) stays.
-- ponytail: predicate arguments (holds(r, alias)) are not resolved. If a work uses aliases in arguments, canonicalize the arguments too.
CREATE OR REPLACE FUNCTION lint_canon(w text, p text) RETURNS text
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb;
BEGIN
  FOR i IN 1..32 LOOP -- guard against alias cycles
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

-- Negation of canonical proposition c (canonical form). If c is `not X`, X. Otherwise the `not c` fact (the one defined exactly as not c), or if none, a string shaped like its ID
-- (no fact row exists, but the same proposition yields the same string).
CREATE FUNCTION lint_neg(w text, c text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT coalesce(
    (SELECT lint_canon(w, def->>'not') FROM fact WHERE work_id = w AND id = c AND def ? 'not'),
    (SELECT min(id) FROM fact WHERE work_id = w AND def->>'not' = c),
    w || '/not ' || c) $$;

-- Whether two canonical propositions negate each other. lint_neg is not an involution on negations with no fact row, so look at both sides.
CREATE OR REPLACE FUNCTION lint_opposite(w text, a text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT lint_neg(w, a) = b OR lint_neg(w, b) = a $$;

-- ── b. Target of derive diagnostics ──
ALTER TABLE time_edge ADD COLUMN target_kind text, ADD COLUMN target_id text;

-- Target = the statement of a suppressed edge if any (the earliest), otherwise the earliest-src statement among edges that have a target.
-- waived stays per edge: the edge of the statement carrying the suppression must be in the cycle (another statement's suppression does not cover it).
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

-- Same as 003's resolve_roles. Only the suppression condition is narrowed to a waiver whose rule, target kind and target ID all match.
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

-- ── c. Graph ──
-- Same as 004's project_graph. Adds until to BELIEVES. The explain query treats only beliefs with no until, or an until after the query point, as alive.
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
