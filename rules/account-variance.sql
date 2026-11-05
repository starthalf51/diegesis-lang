-- rule: account-variance
-- severity: info
-- layer: lint
-- target: act
-- summary: 같은 사건의 기록(account)끼리 담긴 사실이 다르다
--
-- 대상: 기록이 둘 이상인 사건. 기록 A 에 담긴 명제 = A 안 행위의 내용(prop_id ∪ with, 동사 무관), lint_canon 으로 정규화.
-- 보고: A 의 명제 P 마다 같은 사건의 다른 기록 B 에 P 가 없으면 한 건. B 에 ¬P 가 있으면(lint_opposite) contradicted,
--   아니면 absent. 대상은 A 에서 P 를 처음 담은 행위다. 보고서다(info). 위반이 아니다.
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
       format('%s in %s is %s in %s', lint_short(s.prop_id), lint_short(s.account_id), v.kind, lint_short(b.id)) AS message,
       jsonb_build_object('event', s.event_id, 'account', s.account_id, 'other', b.id, 'prop', s.prop_id,
                          'kind', v.kind) AS data
  FROM said s JOIN acc b ON b.event_id = s.event_id AND b.id <> s.account_id
  CROSS JOIN LATERAL (SELECT CASE WHEN EXISTS (SELECT 1 FROM said t WHERE t.account_id = b.id AND lint_opposite($1, s.c, t.c))
                                  THEN 'contradicted' ELSE 'absent' END AS kind) v
 WHERE NOT EXISTS (SELECT 1 FROM said t WHERE t.account_id = b.id AND t.c = s.c)
