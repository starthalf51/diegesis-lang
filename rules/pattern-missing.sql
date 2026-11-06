-- rule: pattern-missing
-- severity: warn
-- layer: lint
-- target: pattern
-- summary: 작가가 선언한 서사 패턴이 작품에서 검출되지 않았다
--
-- 대상: `pattern <id> [at <scene>]` 선언 하나하나(억제 대상 = 그 선언, ID `<장면 또는 모듈>#<패턴>`).
-- 위반: 같은 패턴의 매치 중 선언을 덮는 것(pattern_covers: 장면 없는 선언은 아무 매치, 장면이 있으면 그 장면에 걸린 매치)이 없다.
-- 매치는 lint 가 규칙보다 먼저 선언된 패턴만 돌려 pattern_match 에 넣는다(internal/pg/lint.go). 없는 패턴 ID 도 여기서 걸린다.
SELECT 'pattern' AS target_kind, d.id AS target_id, d.src_id,
       format('pattern %s is declared%s but not found', d.pattern_id,
              CASE WHEN d.scene_id IS NULL THEN '' ELSE ' at ' || lint_short(d.scene_id) END) AS message,
       jsonb_build_object('pattern', d.pattern_id, 'scene', d.scene_id) AS data
  FROM pattern_decl d
 WHERE d.work_id = $1
   AND NOT EXISTS (SELECT 1 FROM pattern_match m
                    WHERE m.work_id = $1 AND m.pattern_id = d.pattern_id
                      AND pattern_covers($1, d.scene_id, m.target_id, m.span_from, m.span_to))
