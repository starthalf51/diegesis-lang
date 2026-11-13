-- rule: term-overload
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 한 장면에서 처음 설명되는 용어가 3개를 넘는다
--
-- A shown scene whose told position is the first explanation (term_explained_at, same basis as render terms_new) of more than 3 terms. Recaps and re-explanations do not count.
SELECT 'scene' AS target_kind, s.id AS target_id,
       format('scene introduces %s new terms (more than 3); split the scene or defer some terms', count(*)) AS message,
       jsonb_build_object('terms', jsonb_agg(t.term_id ORDER BY t.term_id)) AS data
  FROM scene s
  JOIN scene_told_ord($1) o ON o.scene_id = s.id
  JOIN term_explained_at($1) t ON t.ord = o.ord
 WHERE s.work_id = $1 AND s.shown
 GROUP BY s.id
HAVING count(*) > 3;
