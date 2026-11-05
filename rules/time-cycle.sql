-- rule: time-cycle
-- severity: warn
-- layer: derive
-- target: when
-- summary: 시간 제약이 순환한다(만족하는 일정이 없다)
--
-- 판정은 적재 때 solve_time()(db/migrations/002_time.sql)이 한다: 순서 순환(min ≥ 0 간선의 강연결요소)과
-- 그 밖의 음수 사이클마다 run_id 'derive' 진단 한 건(src = 사이클 간선 중 가장 앞선 문장, data.edges = time_edge.ord).
-- 억제(`#! allow time-cycle` 을 when/born/dies 에 붙임)는 그 간선을 풀이에서 빼고 진단을 waived 로 남긴다.
-- layer: derive 라서 린터는 이 파일을 실행하지 않고 diagnostic 에 이미 있는 행을 그대로 보여 준다.
-- 아래는 같은 행을 보는 조회다(사람이 psql 에서 쓸 때).
SELECT target_kind, target_id, message, data
  FROM diagnostic WHERE work_id = $1 AND run_id = 'derive' AND rule_id = 'time-cycle'
