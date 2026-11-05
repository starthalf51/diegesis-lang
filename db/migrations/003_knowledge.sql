-- 003: 역할 보유 구간(resolve_roles) 과 지식 파생(derive_beliefs). 규칙은 vault compiler.md "지식 파생" 표.
SET LOCAL search_path = public, ag_catalog;

-- 행위가 전하는 명제: prop_id 와 with[] (예: `sends ross to macbeth with P`).
CREATE VIEW act_content AS
  SELECT a.work_id, a.id AS act_id, a.pt, a.verb, a.resolved_actor_id AS actor_id, c.prop_id, a.src_id
    FROM act a CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id);

-- 단언하는 발화. 청중이 내용을 믿는다. asks(질문), orders(명령)는 단언이 아니라서 뺀다.
CREATE FUNCTION assertive(verb text) RETURNS bool LANGUAGE sql IMMUTABLE
  AS $$ SELECT verb = ANY ('{tells,says,whispers,hails,lies,shows,decrees,sends}') $$;

-- resolve_roles: role_span 을 채우고 role: 화자를 그 시점의 보유자로 바꾼다.
-- 구간은 earliest 일정(STN 의 가장 이른 해, 그 자체로 모든 제약을 만족한다)에서 [from, until) 이다.
-- 열린 끝은 무한. 끝 시점이 풀리지 않았으면(earliest NULL) 구간을 비워 배타 검사에서 뺀다.
-- 겹침: 같은 역할의 앞선 구간(src 순)과 겹치는데 둘이 떨어져 있을 수도 있으면(경계 선후가 maybe) role-overlap warn 을
-- 남기고 이 구간을 넣지 않는다. 반드시 겹치면 그대로 넣어 배타 제약이 적재를 실패시킨다(컴파일러 버그, storage.md).
-- 화자 해석: from ≤ act 가 확정(yes)이고 act < until 이 확정인(act ≤ until 은 yes, until ≤ act 는 yes 가 아님) 보유자가
-- 정확히 하나면 그 사람. 구간과 같은 반열림이라 경계 시점의 행위는 새 보유자다. 아니면 role-ambiguous.
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

-- derive_beliefs: believes / uses / deception. 믿음의 from_pt 는 그 행위의 시점, cause_act 는 그 행위다.
--   단언 발화      청중(화자 제외, role: 화자면 해석된 보유자 제외) believes P
--   lies           + 화자 believes ¬P(value=false) + deception. 그 밖의 단언은 uses(화자, P)
--   sees           화자 believes P
--   reads art      화자 believes art 에 담긴 명제 전부
--   writes art     uses(화자, 담긴 명제 전부)
--   … because P    uses(화자, P)
--   aside/thinks   없음(독자 기록은 이 테이블들의 몫이 아니다)
--   명시적 `X believes P`(belief 테이블)  X believes P, from_pt = at_pt, declared, cause_act = because_pt 가 행위면 그것
--   명시적 `X doubts P`  행 없음(P 도 ¬P 도 믿지 않는다. 원문은 belief 테이블에 남는다)
-- 화자가 모호하면(resolved_actor_id NULL) 화자 쪽 기록은 남기지 않고 청중 쪽만 남긴다.
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
