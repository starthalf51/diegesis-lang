-- 013: Task 7 수정 1회차. 011·012 는 고치지 않는다(마이그레이션은 추가만).
--   1) 독자는 이미 거짓인 줄 아는 기만을 믿지 않는다. reader_events 를 바꾼다.
--      기만 공개 = lies 행위, 또는 기만으로 쓴 문서(deception 행이 있는 writes 의 문서 — 위조 서신)를 읽는 reads 행위.
--      속은 화자의 공개 = 화자가 그 (b, v) 를 기만 공개에서만 들었다(그 화자의 (b, v) believes 행이 모두 기만 행위에서 왔다).
--      거짓말한 사람 자신의 행(자기 lies 가 원인)은 속은 것이 아니다. 위조 문서를 스스로 읽은 행은 속은 것이다.
--      둘 중 하나인 공개 (b, v) 는, 그보다 앞선 위치((ord, sub) 순)에 기만이 아닌 공개로 (b, ¬v) 가 있었으면 뺀다.
--      거짓말한 사람이 방백·생각으로 ¬v 를 먼저 독자에게 알린 경우도 여기에 든다(그것도 기만이 아닌 공개다).
--      그런 앞선 공개가 없으면 독자는 여전히 들은 대로 믿는다(011 의 기본값).
--      ponytail: 속은 화자는 한 단계만 본다(속은 사람에게서 다시 들은 사람의 말은 그대로 믿는다). 사슬이 길어지면 재귀로 넓힌다.
--   2) pattern_result: 패턴 라이브러리 전체 실행(dieg patterns / dieg test, run_id 'patterns-…')의 마지막 결과.
--      lint 는 선언된 패턴만 자기 run('lint-…')으로 따로 넣으므로 이 뷰를 덮지 않는다. 대시보드는 이 뷰를 읽는다.
SET LOCAL search_path = public, ag_catalog;

-- 기만 공개 행위: lies, 그리고 기만으로 쓴 문서의 명제를 읽는 reads(명제 단위).
CREATE FUNCTION deceptive_act(w text, x_act text, x_prop text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM act a WHERE a.work_id = w AND a.id = x_act AND a.verb = 'lies')
      OR EXISTS (SELECT 1 FROM act r JOIN act wr ON wr.work_id = w AND wr.verb = 'writes' AND wr.objects && r.objects
                   JOIN deception d ON d.act_id = wr.id AND lint_canon(w, d.prop_id) = lint_canon(w, x_prop)
                  WHERE r.work_id = w AND r.id = x_act AND r.verb = 'reads') $$;

CREATE OR REPLACE FUNCTION reader_events(w text) RETURNS TABLE (fact_id text, value bool, ord int, sub int, via text, act_id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH told AS (
    SELECT c.prop_id, s.discourse_ord o, 2 * a.ord sub, a.verb, a.id, a.resolved_actor_id actor
      FROM act a JOIN scene s ON s.id = a.scene_id
      CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id)
     WHERE a.work_id = w AND s.shown AND (assertive(a.verb) OR a.verb IN ('sees', 'aside', 'thinks'))
    UNION ALL
    SELECT p, s.discourse_ord, 2 * a.ord, a.verb, a.id, a.resolved_actor_id
      FROM act a JOIN scene s ON s.id = a.scene_id JOIN artifact art ON art.id = ANY (a.objects)
      CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND s.shown AND a.verb = 'reads'),
  held AS (
    SELECT z.base, coalesce(sd.discourse_ord, 0) d_ord, point_ord(w, wh.until_pt) u_ord
      FROM withhold wh LEFT JOIN scene sd ON sd.id = wh.scene_id AND sd.shown
      CROSS JOIN LATERAL prop_norm(w, wh.prop_id) z
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL),
  acts AS (
    SELECT z.base, z.val, t.o, t.sub, t.verb, t.id, t.actor, deceptive_act(w, t.id, t.prop_id) dec, h.blocked, h.u_ord
      FROM told t CROSS JOIN LATERAL prop_norm(w, t.prop_id) z
      LEFT JOIN LATERAL (SELECT true blocked, h.u_ord FROM held h
                          WHERE h.base = z.base AND h.d_ord <= t.o AND (h.u_ord IS NULL OR h.u_ord > t.o)
                          ORDER BY h.u_ord DESC NULLS FIRST LIMIT 1) h ON true),
  ev AS (
    SELECT base, val, CASE WHEN blocked THEN u_ord ELSE o END o, CASE WHEN blocked THEN -1 ELSE sub END sub, verb, id, actor, dec
      FROM acts WHERE NOT coalesce(blocked, false) OR u_ord IS NOT NULL
    UNION ALL
    SELECT z.base, z.val, coalesce(point_ord(w, r.at_pt), sr.discourse_ord),
           coalesce(2 * (SELECT a.ord FROM act a WHERE a.id = r.at_pt) + 1, 1), 'reveal', NULL, NULL, false
      FROM reveal r LEFT JOIN scene sr ON sr.id = r.scene_id AND sr.shown
      CROSS JOIN LATERAL prop_norm(w, r.prop_id) z
     WHERE r.work_id = w AND r.prop_id IS NOT NULL AND coalesce(point_ord(w, r.at_pt), sr.discourse_ord) IS NOT NULL),
  dupe AS (  -- 속은 화자: 그 (b, v) 를 기만 공개에서만 들었다
    SELECT e.id, e.base, e.val FROM ev e
     WHERE NOT e.dec AND e.actor IS NOT NULL
       AND EXISTS (SELECT 1 FROM believes b CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
                    WHERE b.work_id = w AND b.entity_id = e.actor AND z.base = e.base AND (z.val = b.value) = e.val
                      AND b.cause_act IS NOT NULL AND deceptive_act(w, b.cause_act, b.prop_id)
                      AND NOT EXISTS (SELECT 1 FROM act y WHERE y.id = b.cause_act AND y.resolved_actor_id = b.entity_id AND y.verb = 'lies'))
       AND NOT EXISTS (SELECT 1 FROM believes b CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
                        WHERE b.work_id = w AND b.entity_id = e.actor AND z.base = e.base AND (z.val = b.value) = e.val
                          AND (b.cause_act IS NULL OR NOT deceptive_act(w, b.cause_act, b.prop_id)
                               OR EXISTS (SELECT 1 FROM act y WHERE y.id = b.cause_act AND y.resolved_actor_id = b.entity_id AND y.verb = 'lies'))))
  SELECT e.base, e.val, e.o, e.sub, e.verb, e.id FROM ev e
   WHERE NOT (e.dec OR EXISTS (SELECT 1 FROM dupe d WHERE d.id = e.id AND d.base = e.base AND d.val = e.val))
      OR NOT EXISTS (SELECT 1 FROM ev x WHERE NOT x.dec AND x.base = e.base AND x.val <> e.val
                        AND (x.o, x.sub) < (e.o, e.sub)
                        AND NOT EXISTS (SELECT 1 FROM dupe d WHERE d.id = x.id AND d.base = x.base AND d.val = x.val)) $$;

CREATE VIEW pattern_result AS
  SELECT m.* FROM pattern_match m
   WHERE m.run_id LIKE 'patterns-%'
     AND m.run_id = (SELECT max(x.run_id) FROM pattern_match x WHERE x.work_id = m.work_id AND x.run_id LIKE 'patterns-%');
