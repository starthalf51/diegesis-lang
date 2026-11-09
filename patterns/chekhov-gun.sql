-- pattern: chekhov-gun
-- layer: foreshadow
-- summary: a promise setup is paid off (payoff) — Chekhov's gun
--
-- Match: for each setup with kind = promise, one row if at least one payoff points at that setup. Target = the setup.
-- span = the setup's narration position ~ the earliest payoff's narration position (NULL for an unseen scene or top level).
-- Whether the payoff comes before the setup is the payoff-without-setup rule's job (here only that both exist).
SELECT 'setup' AS target_kind, s.id AS target_id,
       (SELECT x.discourse_ord FROM scene x WHERE x.id = s.scene_id AND x.shown) AS span_from,
       min(sp.discourse_ord) AS span_to,
       jsonb_build_object('prop', s.prop_id, 'text', s.text,
                          'payoffs', jsonb_agg(DISTINCT p.scene_id)) AS data
  FROM setup s
  JOIN payoff p ON p.work_id = s.work_id AND p.setup_id = s.id
  LEFT JOIN scene sp ON sp.id = p.scene_id AND sp.shown
 WHERE s.work_id = $1 AND s.kind = 'promise'
 GROUP BY s.id, s.scene_id, s.prop_id, s.text
