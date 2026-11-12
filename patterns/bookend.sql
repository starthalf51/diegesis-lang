-- pattern: bookend
-- layer: structure
-- summary: the opening and closing scenes mirror each other (characters, places, facts) more than the opening mirrors the middle
--
-- Match: one row per work if metric_structure's bookend >= 0.2 (front-back Jaccard minus front-middle Jaccard, 10% windows).
-- Target = the work; span = the first shown scene's narration position ~ the last's.
SELECT 'work' AS target_kind, w.id AS target_id,
       (SELECT min(s.discourse_ord) FROM scene s WHERE s.work_id = w.id AND s.shown) AS span_from,
       (SELECT max(s.discourse_ord) FROM scene s WHERE s.work_id = w.id AND s.shown) AS span_to,
       jsonb_build_object('bookend', m.value) AS data
  FROM work w CROSS JOIN LATERAL metric_structure($1) m
 WHERE w.id = $1 AND m.term = 'bookend' AND m.value >= 0.2
