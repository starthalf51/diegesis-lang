-- rule: fact-conflict
-- severity: warn
-- layer: lint
-- target: fact
-- summary: 같은 술어·같은 첫 인자인데 나머지 인자가 다른 참 사실이 둘 있다 (예: broke(lens, A) 와 broke(lens, B)), 또는 같은 선두 토큰의 텍스트 사실이 다른 파일에 둘 있다(후보)
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
UNION ALL
-- 텍스트 사실 휴리스틱(후보): 생성 이야기는 술어 없이 `fact x "text" true` 만 쓴다. 술어 정의 없는 참 사실 둘이
--   서로 다른 파일에 있고, id 의 선두 토큰(첫 `_` 앞, 예: lens_smashed → lens)이 같으며, 둘 다 서술되면(행위 내용 또는 독자 공개) 후보로 낸다.
--   같은 파일 안의 같은 토큰은 한 번에 쓴 사실 묶음이라 세지 않는다.
--   선두 토큰만으로는 run3 B 한 편에서 150건(공유 3개면 21건, 4개면 7건 중 6건 오탐)이 울려서, 같은 사건을 다르게 말하는 쌍으로 좁힌다:
--   두 텍스트가 어절 앞 두 글자를 5개 이상 공유한다(렌즈·깨뜨·도경태 같은 주어·동작·행위자). 조사·활용에 둔감하려는 싼 근사다.
SELECT 'fact', a.id, a.src_id,
       format('candidate conflict, check: %s and %s are both true text facts about %s in different files', lint_short(a.id), lint_short(b.id), split_part(lint_short(a.id), '_', 1)),
       jsonb_build_object('other', b.id, 'heuristic', 'text-fact')
  FROM (SELECT f.id, f.src_id, g.file, split_part(lint_short(f.id), '_', 1) AS tok
          FROM fact f JOIN src g ON g.work_id = $1 AND g.id = f.src_id
         WHERE f.work_id = $1 AND f.def IS NULL AND f.truth = 'true' AND position('_' IN lint_short(f.id)) > 1
           AND (EXISTS (SELECT 1 FROM act_content c WHERE c.work_id = $1 AND lint_canon($1, c.prop_id) = f.id)
             OR EXISTS (SELECT 1 FROM reader_events($1) r WHERE r.fact_id = f.id))) a
  JOIN (SELECT f.id, f.src_id, g.file, split_part(lint_short(f.id), '_', 1) AS tok
          FROM fact f JOIN src g ON g.work_id = $1 AND g.id = f.src_id
         WHERE f.work_id = $1 AND f.def IS NULL AND f.truth = 'true' AND position('_' IN lint_short(f.id)) > 1
           AND (EXISTS (SELECT 1 FROM act_content c WHERE c.work_id = $1 AND lint_canon($1, c.prop_id) = f.id)
             OR EXISTS (SELECT 1 FROM reader_events($1) r WHERE r.fact_id = f.id))) b
    ON a.tok = b.tok AND a.id < b.id AND a.file <> b.file
 WHERE (SELECT count(*) FROM (SELECT left(w, 2) FROM regexp_split_to_table((SELECT text FROM fact WHERE id = a.id), '[\s,.''"]+') w WHERE length(w) >= 2
                              INTERSECT
                              SELECT left(w, 2) FROM regexp_split_to_table((SELECT text FROM fact WHERE id = b.id), '[\s,.''"]+') w WHERE length(w) >= 2) z) >= 5
