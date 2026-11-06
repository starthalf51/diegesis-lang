-- 010: 엔티티 속성(플랜 2 Task 3). `attr <entity>.<key> = <value>` 한 문장이 한 행이다.
--   kind: int(정규화한 10진수) | str(문자열) | id(선언된 이름이면 그 선언 ID, 아니면 그대로의 기호).
--   at_pt NULL = 처음부터 참(최상위·선언 블록, 규칙은 origin 으로 본다). account_id = 그 문장이 든 기록(account).
--   판정은 rules/attribute-conflict.sql 이 한다.
SET LOCAL search_path = public, ag_catalog;

CREATE TABLE attribute (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, entity_id text NOT NULL, key text NOT NULL, value text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('int', 'str', 'id')), at_pt text, account_id text, src_ref text NOT NULL,
  src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);
CREATE INDEX ON attribute (work_id, entity_id, key);
