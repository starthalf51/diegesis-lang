-- 009: 렌더 요청(Task 12, ADR-010·012). render_scene(작품, 장면) 은 internal/render.Scene 과 같은 모양의 JSON 을 낸다.
--   ID 는 작품 접두어를 뗀 짧은 꼴(lint_short)이다. 사실 ID 는 정규 명제(lint_canon)라서 별칭끼리 갈라지지 않는다.
--   입·퇴장은 presence 의 바깥 경계로 만든다: from_pt 바로 뒤에 들어오고 to_pt 바로 앞에서 나간다.
--   경계가 행위면 그 행위 앞뒤, 비트 시점이면 그 비트의 첫 행위 앞, 장면 끝이면 마지막 행위 뒤다. 같은 틈에서는 퇴장이 먼저다.
--   장면 시작부터 있는 인물, 출석 기록이 없는 인물(기록(account) 블록)은 on_stage 다.
--   pov_beliefs = 시점 인물이 장면 시작에 확정적으로 아는(lint_known = 2, 의심의 until_pt 반영) 이 장면의 사실.
--   reader_knows = 앞선(서술 순서) 보이는 장면의 행위·reveal 로 독자가 이미 받은 이 장면의 사실.
--   withheld = 이 장면의 서술 위치에서 아직 공개되지 않은 withhold 명제(이 장면의 행위가 전하는 것은 뺀다).
--   previously = 앞선 보이는 장면 셋의 제목과 사실 문장을 이어 붙인 것(LLM 없음).
-- ponytail: 짧은 ID 는 모듈이 달라도 끝 이름이 같으면 겹친다. 겹치는 작품이 생기면 모듈 접두어를 남긴다.
SET LOCAL search_path = public, ag_catalog;

