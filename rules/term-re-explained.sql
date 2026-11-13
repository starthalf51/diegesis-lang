-- rule: term-re-explained
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 이미 설명된 용어를 다시 explain 한다 (recap 을 쓴다)
--
-- A scene's `render explain` names a term that an earlier scene in the telling (scene_told_ord, `discourse after` applied)
--   already explains. The reader stack accumulates: use `recap` for a brief reminder.
WITH told AS (SELECT * FROM scene_told_ord($1)),
expl AS (SELECT * FROM term_explained_at($1))
SELECT DISTINCT 'scene' AS target_kind, d.scene_id AS target_id,
       format('term "%s" is explained again; use recap for a reminder', render_name($1, v, 'en')) AS message,
       jsonb_build_object('term', v) AS data
  FROM directive d JOIN told o ON o.scene_id = d.scene_id, unnest(d."values") v
  JOIN expl ON expl.term_id = v
 WHERE d.work_id = $1 AND d.key = 'explain' AND expl.ord < o.ord
