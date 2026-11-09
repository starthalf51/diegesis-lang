-- 003: role holding intervals (resolve_roles) and knowledge derivation (derive_beliefs). The rules follow the design note compiler.md "knowledge derivation" table.
SET LOCAL search_path = public, ag_catalog;

-- Propositions an act conveys: prop_id and with[] (e.g. `sends ross to macbeth with P`).
CREATE VIEW act_content AS
  SELECT a.work_id, a.id AS act_id, a.pt, a.verb, a.resolved_actor_id AS actor_id, c.prop_id, a.src_id
    FROM act a CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id);

-- Asserting utterances. The audience believes the content. asks (questions) and orders (commands) are not assertions, so they are excluded.
CREATE FUNCTION assertive(verb text) RETURNS bool LANGUAGE sql IMMUTABLE
  AS $$ SELECT verb = ANY ('{tells,says,whispers,hails,lies,shows,decrees,sends}') $$;

-- resolve_roles: fill role_span and turn role: speakers into the holder at that point.
-- The interval is [from, until) on the earliest schedule (the earliest solution of the STN, which by itself satisfies all constraints).
-- An open end is unbounded. If the end point is unsolved (earliest NULL), leave the interval empty so it drops out of the exclusivity check.
-- Overlap: if it overlaps an earlier interval of the same role (in src order) but the two may be apart (boundary order is maybe), leave a role-overlap warn
-- and do not insert this interval. If they must overlap, insert it as is and the exclusion constraint fails the load (a compiler bug, storage.md).
-- Speaker resolution: if exactly one holder has from ≤ act confirmed (yes) and act < until confirmed (act ≤ until is yes, until ≤ act is not yes),
-- that is the person. It is half-open like the interval, so an act at the boundary point belongs to the new holder. Otherwise role-ambiguous.
CREATE FUNCTION resolve_roles(w text) RETURNS void
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
                EXISTS (SELECT 1 FROM waiver v WHERE v.work_id = w AND v.rule = 'role-overlap' AND v.target_id = h.role_id));
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
              EXISTS (SELECT 1 FROM waiver v WHERE v.work_id = w AND v.rule = 'role-ambiguous' AND v.target_id = a.id));
    END IF;
  END LOOP;
END $$;

-- derive_beliefs: believes / uses / deception. A belief's from_pt is the point of that act, cause_act is that act.
--   assertive utterance   audience (excluding the speaker; for role: speakers, excluding the resolved holder) believes P
--   lies           + speaker believes ¬P (value=false) + deception. Any other assertion is uses(speaker, P)
--   sees           speaker believes P
--   reads art      speaker believes every proposition the art holds
--   writes art     uses(speaker, every proposition held)
--   … because P    uses(speaker, P)
--   aside/thinks   none (reader records are not these tables' job)
--   explicit `X believes P` (belief table)  X believes P, from_pt = at_pt, declared, cause_act = because_pt if it is an act
--   explicit `X doubts P`  no row (neither P nor ¬P is believed; the source stays in the belief table)
-- If the speaker is ambiguous (resolved_actor_id NULL), leave no speaker-side record and keep only the audience side.
CREATE FUNCTION derive_beliefs(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  DELETE FROM believes WHERE work_id = w;
  DELETE FROM uses WHERE work_id = w;
  DELETE FROM deception WHERE work_id = w;

  INSERT INTO believes(work_id, entity_id, prop_id, value, from_pt, cause_act, declared, src_id)
  SELECT w, au.entity_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = w AND assertive(c.verb) AND au.entity_id IS DISTINCT FROM c.actor_id
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, false, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'sees' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, true, a.pt, a.id, false, a.src_id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'reads' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, b.entity_id, b.prop_id, true, b.at_pt, (SELECT id FROM act WHERE id = b.because_pt), true, b.src_id
    FROM belief b WHERE b.work_id = w AND b.verb = 'believes' AND b.prop_id IS NOT NULL;

  INSERT INTO uses(work_id, entity_id, prop_id, at_pt, act_id)
  SELECT w, c.actor_id, c.prop_id, c.pt, c.act_id
    FROM act_content c WHERE c.work_id = w AND assertive(c.verb) AND c.verb <> 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, a.pt, a.id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'writes' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, a.because_id, a.pt, a.id
    FROM act a WHERE a.work_id = w AND a.because_id IS NOT NULL AND a.resolved_actor_id IS NOT NULL;

  INSERT INTO deception(work_id, act_id, liar_id, prop_id)
  SELECT w, c.act_id, c.actor_id, c.prop_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL;
END $$;
