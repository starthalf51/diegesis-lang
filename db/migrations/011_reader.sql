-- 011: 독자 모델(플랜 2 Task 7, Review Focus 2). 독자가 언제 무엇을 어떤 값으로 믿는지는 전부 여기서 정한다.
--   공개: 보여 준(shown) 장면의 단언(assertive: tells says whispers hails lies shows decrees sends)·지각(sees, reads 는
--     그 문서에 담긴 명제)·방백(aside)·생각(thinks)의 내용이 그 장면의 discourse_ord 에 독자에게 공개된다.
--     asks·orders·writes·does 는 공개하지 않는다. 보이지 않는 사건(event)·기록(account)의 행위도 공개하지 않는다.
--   독자는 들은 대로 믿는다: value = 단언된 값(거짓말도 그대로 믿는다). 명제는 (기저 사실, 값)으로 정규화한다
--     (`fact np = not p` 를 들으면 (p, false)). 나중에 반대 값이 공개되면 그 위치에서 갱신된다.
--     한 장면 안에서는 마지막 공개가 이긴다(장면 단위 — 위치는 장면 하나가 한 칸이다).
--   `withhold P until X`: withhold 가 든 장면의 위치부터 X 의 서술 위치(X 가 든 보이는 장면) 전까지 P 의 공개를 막는다.
--     막힌 공개는 사라지지 않고 X 의 위치로 미뤄진다(그 장면의 맨 앞). X 가 보이는 장면에 없으면 끝내 공개되지 않는다.
--   `reveal P [at T]`: 그 위치(T 가 든 보이는 장면, 없으면 reveal 이 든 장면)에 P 를 공개한다. withhold 가 막지 않는다.
-- ponytail: 위치 = 장면 단위 discourse_ord. 장면 안 순서는 마지막 공개를 고를 때만 쓴다. 문장 단위 위치가 필요하면 sub 를 내보낸다.
SET LOCAL search_path = public, ag_catalog;

-- 명제 p ≡ (base = val): 별칭은 따라가고 부정은 값을 뒤집는다. 행이 없는 `w/not x` 도 부정이다.
CREATE FUNCTION prop_norm(w text, p text, OUT base text, OUT val bool)
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb;
BEGIN
  base := p; val := true;
  FOR i IN 1..32 LOOP -- 별칭 순환 방지
    SELECT x.def INTO d FROM fact x WHERE x.work_id = w AND x.id = base;
    IF NOT FOUND AND base LIKE w || '/not %' THEN
      base := substr(base, length(w) + 6); val := NOT val;
    ELSIF d ? 'alias' THEN
      base := d->>'alias';
    ELSIF d ? 'not' THEN
      base := d->>'not'; val := NOT val;
    ELSE
      RETURN;
    END IF;
  END LOOP;
END $$;

