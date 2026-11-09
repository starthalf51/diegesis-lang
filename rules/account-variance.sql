-- rule: account-variance
-- severity: info
-- layer: lint
-- target: act
-- summary: accounts of the same event contain different facts
--
-- Target: events with two or more accounts. The propositions in account A = the contents of acts inside A (prop_id ∪ with, any verb), normalized with lint_canon.
-- Report: for each proposition P of A, report it if another account B of the same event lacks P. If B has ¬P (lint_opposite) it is contradicted,
--   otherwise absent. The same (A, P, kind) is merged into one finding listing all the accounts concerned (data.others) — so that with four accounts
--   a proposition present in only one account does not appear three times (Task 10, Gospels). The target is the act that first holds P in A.
--   This is a report (info). It is not a violation.
WITH
acc AS (
  SELECT a.id, a.event_id FROM account a
   WHERE a.work_id = $1 AND (SELECT count(*) FROM account b WHERE b.work_id = $1 AND b.event_id = a.event_id) >= 2),
said AS (
  SELECT DISTINCT ON (ac.id, lint_canon($1, x.prop_id))
         ac.id AS account_id, ac.event_id, x.act_id, x.prop_id, lint_canon($1, x.prop_id) AS c
    FROM acc ac JOIN act a ON a.work_id = $1 AND a.scene_id = ac.id JOIN act_content x ON x.act_id = a.id
   ORDER BY ac.id, lint_canon($1, x.prop_id), a.ord)
SELECT 'act' AS target_kind, s.act_id AS target_id,
       format('%s in %s is %s in %s', lint_short(s.prop_id), lint_short(s.account_id), v.kind,
              string_agg(lint_short(b.id), ', ' ORDER BY b.id)) AS message,
       jsonb_build_object('event', s.event_id, 'account', s.account_id, 'others', jsonb_agg(b.id ORDER BY b.id),
                          'prop', s.prop_id, 'kind', v.kind) AS data
  FROM said s JOIN acc b ON b.event_id = s.event_id AND b.id <> s.account_id
  CROSS JOIN LATERAL (SELECT CASE WHEN EXISTS (SELECT 1 FROM said t WHERE t.account_id = b.id AND lint_opposite($1, s.c, t.c))
                                  THEN 'contradicted' ELSE 'absent' END AS kind) v
 WHERE NOT EXISTS (SELECT 1 FROM said t WHERE t.account_id = b.id AND t.c = s.c)
 GROUP BY s.act_id, s.prop_id, s.account_id, s.event_id, v.kind
