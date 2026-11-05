-- 003: 역할 보유 구간(resolve_roles) 과 지식 파생(derive_beliefs). 규칙은 vault compiler.md "지식 파생" 표.
SET LOCAL search_path = public, ag_catalog;

-- 행위가 전하는 명제: prop_id 와 with[] (예: `sends ross to macbeth with P`).
CREATE VIEW act_content AS
  SELECT a.work_id, a.id AS act_id, a.pt, a.verb, a.resolved_actor_id AS actor_id, c.prop_id
    FROM act a CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id);

-- 단언하는 발화. 청중이 내용을 믿는다. asks(질문), orders(명령)는 단언이 아니라서 뺀다.
CREATE FUNCTION assertive(verb text) RETURNS bool LANGUAGE sql IMMUTABLE
  AS $$ SELECT verb = ANY ('{tells,says,whispers,hails,lies,shows,decrees,sends}') $$;

-- resolve_roles: role_span 을 채우고 role: 화자를 그 시점의 보유자로 바꾼다.
-- 구간은 earliest 일정(STN 의 가장 이른 해, 그 자체로 모든 제약을 만족한다)에서 [from, until) 이다.
-- 열린 끝은 무한. 끝 시점이 풀리지 않았으면(earliest NULL, 시간 모순) 구간을 비워 배타 검사에서 뺀다.
-- 화자 해석은 precedes 로 한다. from ≤ act ≤ until 이 확정(yes)인 보유자가 정확히 하나면 그 사람,
-- 아니면 role-ambiguous 진단을 남기고 resolved_actor_id 는 NULL 로 둔다.
CREATE FUNCTION resolve_roles(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
DECLARE a record; hs text[];
BEGIN
  DELETE FROM role_span WHERE work_id = w;
  DELETE FROM diagnostic WHERE work_id = w AND run_id = 'derive' AND rule_id = 'role-ambiguous';
  INSERT INTO role_span(work_id, role_id, entity_id, from_pt, until_pt, span, shared, src_id)
  SELECT w, h.role_id, h.entity_id, h.from_pt, h.until_pt,
         CASE WHEN (h.from_pt IS NOT NULL AND pf.earliest IS NULL) OR (h.until_pt IS NOT NULL AND pu.earliest IS NULL)
                   OR pf.earliest > pu.earliest THEN 'empty'::int8range
              ELSE int8range(pf.earliest, pu.earliest, '[)') END,
         h.shared OR coalesce(r.shared, false), h.src_id
    FROM role_holder h
    LEFT JOIN role r ON r.id = h.role_id
    LEFT JOIN point pf ON pf.id = h.from_pt
    LEFT JOIN point pu ON pu.id = h.until_pt
   WHERE h.work_id = w;

  UPDATE act SET resolved_actor_id = CASE WHEN actor_is_role THEN NULL ELSE actor_id END WHERE work_id = w;
  FOR a IN SELECT id, pt, actor_id, src_id FROM act WHERE work_id = w AND actor_is_role ORDER BY id LOOP
    SELECT array_agg(s.entity_id ORDER BY s.entity_id) INTO hs FROM role_span s
     WHERE s.work_id = w AND s.role_id = a.actor_id
       AND (s.from_pt IS NULL OR precedes(w, s.from_pt, a.pt) = 'yes')
       AND (s.until_pt IS NULL OR precedes(w, a.pt, s.until_pt) = 'yes');
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

-- derive_beliefs: believes / uses / deception. 믿음의 from_pt 는 그 행위의 시점, cause_act 는 그 행위다.
--   단언 발화      청중(화자 제외, role: 화자면 해석된 보유자 제외) believes P
--   lies           + 화자 believes ¬P(value=false) + deception. 그 밖의 단언은 uses(화자, P)
--   sees           화자 believes P
--   reads art      화자 believes art 에 담긴 명제 전부
--   writes art     uses(화자, 담긴 명제 전부)
--   … because P    uses(화자, P)
--   aside/thinks   없음(독자 기록은 이 테이블들의 몫이 아니다)
-- 화자가 모호하면(resolved_actor_id NULL) 화자 쪽 기록은 남기지 않고 청중 쪽만 남긴다.
CREATE FUNCTION derive_beliefs(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  DELETE FROM believes WHERE work_id = w;
  DELETE FROM uses WHERE work_id = w;
  DELETE FROM deception WHERE work_id = w;

  INSERT INTO believes(work_id, entity_id, prop_id, value, from_pt, cause_act)
  SELECT w, au.entity_id, c.prop_id, true, c.pt, c.act_id
    FROM act_content c JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = w AND assertive(c.verb) AND au.entity_id IS DISTINCT FROM c.actor_id
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, false, c.pt, c.act_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, true, c.pt, c.act_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'sees' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, true, a.pt, a.id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'reads' AND a.resolved_actor_id IS NOT NULL;

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
