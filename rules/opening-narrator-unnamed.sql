-- rule: opening-narrator-unnamed
-- severity: warn
-- layer: lint
-- target: scene
-- summary: 1인칭 화자가 오프닝 모듈에서 한 번도 이름으로 불리지 않는다
--
-- The module (file) holding the first shown scene in telling order. If one of its shown scenes has `narration first-person` and a `pov`, and the pov character's name
--   (name table, any language, or the entity name) appears in no act quote and no act fact sentence (render_fact_text) of that module, report once, on the first such scene.
--   Third person, no narration, and no pov are never reported. Matching is a case-insensitive substring (Korean particles attach to names).
WITH first_mod AS (
  SELECT s.module_id FROM scene s JOIN scene_told_ord($1) o ON o.scene_id = s.id
   WHERE s.work_id = $1 ORDER BY o.ord LIMIT 1),
fp AS (
  SELECT s.id, o.ord, lint_short(p."values"[1]) AS pov
    FROM scene s
    JOIN first_mod m ON s.module_id IS NOT DISTINCT FROM m.module_id
    JOIN scene_told_ord($1) o ON o.scene_id = s.id
    JOIN directive p ON p.work_id = $1 AND p.scene_id = s.id AND p.key = 'pov'
    JOIN directive n ON n.work_id = $1 AND n.scene_id = s.id AND n.key = 'narration' AND n."values"[1] = 'first-person'
   WHERE s.work_id = $1 AND s.shown),
top AS (SELECT * FROM fp ORDER BY ord LIMIT 1),
names AS (
  SELECT n.text FROM name n, top WHERE n.work_id = $1 AND lint_short(n.owner_id) = top.pov
  UNION
  SELECT e.name FROM entity e, top WHERE e.work_id = $1 AND lint_short(e.id) = top.pov),
spoken AS (
  SELECT a.quote AS t FROM act a JOIN scene s ON s.id = a.scene_id JOIN first_mod m ON s.module_id IS NOT DISTINCT FROM m.module_id
   WHERE a.work_id = $1 AND a.quote <> ''
  UNION ALL
  SELECT render_fact_text($1, a.prop_id) FROM act a JOIN scene s ON s.id = a.scene_id JOIN first_mod m ON s.module_id IS NOT DISTINCT FROM m.module_id
   WHERE a.work_id = $1 AND a.prop_id IS NOT NULL)
SELECT 'scene' AS target_kind, top.id AS target_id,
       format('the first-person narrator %s is never named in the opening module; let a line of dialogue or a fact give the reader the name', top.pov) AS message,
       jsonb_build_object('pov', top.pov) AS data
  FROM top
 WHERE NOT EXISTS (SELECT 1 FROM names nm, spoken sp WHERE nm.text <> '' AND position(lower(nm.text) IN lower(sp.t)) > 0);
