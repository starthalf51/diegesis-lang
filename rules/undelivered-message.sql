-- rule: undelivered-message
-- severity: warn
-- layer: lint
-- target: act
-- summary: the message of `sends m to t with P` was not delivered to t
--
-- Target: one pair each of a sends act's recipient t (the to list) and content proposition P (prop_id ∪ with).
--   (The audience of an offstage sends is only the messenger. t knows only through delivery.)
-- Delivery: another act in which the messenger (anyone in objects) tells P by an assertive utterance (excluding sends, including lies — it was delivered even if false) and
--   t is in that audience. Aliases are aligned with lint_canon.
-- Verdict: fine if there is a delivery with precedes(sends, delivery) = yes, info if only maybe, warn if none.
WITH
msg AS (
  SELECT s.id AS act_id, s.pt, s.objects AS messengers, t.id AS recipient, x.prop_id, lint_canon($1, x.prop_id) AS c
    FROM act s CROSS JOIN LATERAL unnest(s."to") t(id) JOIN act_content x ON x.act_id = s.id
   WHERE s.work_id = $1 AND s.verb = 'sends'),
told AS (
  SELECT r.act_id, r.pt, r.actor_id, lint_canon($1, r.prop_id) AS c, au.entity_id AS hearer
    FROM act_content r JOIN audience au ON au.act_id = r.act_id
   WHERE r.work_id = $1 AND assertive(r.verb) AND r.verb <> 'sends'),
graded AS (
  SELECT m.act_id, m.recipient, m.prop_id,
         max(CASE WHEN d.act_id IS NULL THEN 0
                  ELSE CASE precedes($1, m.pt, d.pt) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END) AS g
    FROM msg m
    LEFT JOIN told d ON d.actor_id = ANY (m.messengers) AND d.c = m.c AND d.hearer = m.recipient AND d.act_id <> m.act_id
   GROUP BY 1, 2, 3)
SELECT 'act' AS target_kind, act_id AS target_id,
       CASE g WHEN 1 THEN 'info' ELSE 'warn' END AS severity,
       format(CASE g WHEN 1 THEN 'message %s to %s may be delivered only before it was sent (order not determined)'
                     ELSE 'message %s to %s is never delivered' END,
              lint_short(prop_id), lint_short(recipient)) AS message,
       jsonb_build_object('recipient', recipient, 'prop', prop_id, 'order', CASE g WHEN 1 THEN 'maybe' ELSE 'none' END) AS data
  FROM graded WHERE g < 2