-- 시점이 든 보이는 장면의 서술 위치(장면 시작·끝·비트·행위). 보이는 장면 밖이면 NULL.
CREATE FUNCTION point_ord(w text, pt text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT s.discourse_ord FROM scene s
   WHERE s.work_id = w AND s.shown AND (pt IN (s.start_pt, s.end_pt) OR starts_with(pt, s.id || '#'))
   ORDER BY s.discourse_ord LIMIT 1 $$;

-- 상태 술어: 시점마다 참거짓이 정해지는 기저 사실(holds(역할, x) = 역할 보유, alive(x) = 출생·사망 시점).
CREATE FUNCTION fact_is_state(w text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT coalesce((SELECT def->>'pred' IN ('holds', 'alive') FROM fact WHERE work_id = w AND id = b), false) $$;

-- 기저 사실 b 가 시점 at 에 참인가: true / false / NULL(정해지지 않음).
--   holds(r, x): x 의 r 보유 구간(role_span, 반열림 [from, until)) 하나가 at 을 확실히 덮으면 참, 모든 구간이 확실히
--     비켜 가면(구간이 없어도) 거짓. alive(x): x.born ≤ at < x.died 가 확실하면 참, at < born 이나 died ≤ at 이 확실하면 거짓.
--   그 밖의 사실은 시점과 무관하게 선언된 진리값(true/false). derived·unknown 은 NULL.
CREATE FUNCTION truth_at(w text, b text, at text) RETURNS bool
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE d jsonb; t text; x text; born text; died text;
BEGIN
  SELECT f.def, f.truth INTO d, t FROM fact f WHERE f.work_id = w AND f.id = b;
  IF d->>'pred' = 'holds' THEN
    IF at IS NULL THEN RETURN NULL; END IF;
    IF EXISTS (SELECT 1 FROM role_span s WHERE s.work_id = w AND s.role_id = d->'args'->>0 AND s.entity_id = d->'args'->>1
                  AND (s.from_pt IS NULL OR precedes(w, s.from_pt, at) = 'yes')
                  AND (s.until_pt IS NULL OR (precedes(w, at, s.until_pt) = 'yes' AND precedes(w, s.until_pt, at) <> 'yes'))) THEN
      RETURN true;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM role_span s WHERE s.work_id = w AND s.role_id = d->'args'->>0 AND s.entity_id = d->'args'->>1
                      AND NOT ((s.from_pt IS NOT NULL AND precedes(w, at, s.from_pt) = 'yes' AND precedes(w, s.from_pt, at) <> 'yes')
                            OR (s.until_pt IS NOT NULL AND precedes(w, s.until_pt, at) = 'yes'))) THEN
      RETURN false;
    END IF;
    RETURN NULL;
  ELSIF d->>'pred' = 'alive' THEN
    IF at IS NULL THEN RETURN NULL; END IF;
    x := d->'args'->>0;
    SELECT p.id INTO born FROM point p WHERE p.work_id = w AND p.id = x || '.born';
    SELECT p.id INTO died FROM point p WHERE p.work_id = w AND p.id = x || '.died';
    IF (born IS NOT NULL AND precedes(w, at, born) = 'yes' AND precedes(w, born, at) <> 'yes')
       OR (died IS NOT NULL AND precedes(w, died, at) = 'yes') THEN
      RETURN false;
    END IF;
    IF (born IS NULL OR precedes(w, born, at) = 'yes')
       AND (died IS NULL OR (precedes(w, at, died) = 'yes' AND precedes(w, died, at) <> 'yes')) THEN
      RETURN true;
    END IF;
    RETURN NULL;
  END IF;
  RETURN CASE t WHEN 'true' THEN true WHEN 'false' THEN false END;
END $$;

-- 독자 공개 하나하나: (기저 사실, 값, 서술 위치, 장면 안 순번, 경로, 행위). sub 는 같은 위치에서 마지막을 고르는 데만 쓴다.
CREATE FUNCTION reader_events(w text) RETURNS TABLE (fact_id text, value bool, ord int, sub int, via text, act_id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH told AS (
    SELECT c.prop_id, s.discourse_ord o, 2 * a.ord sub, a.verb, a.id
      FROM act a JOIN scene s ON s.id = a.scene_id
      CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") c(prop_id)
     WHERE a.work_id = w AND s.shown AND (assertive(a.verb) OR a.verb IN ('sees', 'aside', 'thinks'))
    UNION ALL
    SELECT p, s.discourse_ord, 2 * a.ord, a.verb, a.id
      FROM act a JOIN scene s ON s.id = a.scene_id JOIN artifact art ON art.id = ANY (a.objects)
      CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND s.shown AND a.verb = 'reads'),
  held AS (
    SELECT z.base, coalesce(sd.discourse_ord, 0) d_ord, point_ord(w, wh.until_pt) u_ord
      FROM withhold wh LEFT JOIN scene sd ON sd.id = wh.scene_id AND sd.shown
      CROSS JOIN LATERAL prop_norm(w, wh.prop_id) z
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL),
  acts AS (
    SELECT z.base, z.val, t.o, t.sub, t.verb, t.id, h.blocked, h.u_ord
      FROM told t CROSS JOIN LATERAL prop_norm(w, t.prop_id) z
      LEFT JOIN LATERAL (SELECT true blocked, h.u_ord FROM held h
                          WHERE h.base = z.base AND h.d_ord <= t.o AND (h.u_ord IS NULL OR h.u_ord > t.o)
                          ORDER BY h.u_ord DESC NULLS FIRST LIMIT 1) h ON true)
  SELECT base, val, CASE WHEN blocked THEN u_ord ELSE o END, CASE WHEN blocked THEN -1 ELSE sub END, verb, id
    FROM acts WHERE NOT coalesce(blocked, false) OR u_ord IS NOT NULL
  UNION ALL
  SELECT z.base, z.val, coalesce(point_ord(w, r.at_pt), sr.discourse_ord),
         coalesce(2 * (SELECT a.ord FROM act a WHERE a.id = r.at_pt) + 1, 1), 'reveal', NULL
    FROM reveal r LEFT JOIN scene sr ON sr.id = r.scene_id AND sr.shown
    CROSS JOIN LATERAL prop_norm(w, r.prop_id) z
   WHERE r.work_id = w AND r.prop_id IS NOT NULL AND coalesce(point_ord(w, r.at_pt), sr.discourse_ord) IS NOT NULL $$;

-- 독자 믿음의 변화점: from_ord 부터 독자는 fact_id = value 라고 믿는다(처음 공개, 그리고 값이 바뀔 때마다 한 행).
-- f 가 주어지면 그 명제의 기저 사실만(fact_id 는 기저 사실, value 도 기저 사실의 값이다). NULL 이면 모든 사실.
CREATE FUNCTION reader_knows(w text, f text DEFAULT NULL) RETURNS TABLE (fact_id text, from_ord int, value bool)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH e AS (SELECT * FROM reader_events(w) x WHERE f IS NULL OR x.fact_id = (SELECT base FROM prop_norm(w, f))),
  last AS (SELECT DISTINCT ON (e.fact_id, e.ord) e.fact_id, e.ord, e.value FROM e
            ORDER BY e.fact_id, e.ord, e.sub DESC, e.act_id DESC NULLS LAST),
  ch AS (SELECT l.*, lag(l.value) OVER (PARTITION BY l.fact_id ORDER BY l.ord) prev FROM last l)
  SELECT ch.fact_id, ch.ord, ch.value FROM ch WHERE ch.prev IS DISTINCT FROM ch.value ORDER BY 1, 2 $$;

-- 서술 위치 o 에서(그 장면의 공개까지 포함) 독자가 믿는 (사실, 값).
CREATE FUNCTION reader_state(w text, o int) RETURNS TABLE (fact_id text, value bool)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT DISTINCT ON (k.fact_id) k.fact_id, k.value FROM reader_knows(w) k WHERE k.from_ord <= o
   ORDER BY k.fact_id, k.from_ord DESC $$;

-- 인물 e 가 시점 at 에 기저 사실 fb 를 fv 로 아는가: 'yes' / 'opp'(반대로 믿는다) / 'maybe'(순서가 정해지지 않은 출처뿐) / 'no'.
--   출처 = e(또는 e 가 든 그룹)의 believes 행(의심이 거둔 행은 until_pt 부터 빠진다), e 자신이 단언(거짓말 제외)·방백·생각한 내용.
--   출처 시점은 at 보다 확실히 앞(같은 시점은 선언된 믿음만). 배경 지식(시점 없음)은 처음부터.
CREATE FUNCTION char_knows(w text, ent text, fb text, fv bool, at text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src AS (
    SELECT (z.val = b.value) v, b.from_pt pt, b.until_pt upt, b.declared
      FROM believes b JOIN lint_self_and_groups(w, ent) u ON u.id = b.entity_id
      CROSS JOIN LATERAL prop_norm(w, b.prop_id) z
     WHERE b.work_id = w AND z.base = fb
    UNION ALL
    SELECT z.val, c.pt, NULL, false
      FROM act_content c CROSS JOIN LATERAL prop_norm(w, c.prop_id) z
     WHERE c.work_id = w AND c.actor_id = ent AND z.base = fb
       AND (assertive(c.verb) AND c.verb <> 'lies' OR c.verb IN ('aside', 'thinks'))),
  held AS (
    SELECT v, (pt IS NULL OR ((pt <> at OR declared) AND precedes(w, pt, at) = 'yes'))
              AND (upt IS NULL OR (precedes(w, at, upt) = 'yes' AND precedes(w, upt, at) <> 'yes')) AS sure
      FROM src WHERE (pt IS NULL OR ((pt <> at OR declared) AND precedes(w, pt, at) <> 'no'))
                 AND (upt IS NULL OR precedes(w, upt, at) <> 'yes')) -- 확실히 거둬진 믿음은 maybe 도 아니다
  SELECT CASE WHEN bool_or(sure AND v = fv) THEN 'yes' WHEN bool_or(sure AND v <> fv) THEN 'opp'
              WHEN bool_or(v = fv) THEN 'maybe' ELSE 'no' END FROM held $$;
