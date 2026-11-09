-- 001: IR tables (1:1 with the ir.Work slices) + derived tables. Every row is tied to a work by work_id.
-- Deleting a work (DELETE FROM work) removes all child rows (reload, Review Focus 3).
-- The loader turns empty-string references and src_id 0 into NULL.
SET LOCAL search_path = public, ag_catalog;

-- work.src_id has no FK (src points at work, so it would be circular).
CREATE TABLE work (
  id text PRIMARY KEY, title text NOT NULL, unit text NOT NULL, start text NOT NULL, src_id int);

CREATE TABLE module (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, name text NOT NULL, paths text[] NOT NULL, imports text[] NOT NULL);

CREATE TABLE src (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id int NOT NULL, module_id text, file text NOT NULL, line int NOT NULL, col int NOT NULL,
  PRIMARY KEY (work_id, id));

CREATE TABLE entity (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, module_id text, kind text NOT NULL, tier text NOT NULL, name text NOT NULL,
  knows text, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE group_member (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  group_id text NOT NULL, member_id text NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE name (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  owner_id text NOT NULL, lang text NOT NULL, text text NOT NULL, ord int NOT NULL,
  src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE role (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, names text[] NOT NULL, shared bool NOT NULL, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- Verbatim source text. resolve_roles computes the interval into role_span.
CREATE TABLE role_holder (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  role_id text NOT NULL, entity_id text NOT NULL, holder_is_role bool NOT NULL,
  from_tref text NOT NULL, until_tref text NOT NULL, from_pt text, until_pt text,
  shared bool NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- earliest/latest are filled by solve_time. NULL = unbounded (or unsolvable).
CREATE TABLE point (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, kind text NOT NULL, src_id int, earliest int8, latest int8,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- min ≤ to − from ≤ max. ord is the IR slice index (used when a diagnostic points at an edge).
CREATE TABLE time_edge (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  ord int NOT NULL, from_pt text NOT NULL, to_pt text NOT NULL, min int8, max int8,
  waived bool NOT NULL, waiver_rule text NOT NULL, waiver_reason text NOT NULL, src_id int,
  PRIMARY KEY (work_id, ord), FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE scene (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, module_id text, kind text NOT NULL, title text NOT NULL, place_id text,
  thread_id text, start_pt text, end_pt text, discourse_ord int NOT NULL, discourse_after text,
  shown bool NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- Outer bounds only (from_pt = just before entry, to_pt = just after exit). The linter computes the certain interval.
CREATE TABLE presence (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, scene_id text NOT NULL, place_id text, from_pt text, to_pt text,
  src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- resolved_actor_id is filled by resolve_roles (role: the holder of the role at that point; NULL if ambiguous).
CREATE TABLE act (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, scene_id text, beat_id text, ord int NOT NULL, pt text NOT NULL,
  actor_id text NOT NULL, actor_is_role bool NOT NULL, verb text NOT NULL, action text NOT NULL,
  objects text[] NOT NULL, "to" text[] NOT NULL, "with" text[] NOT NULL, prop_id text,
  quote text NOT NULL, because_id text, src_ref text NOT NULL, src_id int,
  resolved_actor_id text, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE audience (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  act_id text NOT NULL, entity_id text NOT NULL);

-- Explicit believes/doubts statements (source). Derived beliefs live in the believes table.
CREATE TABLE belief (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, verb text NOT NULL, prop_id text, scene_id text, at_pt text,
  because_pt text, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE fact (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, text text NOT NULL, truth text NOT NULL, def jsonb, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE artifact (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, module_id text, name text NOT NULL, contains text[] NOT NULL,
  text text[] NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE setup (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, kind text NOT NULL, prop_id text, text text NOT NULL, scene_id text,
  at_pt text, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE payoff (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  setup_id text NOT NULL, scene_id text, at_pt text, module_id text, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE account (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, event_id text NOT NULL, source_id text, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE parent (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  parent_id text NOT NULL, child_id text NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE distance (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  place_a text NOT NULL, place_b text NOT NULL, min int8 NOT NULL, max int8 NOT NULL, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE reveal (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  prop_id text, scene_id text, at_pt text, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- until_pt is the X.start of `withhold … until X`.
CREATE TABLE withhold (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  prop_id text, scene_id text, until_pt text, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE directive (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  scene_id text, key text NOT NULL, "values" text[] NOT NULL, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE waiver (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  rule text NOT NULL, target_kind text NOT NULL, target_id text, reason text NOT NULL, src_id int,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- ── Derived (filled by derive) ──

-- value=false means believing ¬P (the lying speaker). from_pt = the point of that act.
-- declared = a row from a `believes` statement the author wrote (belief table). src_id is the position of that statement or of the causing act.
CREATE TABLE believes (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, prop_id text NOT NULL, value bool NOT NULL, from_pt text, cause_act text,
  declared bool NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE TABLE uses (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, prop_id text NOT NULL, at_pt text, act_id text NOT NULL);

CREATE TABLE deception (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  act_id text NOT NULL, liar_id text NOT NULL, prop_id text NOT NULL);

-- Holding interval. Two actors cannot hold one role at once (except shared). A violation is a load failure (a compiler bug).
CREATE TABLE role_span (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  role_id text NOT NULL, entity_id text NOT NULL, from_pt text, until_pt text,
  span int8range NOT NULL, shared bool NOT NULL, src_id int,
  EXCLUDE USING gist (role_id WITH =, span WITH &&) WHERE (NOT shared));

-- run_id 'derive' = diagnostics the PG derivation emits at load time. The linter (Task 6) uses its own run_id.
CREATE TABLE diagnostic (
  run_id text NOT NULL, work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  rule_id text NOT NULL, severity text NOT NULL, target_kind text, target_id text, src_id int,
  message text NOT NULL, data jsonb, waived bool NOT NULL DEFAULT false,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE INDEX ON time_edge (work_id, from_pt);
CREATE INDEX ON audience (act_id);
CREATE INDEX ON point (work_id);

-- The loader calls this last. 004_graph.sql turns it into the real derivation.
CREATE FUNCTION derive(w text) RETURNS void LANGUAGE sql AS $$ SELECT $$;

-- One AGE graph per work. ponytail: 'a-b' and 'a_b' map to the same name. Check once a work-ID rule exists.
CREATE FUNCTION graph_name(w text) RETURNS text LANGUAGE sql IMMUTABLE
  AS $$ SELECT 'g_' || regexp_replace(lower(w), '[^a-z0-9_]', '_', 'g') $$;
