-- 001: IR 테이블(ir.Work 슬라이스와 1:1) + 파생 테이블. 모든 행은 work_id 로 작품에 묶인다.
-- 작품 삭제(DELETE FROM work) 가 하위 행을 전부 지운다(재적재, Review Focus 3).
-- 빈 문자열 참조와 src_id 0 은 적재기가 NULL 로 바꾼다.
SET LOCAL search_path = public, ag_catalog;

-- work.src_id 는 FK 가 없다(src 가 work 를 가리키므로 순환).
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

-- 원문 그대로. 구간은 resolve_roles 가 role_span 에 계산한다.
CREATE TABLE role_holder (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  role_id text NOT NULL, entity_id text NOT NULL, holder_is_role bool NOT NULL,
  from_tref text NOT NULL, until_tref text NOT NULL, from_pt text, until_pt text,
  shared bool NOT NULL, src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- earliest/latest 는 solve_time 이 채운다. NULL = 무한(또는 풀 수 없음).
CREATE TABLE point (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  id text PRIMARY KEY, kind text NOT NULL, src_id int, earliest int8, latest int8,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

-- min ≤ to − from ≤ max. ord 는 IR 슬라이스 순번(진단이 간선을 가리킬 때 쓴다).
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

-- 바깥 경계만(from_pt = 입장 직전, to_pt = 퇴장 직후). 필연 구간은 린터가 계산한다.
CREATE TABLE presence (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, scene_id text NOT NULL, place_id text, from_pt text, to_pt text,
  src_id int, FOREIGN KEY (work_id, src_id) REFERENCES src);

-- resolved_actor_id 는 resolve_roles 가 채운다(role: 화자는 그 시점의 보유자, 모호하면 NULL).
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

-- 명시적 believes/doubts 문장(원문). 파생 믿음은 believes 테이블이다.
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

-- until_pt 는 `withhold … until X` 의 X.start 다.
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

-- ── 파생 (derive 가 채운다) ──

-- value=false 는 ¬P 를 믿는다(거짓말한 화자). from_pt = 그 행위의 시점.
CREATE TABLE believes (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, prop_id text NOT NULL, value bool NOT NULL, from_pt text, cause_act text);

CREATE TABLE uses (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  entity_id text NOT NULL, prop_id text NOT NULL, at_pt text, act_id text NOT NULL);

CREATE TABLE deception (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  act_id text NOT NULL, liar_id text NOT NULL, prop_id text NOT NULL);

-- 보유 구간. 한 역할을 동시에 둘이 가질 수 없다(shared 제외). 위반은 적재 실패다(컴파일러 버그).
CREATE TABLE role_span (
  work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  role_id text NOT NULL, entity_id text NOT NULL, from_pt text, until_pt text,
  span int8range NOT NULL, shared bool NOT NULL, src_id int,
  EXCLUDE USING gist (role_id WITH =, span WITH &&) WHERE (NOT shared));

-- run_id 'derive' = 적재 때 PG 파생이 낸 진단. 린터(Task 6)는 자기 run_id 를 쓴다.
CREATE TABLE diagnostic (
  run_id text NOT NULL, work_id text NOT NULL REFERENCES work(id) ON DELETE CASCADE,
  rule_id text NOT NULL, severity text NOT NULL, target_kind text, target_id text, src_id int,
  message text NOT NULL, data jsonb, waived bool NOT NULL DEFAULT false,
  FOREIGN KEY (work_id, src_id) REFERENCES src);

CREATE INDEX ON time_edge (work_id, from_pt);
CREATE INDEX ON audience (act_id);
CREATE INDEX ON point (work_id);

-- 적재기가 마지막에 부른다. 004_graph.sql 이 실제 파생으로 바꾼다.
CREATE FUNCTION derive(w text) RETURNS void LANGUAGE sql AS $$ SELECT $$;

-- 작품마다 AGE 그래프 하나. ponytail: 'a-b' 와 'a_b' 는 같은 이름이 된다. 작품 ID 규칙이 생기면 검사한다.
CREATE FUNCTION graph_name(w text) RETURNS text LANGUAGE sql IMMUTABLE
  AS $$ SELECT 'g_' || regexp_replace(lower(w), '[^a-z0-9_]', '_', 'g') $$;
