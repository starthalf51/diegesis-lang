-- 012: 서사 패턴(플랜 2 Task 7). 패턴 정의는 patterns/*.sql(파일 하나가 패턴 하나, 본문은 $1 = 작품인 SELECT)이고
--   러너(internal/pg/patterns.go)는 그 결과를 pattern_match 에 옮겨 담을 뿐이다. 판정은 패턴 SQL 과 011 독자 모델이 한다.
--   작가 선언 `pattern <id> [at <scene>]` 은 pattern_decl 이다. 선언했는데 매치가 없으면 lint 규칙 pattern-missing(warn).
--   매치됐는데 선언이 없으면 보고만 한다(진단 아님).
--   fact.tag: `fact … tag <name>`(R3). tag identity = 정체 사실(recognition 패턴이 고른다).
SET LOCAL search_path = public, ag_catalog;

ALTER TABLE fact ADD COLUMN tag text;

CREATE TABLE pattern_decl (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text NOT NULL, pattern_id text NOT NULL, scene_id text, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- span = discourse_ord 구간(보이지 않는 장면만 걸린 매치는 NULL). 작품마다 마지막 실행의 행만 남는다.
CREATE TABLE pattern_match (
  run_id text NOT NULL, work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  pattern_id text NOT NULL, target_kind text NOT NULL, target_id text NOT NULL,
  span_from int, span_to int, data jsonb);
CREATE INDEX ON pattern_match (work_id, pattern_id);

-- 선언이 매치를 덮는가. 장면 없는 선언 = 같은 패턴의 아무 매치. 장면이 있으면: 대상이 그 장면이거나 그 장면 안의 것
-- (행위·비트 `장면#…`), 또는 매치 구간이 그 장면의 서술 위치를 덮는다.
CREATE FUNCTION pattern_covers(w text, scene_id text, target_id text, span_from int, span_to int) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT scene_id IS NULL OR target_id = scene_id OR starts_with(target_id, scene_id || '#')
      OR EXISTS (SELECT 1 FROM scene s WHERE s.work_id = w AND s.id = scene_id AND s.shown
                    AND s.discourse_ord BETWEEN span_from AND span_to) $$;

-- 예언자: 자신 또는 속한 그룹이 knows oracle(omniscient 는 예언자가 아니다).
CREATE FUNCTION is_oracle(w text, e text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM entity x JOIN lint_self_and_groups(w, e) u ON u.id = x.id
                  WHERE x.work_id = w AND x.knows = 'oracle') $$;

-- 행위가 전하는 명제를 정규화한 것(장면의 서술 위치 포함). reads 는 문서에 담긴 명제. via = 동사.
CREATE FUNCTION act_props(w text) RETURNS TABLE (act_id text, scene_id text, pt text, actor_id text, verb text,
  prop_id text, base text, val bool, ord int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH c AS (
    SELECT a.id, a.scene_id, a.pt, a.resolved_actor_id, a.verb, p
      FROM act a CROSS JOIN LATERAL unnest(array_remove(ARRAY[a.prop_id], NULL) || a."with") p WHERE a.work_id = w
    UNION ALL
    SELECT a.id, a.scene_id, a.pt, a.resolved_actor_id, a.verb, p
      FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
     WHERE a.work_id = w AND a.verb IN ('reads', 'writes'))
  SELECT c.id, c.scene_id, c.pt, c.resolved_actor_id, c.verb, c.p, z.base, z.val,
         (SELECT s.discourse_ord FROM scene s WHERE s.id = c.scene_id AND s.shown)
    FROM c CROSS JOIN LATERAL prop_norm(w, c.p) z $$;