-- 이름: 그 언어 → 언어 없음 → 엔터티 이름 → 짧은 ID.
CREATE FUNCTION render_name(w text, x text, l text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT coalesce(
    (SELECT n.text FROM name n WHERE n.work_id = w AND n.owner_id = x AND n.lang = l ORDER BY n.ord LIMIT 1),
    (SELECT n.text FROM name n WHERE n.work_id = w AND n.owner_id = x AND n.lang = '' ORDER BY n.ord LIMIT 1),
    (SELECT e.name FROM entity e WHERE e.work_id = w AND e.id = x),
    lint_short(x)) $$;

-- 사실 문장(영어). 별칭은 따라가고, 부정은 "It is not true that …", 술어는 holds/alive 만 문장으로 푼다.
CREATE FUNCTION render_fact_text(w text, f text) RETURNS text
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb; t text;
BEGIN
  FOR i IN 1..32 LOOP -- 별칭 순환 방지
    SELECT x.def, x.text INTO d, t FROM fact x WHERE x.work_id = w AND x.id = f;
    IF NOT FOUND AND f LIKE w || '/not %' THEN -- lint_neg 가 만든, 행이 없는 부정
      RETURN 'It is not true that ' || render_fact_text(w, substr(f, length(w) + 6));
    ELSIF d ? 'alias' THEN
      f := d->>'alias';
    ELSIF d ? 'not' THEN
      RETURN 'It is not true that ' || render_fact_text(w, d->>'not');
    ELSIF d ? 'pred' THEN
      RETURN CASE d->>'pred'
        WHEN 'holds' THEN render_name(w, d->'args'->>1, 'en') || ' is ' || render_name(w, d->'args'->>0, 'en')
        WHEN 'alive' THEN render_name(w, d->'args'->>0, 'en') || ' is alive'
        ELSE (d->>'pred') || '(' || (SELECT string_agg(render_name(w, a, 'en'), ', ')
                                       FROM jsonb_array_elements_text(d->'args') a) || ')' END;
    ELSE
      RETURN coalesce(nullif(t, ''), (SELECT n.text FROM name n WHERE n.work_id = w AND n.owner_id = f ORDER BY n.ord LIMIT 1),
                      lint_short(f));
    END IF;
  END LOOP;
  RETURN lint_short(f);
END $$;

-- 작품 안에서 참인가: 별칭은 따라가고 부정은 뒤집는다. truth = false 만 거짓(derived·unknown 은 참으로 둔다).
CREATE FUNCTION render_fact_true(w text, f text) RETURNS bool
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb; t text; neg bool := false;
BEGIN
  FOR i IN 1..32 LOOP
    SELECT x.def, x.truth INTO d, t FROM fact x WHERE x.work_id = w AND x.id = f;
    IF NOT FOUND AND f LIKE w || '/not %' THEN
      f := substr(f, length(w) + 6); neg := NOT neg;
    ELSIF d ? 'alias' THEN
      f := d->>'alias';
    ELSIF d ? 'not' THEN
      f := d->>'not'; neg := NOT neg;
    ELSE
      RETURN (coalesce(t, 'true') <> 'false') <> neg;
    END IF;
  END LOOP;
  RETURN NOT neg;
END $$;

CREATE FUNCTION render_scene(w text, s text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SET search_path = public, ag_catalog AS $$
DECLARE
  sc scene; n int; pov_v text; pov text; acts jsonb; beats jsonb; cast_ids text[]; facts text[]; held text[];
BEGIN
  SELECT count(*) INTO n FROM scene x WHERE x.work_id = w AND (x.id = s OR lint_short(x.id) = s);
  IF n <> 1 THEN
    RAISE EXCEPTION 'render_scene: % scenes named % in %', n, s, w;
  END IF;
  SELECT * INTO sc FROM scene x WHERE x.work_id = w AND (x.id = s OR lint_short(x.id) = s);

  -- 행위(k = 장면 안 순번)와 입·퇴장을 한 줄로 세운다: (틈 g, sub) — 퇴장 0, 입장 1, 행위 2.
  CREATE TEMP TABLE IF NOT EXISTS render_row (g int, sub int, beat text, act jsonb, ents text[], fact text) ON COMMIT DROP;
  DELETE FROM render_row;
  CREATE TEMP TABLE IF NOT EXISTS render_act ON COMMIT DROP AS SELECT a.*, 0::bigint AS k FROM act a LIMIT 0;
  DELETE FROM render_act;
  INSERT INTO render_act SELECT a.*, row_number() OVER (ORDER BY a.ord) FROM act a WHERE a.work_id = w AND a.scene_id = sc.id;
  SELECT count(*) INTO n FROM render_act;

  INSERT INTO render_row
  SELECT a.k, 2, a.beat_id,
         jsonb_build_object('actor', lint_short(coalesce(a.resolved_actor_id, a.actor_id)), 'verb', a.verb,
           'targets', to_jsonb(ARRAY(SELECT lint_short(t) FROM unnest(a.objects || a."to") WITH ORDINALITY u(t, o)
                                      WHERE EXISTS (SELECT 1 FROM entity e WHERE e.id = u.t AND e.kind <> 'place') ORDER BY o)),
           'quote', a.quote,
           'action', trim(replace(a.action, '_', ' ') || ' ' || coalesce((SELECT string_agg(render_name(w, art.id, 'en'), ', ')
                                                                          FROM artifact art WHERE art.id = ANY (a.objects)), ''))),
         ARRAY[coalesce(a.resolved_actor_id, a.actor_id)] || ARRAY(SELECT t FROM unnest(a.objects || a."to") t
                                                              WHERE EXISTS (SELECT 1 FROM entity e WHERE e.id = t AND e.kind <> 'place')),
         lint_canon(w, coalesce(a.prop_id, (SELECT art.contains[1] FROM artifact art
                                             WHERE art.id = ANY (a.objects) AND a.verb IN ('reads', 'writes') LIMIT 1)))
    FROM render_act a;

  INSERT INTO render_row
  SELECT e.g, e.sub, (SELECT r.beat FROM render_act ra JOIN render_row r ON r.g = ra.k AND r.sub = 2
                       WHERE ra.k = least(e.g, n)),
         jsonb_build_object('actor', '', 'verb', e.verb, 'targets', to_jsonb(array_agg(lint_short(e.entity_id) ORDER BY e.entity_id))),
         array_agg(e.entity_id), NULL
    FROM (SELECT 'enter' AS verb, 1 AS sub, p.entity_id,
                 coalesce((SELECT ra.k + 1 FROM render_act ra WHERE ra.id = p.from_pt),
                          (SELECT min(ra.k) FROM render_act ra WHERE ra.beat_id = p.from_pt), 1) AS g
            FROM presence p WHERE p.work_id = w AND p.scene_id = sc.id AND p.from_pt IS DISTINCT FROM sc.start_pt
          UNION ALL
          SELECT 'exit', 0, p.entity_id,
                 coalesce((SELECT ra.k FROM render_act ra WHERE ra.id = p.to_pt),
                          (SELECT min(ra.k) FROM render_act ra WHERE ra.beat_id = p.to_pt), n + 1)
            FROM presence p WHERE p.work_id = w AND p.scene_id = sc.id) e
   GROUP BY e.g, e.sub, e.verb;

  -- 시점 인물: 지시어 pov 의 첫 값(짧은 이름).
  SELECT d."values"[1] INTO pov_v FROM directive d WHERE d.work_id = w AND d.scene_id = sc.id AND d.key = 'pov' LIMIT 1;
  SELECT e.id INTO pov FROM entity e WHERE e.work_id = w AND lint_short(e.id) = pov_v ORDER BY e.id LIMIT 1;

  SELECT array_agg(DISTINCT x) INTO cast_ids FROM (
    SELECT unnest(r.ents) x FROM render_row r UNION SELECT p.entity_id FROM presence p WHERE p.scene_id = sc.id UNION SELECT pov) c
   WHERE x IS NOT NULL;
  SELECT coalesce(array_agg(DISTINCT r.fact) FILTER (WHERE r.fact IS NOT NULL), '{}') INTO facts FROM render_row r;

  SELECT coalesce(array_agg(DISTINCT c), '{}') INTO held FROM (
    SELECT lint_canon(w, wh.prop_id) c, wh.until_pt, su.discourse_ord u_ord, sd.discourse_ord d_ord
      FROM withhold wh
      LEFT JOIN scene sd ON sd.id = wh.scene_id
      LEFT JOIN scene su ON su.work_id = w AND (su.start_pt = wh.until_pt OR wh.until_pt LIKE su.id || '#%')
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL) h
   WHERE (h.d_ord IS NULL OR h.d_ord <= sc.discourse_ord)
     AND (h.until_pt IS NULL OR h.u_ord > sc.discourse_ord)
     AND NOT h.c = ANY (facts)
     AND NOT EXISTS (SELECT 1 FROM reveal r JOIN scene sr ON sr.id = r.scene_id
                      WHERE r.work_id = w AND lint_canon(w, r.prop_id) = h.c AND sr.discourse_ord < sc.discourse_ord);

  -- 비트: 같은 비트 ID 가 이어지는 구간(섬)마다 하나.
  SELECT jsonb_agg(jsonb_build_object('id', coalesce(substring(b.beat from '#(.*)$'), ''), 'acts', b.acts) ORDER BY b.first) INTO beats FROM (
    SELECT i.beat, min(i.pos) first, jsonb_agg(i.act ORDER BY i.pos) acts FROM (
      SELECT r.beat, r.pos, r.act || jsonb_build_object('fact', coalesce(lint_short(r.fact), '')) act,
             r.pos - row_number() OVER (PARTITION BY r.beat ORDER BY r.pos) isl
        FROM (SELECT x.*, row_number() OVER (ORDER BY x.g, x.sub) pos FROM render_row x) r) i
     GROUP BY i.beat, i.isl) b;

  RETURN jsonb_build_object(
    'id', lint_short(sc.id),
    'title', coalesce(nullif(sc.title, ''), lint_short(sc.id)),
    'place', coalesce(render_name(w, sc.place_id, 'en'), ''),
    'previously', coalesce((
      SELECT string_agg(t.line, ' ' ORDER BY t.ord) FROM (
        SELECT s2.discourse_ord ord, coalesce(nullif(s2.title, ''), 'Scene ' || lint_short(s2.id)) || ': ' ||
               coalesce((SELECT string_agg(DISTINCT render_fact_text(w, lint_canon(w, c.prop_id)), '; ')
                           FROM act_content c JOIN act a ON a.id = c.act_id
                          WHERE a.scene_id = s2.id AND NOT lint_canon(w, c.prop_id) = ANY (held)), '(no facts)') || '.' line
          FROM scene s2 WHERE s2.work_id = w AND s2.shown AND sc.shown AND s2.discourse_ord < sc.discourse_ord
         ORDER BY s2.discourse_ord DESC LIMIT 3) t), ''),
    'cast', coalesce((SELECT jsonb_agg(jsonb_build_object('id', lint_short(x), 'names', coalesce(
               (SELECT jsonb_object_agg(l.lang, l.texts) FROM (SELECT nm.lang, jsonb_agg(nm.text ORDER BY nm.ord) texts
                  FROM name nm WHERE nm.work_id = w AND nm.owner_id = x AND nm.lang <> '' GROUP BY nm.lang) l),
               jsonb_build_object('en', jsonb_build_array(render_name(w, x, 'en'))))) ORDER BY x)
       FROM unnest(cast_ids) x), '[]'),
    'on_stage', to_jsonb(ARRAY(SELECT lint_short(x) FROM unnest(cast_ids) x
       WHERE EXISTS (SELECT 1 FROM presence p WHERE p.scene_id = sc.id AND p.entity_id = x AND p.from_pt = sc.start_pt)
          OR NOT EXISTS (SELECT 1 FROM presence p WHERE p.scene_id = sc.id AND p.entity_id = x) ORDER BY x)),
    'facts', coalesce((SELECT jsonb_agg(jsonb_build_object('id', lint_short(f), 'text', render_fact_text(w, f),
                                                          'true', render_fact_true(w, f)) ORDER BY f)
       FROM unnest(facts || held) f), '[]'),
    'beats', coalesce(beats, '[]'),
    'directives', jsonb_build_object(
       'pov', coalesce(lint_short(pov), ''),
       'pace', coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = sc.id AND d.key = 'pace' LIMIT 1), ''),
       'tension', coalesce((SELECT d."values"[1]::float8 FROM directive d WHERE d.scene_id = sc.id AND d.key = 'tension'
                              AND d."values"[1] ~ '^[0-9.]+$' LIMIT 1), 0),
       'focus', coalesce((SELECT to_jsonb(d."values") FROM directive d WHERE d.scene_id = sc.id AND d.key = 'focus' LIMIT 1), '[]'),
       'omit', coalesce((SELECT to_jsonb(d."values") FROM directive d WHERE d.scene_id = sc.id AND d.key = 'omit' LIMIT 1), '[]')),
    'pov_beliefs', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(facts) f
       WHERE pov IS NOT NULL AND lint_known(w, pov, f, sc.start_pt) = 2 ORDER BY f)),
    'reader_knows', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(facts) f WHERE sc.shown AND (
       EXISTS (SELECT 1 FROM act a JOIN scene s2 ON s2.id = a.scene_id
                WHERE a.work_id = w AND s2.shown AND s2.discourse_ord < sc.discourse_ord
                  AND (lint_canon(w, a.prop_id) = f OR f IN (SELECT lint_canon(w, p) FROM unnest(a."with") p)
                       OR (a.verb = 'reads' AND f IN (SELECT lint_canon(w, p) FROM artifact art, unnest(art.contains) p
                                                       WHERE art.id = ANY (a.objects)))))
       OR EXISTS (SELECT 1 FROM reveal r JOIN scene sr ON sr.id = r.scene_id
                   WHERE r.work_id = w AND lint_canon(w, r.prop_id) = f AND sr.discourse_ord < sc.discourse_ord)) ORDER BY f)),
    'withheld', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(held) f ORDER BY f)),
    'narration', coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = sc.id AND d.key = 'narration'
                             AND d."values"[1] IN ('first-person', 'third-person') LIMIT 1), ''));
END $$;
