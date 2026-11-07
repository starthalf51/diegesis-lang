-- rule: stays-gone
-- severity: warn
-- layer: lint
-- target: act
-- summary: 이야기에서 떠났다(status gone)고 선언된 인물이 다시 행동하거나 등장한다
--
-- 규약: 인물이 이야기에서 떠나면 `attr <X>.status = gone`, 돌아오면 `attr <X>.status = present`.
-- 대상: 행위자 X 의 행위(target act), 그리고 X 의 무대 출석(enter; presence 행에는 id 가 없어
--   target = presence, id = <장면>/<X>, 소스 위치는 enter 의 src_id).
-- 시점에 유효한 값: object-location 과 같다 — account 밖의 X.status 단언 중 대상보다 반드시 앞서고(precedes = yes),
--   그 뒤에 또 다른 X.status 단언이 끼지 않은 것. 유효한 값이 여럿이면 하나라도 gone 이 아니면 조용하다
--   (순서가 정해지지 않았으면 되돌아온 것일 수 있다).
-- 위반: 유효한 값이 있고 모두 gone 이다.
WITH origin AS (SELECT id FROM point WHERE work_id = $1 AND kind = 'origin' ORDER BY id LIMIT 1),
st AS (
  SELECT x.entity_id, x.value, x.src_id, coalesce(x.at_pt, (SELECT id FROM origin)) AS pt
    FROM attribute x
   WHERE x.work_id = $1 AND x.key = 'status' AND x.account_id IS NULL),
ev AS (
  SELECT 'act' AS kind, a.id AS tid, a.resolved_actor_id AS ent, a.pt, a.src_id, coalesce(nullif(a.action, ''), a.verb) AS what
    FROM act a WHERE a.work_id = $1 AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT 'presence', p.scene_id || '/' || p.entity_id, p.entity_id, p.from_pt, p.src_id, 'enters'
    FROM presence p WHERE p.work_id = $1 AND p.from_pt IS NOT NULL),
cur AS (
  SELECT e.*, s.value, s.src_id AS s_src
    FROM ev e JOIN st s ON s.entity_id = e.ent AND precedes($1, s.pt, e.pt) = 'yes'
   WHERE NOT EXISTS (SELECT 1 FROM st q WHERE q.entity_id = s.entity_id AND q.pt <> s.pt
                        AND precedes($1, s.pt, q.pt) = 'yes' AND precedes($1, q.pt, e.pt) = 'yes')),
bad AS (
  SELECT kind, tid, ent, what, src_id, min(s_src) AS s_src FROM cur
   GROUP BY kind, tid, ent, what, src_id HAVING bool_and(value = 'gone'))
SELECT b.kind AS target_kind, b.tid AS target_id, b.src_id,
       format('%s %s after leaving (status gone at %s:%s)', lint_short(b.ent), b.what, g.file, g.line) AS message,
       jsonb_build_object('entity', b.ent, 'gone_src_id', b.s_src) AS data
  FROM bad b LEFT JOIN src g ON g.work_id = $1 AND g.id = b.s_src
