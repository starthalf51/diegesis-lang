-- rule: brief-term-unshown
-- severity: info
-- layer: lint
-- target: scene
-- summary: 설명하는 용어에 그것을 보여 줄 사건(shown_by)이 없다
--
-- Data gap for `dieg brief`: a scene's `render explain` names a term with no `attr <term>.shown_by = <fact>`, so the
--   brief can only hand the writer a bare label. Reported only in a work that uses shown_by at all.
SELECT DISTINCT 'scene' AS target_kind, d.scene_id AS target_id,
       format('term %s is explained here but no fact shows it (attr <term>.shown_by)', render_name($1, v, 'en')) AS message,
       jsonb_build_object('term', v) AS data
  FROM directive d, unnest(d."values") v
 WHERE d.work_id = $1 AND d.key = 'explain'
   AND EXISTS (SELECT 1 FROM attribute x WHERE x.work_id = $1 AND x.key = 'shown_by')
   AND NOT EXISTS (SELECT 1 FROM attribute s WHERE s.work_id = $1 AND s.entity_id = v AND s.key = 'shown_by')
