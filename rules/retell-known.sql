-- rule: retell-known
-- severity: warn
-- layer: lint
-- target: act
-- summary: the same speaker tells a listener something the listener already believes, in the same words (and the listener's beliefs did not change in between)
--
-- Aims at duplicates left behind by revisions (the old place of a moved line). Literary repetition ("let me say it again") uses different words, so it is not caught.
-- Target: for each content proposition P of an assertive utterance (excluding lies, decrees, sends), listener B (excluding the speaker). sends hands delivery to a messenger, so it is not repetition.
-- Violation: B's believes(P, true) already arose from an earlier act of the same speaker (cause_act, point definitely earlier),
--   the quotes of the two acts are the same or one is empty (if both exist and differ, it is another telling in different words),
--   and B's belief did not change before the later act — no row where B came to believe not-P or to believe P false,
--   and no statement where B doubts P (`doubts` leaves no believes row, so look in the belief table).
-- Quiet if someone who does not know yet is mixed into the audience, or if the same speaker has already said the same thing more than twice as a motif (repetition by a senile elder, etc.).
-- Quiet if someone who does not know yet is mixed into the audience, or if the same speaker has already said the same thing more than twice as a motif (repetition by a senile elder, etc.).
-- A `believes` the author wrote directly (declared) is not counted as a source (an assumption from outside the work, so the repetition may be staging).
WITH told AS (
  SELECT DISTINCT c.act_id, c.pt, c.actor_id, au.entity_id AS hearer, c.prop_id, lint_canon($1, c.prop_id) AS cp, a.quote
    FROM act_content c JOIN act a ON a.id = c.act_id JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = $1 AND assertive(c.verb) AND c.verb NOT IN ('lies', 'decrees', 'sends')
     AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id),
cand AS MATERIALIZED (  -- cand AS MATERIALIZED (  -- filter on the cheap conditions first (same speaker, listener, proposition, quote) to cut precedes calls
  SELECT t.*, k.cause_act, k.from_pt AS k_pt
    FROM told t
    JOIN believes k ON k.work_id = $1 AND k.entity_id = t.hearer AND k.value AND NOT k.declared
                   AND lint_canon($1, k.prop_id) = t.cp AND k.from_pt IS NOT NULL AND k.from_pt <> t.pt
    JOIN act pa ON pa.id = k.cause_act AND pa.resolved_actor_id = t.actor_id
               AND (pa.quote = '' OR t.quote = '' OR pa.quote = t.quote))
SELECT DISTINCT ON (t.act_id, t.hearer, t.cp)
       'act' AS target_kind, t.act_id AS target_id,
       format('%s tells %s %s again; already believed since %s', lint_short(t.actor_id), lint_short(t.hearer), lint_short(t.prop_id), lint_short(t.cause_act)) AS message,
       jsonb_build_object('hearer', t.hearer, 'prop', t.prop_id, 'first', t.cause_act) AS data
  FROM cand t
 WHERE precedes($1, t.k_pt, t.pt) = 'yes'
   AND NOT EXISTS (  -- AND NOT EXISTS (  -- an utterance with a new listener mixed in (an announcement, a late revelation) is not repetition: everyone in the audience must already know
   SELECT 1 FROM audience au
    WHERE au.act_id = t.act_id AND au.entity_id <> t.actor_id
      AND NOT EXISTS (SELECT 1 FROM cand u WHERE u.act_id = t.act_id AND u.cp = t.cp AND u.hearer = au.entity_id))
   AND (SELECT count(DISTINCT u.cause_act) FROM cand u WHERE u.act_id = t.act_id AND u.hearer = t.hearer AND u.cp = t.cp) < 2
   AND NOT EXISTS (
   SELECT 1 FROM believes x
    WHERE x.work_id = $1 AND x.entity_id = t.hearer AND x.from_pt IS NOT NULL
      AND ((x.value = false AND lint_canon($1, x.prop_id) = t.cp)
        OR (x.value AND lint_opposite($1, lint_canon($1, x.prop_id), t.cp)))
      AND precedes($1, t.k_pt, x.from_pt) = 'yes' AND precedes($1, x.from_pt, t.pt) <> 'no')
   AND NOT EXISTS (
   SELECT 1 FROM belief d
    WHERE d.work_id = $1 AND d.entity_id = t.hearer AND d.verb = 'doubts' AND d.at_pt IS NOT NULL
      AND lint_canon($1, d.prop_id) = t.cp
      AND precedes($1, t.k_pt, d.at_pt) = 'yes' AND precedes($1, d.at_pt, t.pt) <> 'no')
 ORDER BY t.act_id, t.hearer, t.cp, t.cause_act
