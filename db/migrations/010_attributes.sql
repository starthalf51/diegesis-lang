-- 010: entity attributes (Plan 2 Task 3). One `attr <entity>.<key> = <value>` statement is one row.
--   kind: int (normalized decimal) | str (string) | id (the declared ID if it is a declared name, otherwise the symbol as is).
--   at_pt NULL = true from the start (top-level/declaration block; the rule looks at origin). account_id = the account holding that statement.
--   The verdict is made by rules/attribute-conflict.sql.
SET LOCAL search_path = public, ag_catalog;

CREATE TABLE attribute (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, entity_id text NOT NULL, key text NOT NULL, value text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('int', 'str', 'id')), at_pt text, account_id text, src_ref text NOT NULL,
  src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);
CREATE INDEX ON attribute (work_id, entity_id, key);
