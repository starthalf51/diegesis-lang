-- pattern: dramatic-irony
-- layer: information
-- summary: the reader knows a true P, and a character in the same scene does not know P or believes the opposite (per scene)
--
-- For each visible scene s: among the (b, v) the reader has come to believe before s (narration position < s), those true at the start of s (truth_at = v),
-- where that fact is dealt with in s (an act in s conveys or asks it, or a belief/doubt statement in s is about that fact — unrelated facts are not counted),
-- and among the characters present (presence) in s there is one who at the start of s believes the opposite (char_knows = opp), or has no source (no) but in that scene
-- touched that fact (the actor or audience of an act conveying that fact, or the subject of a belief/doubt statement about it).
-- If there are only sources with undetermined order (maybe), it is not counted. A character with no record who is merely unrelated to the fact is not counted (noise).
-- Prophets and omniscients (lint_exempt) are excluded, and a character who said or thought that fact earlier knows it (a source for char_knows).
-- Target = scene. span = that scene's one slot. data.irony = (character, fact, value, the character's knowledge).
WITH kn AS (SELECT * FROM reader_knows($1)),
sc AS (SELECT s.id, s.discourse_ord AS ord, s.start_pt FROM scene s WHERE s.work_id = $1 AND s.shown),
k AS (
  SELECT sc.id AS scene_id, sc.ord, sc.start_pt, r.fact_id, r.value
    FROM sc CROSS JOIN LATERAL (SELECT DISTINCT ON (kn.fact_id) kn.fact_id, kn.value FROM kn
                                 WHERE kn.from_ord < sc.ord ORDER BY kn.fact_id, kn.from_ord DESC) r
   WHERE truth_at($1, r.fact_id, sc.start_pt) = r.value),
inplay AS (
  SELECT a.scene_id, a.base FROM act_props($1) a
  UNION
  SELECT b.scene_id, z.base FROM belief b CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
   WHERE b.work_id = $1 AND b.scene_id IS NOT NULL AND b.prop_id IS NOT NULL),
engaged AS (  -- engaged AS (  -- characters who touched that fact in that scene: actor/audience of an act conveying it, subject of a belief/doubt statement about it
  SELECT a.scene_id, a.base, a.actor_id AS entity_id FROM act_props($1) a
  UNION
  SELECT a.scene_id, a.base, au.entity_id FROM act_props($1) a JOIN audience au ON au.act_id = a.act_id
  UNION
  SELECT b.scene_id, z.base, b.entity_id FROM belief b CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
   WHERE b.work_id = $1 AND b.scene_id IS NOT NULL AND b.prop_id IS NOT NULL),
ir AS (
  SELECT k.scene_id, k.ord, k.fact_id, k.value, p.entity_id,
         char_knows($1, p.entity_id, k.fact_id, k.value, k.start_pt) AS knows,
         EXISTS (SELECT 1 FROM engaged g WHERE g.scene_id = k.scene_id AND g.base = k.fact_id AND g.entity_id = p.entity_id) AS eng
    FROM k JOIN inplay i ON i.scene_id = k.scene_id AND i.base = k.fact_id
    JOIN (SELECT DISTINCT entity_id, scene_id FROM presence WHERE work_id = $1) p ON p.scene_id = k.scene_id
   WHERE NOT lint_exempt($1, p.entity_id))
SELECT 'scene' AS target_kind, scene_id AS target_id, ord AS span_from, ord AS span_to,
       jsonb_build_object('irony', jsonb_agg(jsonb_build_object('character', entity_id, 'fact', fact_id, 'value', value,
                                                                'character_knows', knows) ORDER BY entity_id, fact_id)) AS data
  FROM ir WHERE knows = 'opp' OR (knows = 'no' AND eng)
 GROUP BY scene_id, ord
