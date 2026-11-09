-- rule: stays-gone
-- severity: warn
-- layer: lint
-- target: act
-- summary: 이야기에서 떠났다(status gone)고 선언된 인물이 다시 행동하거나 등장한다
--
-- Convention: when a character leaves the story `attr <X>.status = gone`, when they return `attr <X>.status = present`.
-- Target: acts of actor X (target act), and X's stage appearance (enter; presence rows have no id, so
--   target = presence, id = <scene>/<X>, source position is the enter's src_id).
-- Value valid at the point: same as object-location — among X.status assertions outside an account, those necessarily before the target (precedes = yes),
--   with no other X.status assertion in between. If several values are valid it is quiet unless every one is gone
--   (if the order is undetermined they may have come back).
-- Violation: there are valid values and all are gone.
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
