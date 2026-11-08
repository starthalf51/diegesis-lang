-- rule: fact-conflict
-- severity: warn
-- layer: lint
-- target: fact
-- summary: 같은 술어·같은 첫 인자인데 나머지 인자가 다른 참 사실이 둘 있다 (예: broke(lens, A) 와 broke(lens, B))
--
-- 참 사실 = `fact x = pred(args)` 로 선언된(별칭이 가리키는) 술어 행. `not` 이나 별칭 행, 추론 중 생긴 술어 행은 세지 않는다.
-- 한 쌍마다 한 번(작은 id 쪽)에 낸다. 정말 둘 다 참인 술어(여럿이 가능한 관계)면 waive 한다.
WITH pf AS (
  SELECT f.id, f.src_id, f.def->>'pred' AS pred, f.def->'args' AS args
    FROM fact f
   WHERE f.work_id = $1 AND f.def ? 'pred' AND jsonb_array_length(f.def->'args') >= 2
     AND EXISTS (SELECT 1 FROM fact x WHERE x.work_id = $1 AND x.def->>'alias' = f.id))
SELECT 'fact' AS target_kind, a.id AS target_id, a.src_id,
       format('%s conflicts with %s (same predicate and first argument, different rest)', lint_short(a.id), lint_short(b.id)) AS message,
       jsonb_build_object('other', b.id) AS data
  FROM pf a JOIN pf b ON a.pred = b.pred AND a.args->0 = b.args->0 AND a.id < b.id AND a.args <> b.args
