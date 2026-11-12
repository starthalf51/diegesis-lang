-- rule: reader-unknown-term
-- severity: error
-- layer: lint
-- target: act
-- summary: 작품 고유 용어가 설명되기 전에 독자에게 닿는다
--
-- Text that reaches the reader in a shown scene: the text of facts an act carries (prop and with), act quotes, `does` actions
--   (target = that act), and the scene's `render focus` strings (target = directive, id = the scene).
-- Violation: a term surface hits that text (term_hit, db/migrations/019_reader_terms.sql) and the term has no `explain` at or before
--   the scene's told position (term_explained_at; told order applies `discourse after`). One row per target and term (longest surface).
WITH told AS (SELECT * FROM scene_told_ord($1)),
expl AS (SELECT * FROM term_explained_at($1)),
txt AS (
  SELECT 'act' AS k, a.id, a.scene_id, f.text AS t
    FROM act a JOIN fact f ON f.work_id = $1 AND f.id = ANY (a.prop_id || a."with") WHERE a.work_id = $1
  UNION ALL SELECT 'act', a.id, a.scene_id, a.quote FROM act a WHERE a.work_id = $1 AND a.quote <> ''
  UNION ALL SELECT 'act', a.id, a.scene_id, a.action FROM act a WHERE a.work_id = $1 AND a.action <> ''
  UNION ALL SELECT 'directive', d.scene_id, d.scene_id, v FROM directive d, unnest(d."values") v
             WHERE d.work_id = $1 AND d.key = 'focus'),
hit AS (
  SELECT DISTINCT ON (x.k, x.id, e.id) x.k, x.id, e.id AS term, n.text AS surface
    FROM txt x JOIN told o ON o.scene_id = x.scene_id
    JOIN entity e ON e.work_id = $1 AND e.kind = 'term'
    JOIN name n ON n.work_id = $1 AND n.owner_id = e.id
    LEFT JOIN expl ON expl.term_id = e.id
   WHERE (expl.ord IS NULL OR expl.ord > o.ord) AND term_hit(x.t, n.text)
   ORDER BY x.k, x.id, e.id, length(n.text) DESC, n.ord)
SELECT k AS target_kind, id AS target_id,
       format('term "%s" reaches the reader before it is explained', surface) AS message,
       jsonb_build_object('term', term, 'surface', surface) AS data
  FROM hit
