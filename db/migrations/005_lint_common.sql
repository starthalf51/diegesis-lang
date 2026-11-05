-- 005: 린트 규칙(rules/*.sql)이 함께 쓰는 판정 조각. 규칙마다의 의미는 각 규칙 파일 헤더에 있다.
SET LOCAL search_path = public, ag_catalog;

-- 정규 명제: 별칭(`fact x = Q`)이면 Q, 아니면 그대로. ponytail: 한 단계만 푼다(별칭의 별칭은 그대로).
CREATE FUNCTION lint_canon(w text, p text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT coalesce((SELECT def->>'alias' FROM fact WHERE work_id = w AND id = p), p) $$;

-- e 와 e 가 (간접) 속한 그룹들. 그룹의 믿음·면제는 구성원이 물려받는다.
CREATE FUNCTION lint_self_and_groups(w text, e text) RETURNS TABLE (id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH RECURSIVE up(id) AS (
    SELECT e UNION SELECT g.group_id FROM group_member g JOIN up ON g.member_id = up.id WHERE g.work_id = w)
  SELECT id FROM up $$;

-- 정보 흐름 면제: 자신 또는 속한 그룹이 knows oracle|omniscient.
CREATE FUNCTION lint_exempt(w text, e text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM entity x JOIN lint_self_and_groups(w, e) u ON u.id = x.id
                  WHERE x.work_id = w AND x.knows IN ('oracle', 'omniscient')) $$;

-- 두 정규 명제가 서로 부정인가(`not P` 와 P). 인자는 lint_canon 을 거친 ID.
CREATE FUNCTION lint_opposite(w text, a text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM fact f WHERE f.work_id = w AND f.def ? 'not'
                    AND ((f.id = a AND lint_canon(w, f.def->>'not') = b)
                      OR (f.id = b AND lint_canon(w, f.def->>'not') = a))) $$;

-- e 가 시점 at 에 정규 명제 c 를 아는 정도: 2 = 확정, 1 = 순서가 정해지지 않은 출처(maybe)뿐, 0 = 출처 없음.
-- 출처 = e 또는 e 가 속한 그룹의 believes(P, true), 그리고 그들의 decrees P(수행문: 선포한 사람은 안다).
-- 출처 시점 NULL = 처음부터(배경 지식, 최상위·선언 블록의 believes) → 확정. 그 밖에는 precedes(출처, at).
-- 출처 시점이 at 과 같으면(같은 행위) 작가가 선언한 믿음(declared)일 때만 센다.
CREATE FUNCTION lint_known(w text, e text, c text, at text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src(pt, declared) AS (
    SELECT b.from_pt, b.declared FROM believes b JOIN lint_self_and_groups(w, e) u ON u.id = b.entity_id
     WHERE b.work_id = w AND b.value AND lint_canon(w, b.prop_id) = c
    UNION ALL
    SELECT x.pt, false FROM act_content x JOIN lint_self_and_groups(w, e) u ON u.id = x.actor_id
     WHERE x.work_id = w AND x.verb = 'decrees' AND lint_canon(w, x.prop_id) = c)
  SELECT coalesce(max(CASE WHEN pt IS NULL THEN 2
                           ELSE CASE precedes(w, pt, at) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END), 0)
    FROM src WHERE pt IS DISTINCT FROM at OR declared $$;

-- 진단 메시지용 짧은 이름: 'w/mod/x' → 'x', 'w/holds(w/m/r,w/m/x)' → 'holds(r,x)'.
CREATE FUNCTION lint_short(x text) RETURNS text LANGUAGE sql IMMUTABLE
  AS $$ SELECT regexp_replace(x, '[^(), ]*/', '', 'g') $$;
