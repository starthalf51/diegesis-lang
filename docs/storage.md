> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Storage — PG schema and AGE graph

> The source of truth is relational tables. The AGE graph is a **projection** rebuilt from the tables after loading. All judgment and derivation runs inside PG. Code: `db/migrations/` → [compiler](compiler.md)

## Image

- `deploy/postgres/Dockerfile`: builds pgvector v0.8.7 from source on top of `apache/age:release_PG18_1.8.0`. The versions are pinned. AGE 1.8.0 is PG18 only.
- `000_extensions` enables `age`, `vector` and `btree_gist`. For every session it preloads age and puts `ag_catalog` on the search path.
- No table uses pgvector at present.
- The DSN for local development is given by the environment variable `SL_PG_DSN`.

## Migrations

Only the number increases. A file already used is not edited (013 layering on top of 011 and 012 without touching them is an example). One exception: 003 had only comments fixed on 2026-11-09 (no behavior change). `dieg db migrate` applies them.

| No. | Content |
|---|---|
| 000 | extensions, session settings |
| 001 | IR tables (1:1 with the slices of `ir.Work`) and derived tables. Every row is tied to its work by `work_id`. Deleting a work deletes the child rows |
| 002 | time: `solve_time`, `point_dist`, `precedes`, negative-cycle diagnostics |
| 003 | role holding spans (`resolve_roles`), knowledge derivation (`derive_beliefs`), the `act_content` view |
| 004 | AGE projection (`project_graph`) and the `derive` entry point |
| 005 | shared lint pieces (`lint_canon` · `lint_known` · `lint_opposite` · `lint_short`, etc.) |
| 006 | doubt (doubts): `until_pt` on belief rows |
| 007 | deception: lies told in writing, and doubt of documents read |
| 008 | alias and negation interpretation fixes, waiver targets for derive diagnostics, `until` on graph BELIEVES |
| 009 | `render_scene`: the render-request JSON |
| 010 | the `attribute` table (entity attributes) |
| 011 | reader model |
| 012 | patterns: `pattern_decl` · `pattern_match` |
| 013 | reader-model fix (`reader_lies`), the `pattern_result` view |
| 014 | metrics `metric_*` · `graph_*` |
| 015 | read-only role `sl_ro` |
| 016 | `sl_ro` privilege cleanup: all dashboard functions are INVOKER, and execute rights go only to a list of names |

## Tables

Time is `int8` seconds counted from the work's origin. Every row references `src` (file, line, column).

| Group | Tables |
|---|---|
| Structure | `work` · `module` · `src` · `entity` · `group_member` · `name` · `role` · `role_holder` · `role_span` · `parent` · `distance` |
| Time | `point` (`earliest`/`latest` are solver results) · `time_edge` · `point_dist` · `scene` · `presence` |
| Acts / knowledge | `act` · `audience` · `belief` (declared) · `believes` (derived, including `until_pt`) · `uses` · `deception` · `fact` (including `tag`) · `artifact` · `account` |
| Reader | `reveal` · `withhold`. Reader beliefs are produced by `reader_*` functions, not by a table |
| Foreshadowing / patterns | `setup` · `payoff` · `pattern_decl` · `pattern_match` |
| Attributes | `attribute` (kind: int / str / id, `at_pt` NULL = true from the start) |
| Diagnostics | `diagnostic` · `waiver` · `directive` |

What changed from the design draft:

- There is no pairwise distance table `transition`. `point_dist` holds the shortest distance of every pair of points.
- `presence` holds only the outer boundary (`from_pt`/`to_pt`). The linter computes the necessary span. An exclusion constraint against being in two places at the same time is not in the schema; a rule judges it.
- The only exclusion constraint is `role_holder` (two holders of one role at the same time; `shared` roles are the exception).
- No scene-embedding table was made. Render directives are handled by `directive`.

## Time

- One `time_edge` row is `min ≤ t_to − t_from ≤ max`. A NULL bound means there is no edge.
- `solve_time` fills `earliest/latest` of `point`. The shortest distance from the origin is `latest`, and the reverse direction gives `earliest`.
- `point_dist` is the shortest distance of every pair of points (all-pairs STN). Judgments that need relative distance (travel time, the order of acquiring and using information, elapsed time of attributes) read it. The cost is O(V²).
- `precedes(a, b)` returns settled / possible / impossible. If not settled, the lint level drops to info.
- Negative cycles and order cycles come out as `time-cycle` diagnostics. The `when`/`born`/`dies` statements that formed the cycle are the waiver targets.
- With no absolute time, it is `earliest=0, latest=NULL`. Metrics do not count such a point as story time.
- The length of a scene or event is put in by `lasts a..b` as a start→end edge.

## Knowledge and reader

- `believes` is produced by `derive_beliefs`. One row each comes from speech, perception, reading and declarations, with the causing event (`cause_act`) attached.
- **doubts**: `X doubts P` withdraws belief rows obtained *by hearing* (assertion-speech audience rows) before that point. The earliest doubt time goes into the row's `until_pt`. Perception, reading and explicit believes are not withdrawn. A background doubts without a time withdraws nothing.
- **deceit**: doubt also withdraws beliefs obtained from a document that was read. If the writer definitely knows ¬P at the time of writing, that document's P is a lie and a `deception` row is created. A forged letter takes this path.
- **Reader model**: the reader receives the assertions, perceptions, asides and thoughts of *shown* scenes at that scene's `discourse_ord`. The reader believes as told (lies too). Propositions are normalized to (underlying fact, value), and when the opposite value is disclosed later, it is updated at that position. `withhold P until X` blocks disclosure until the position where X appears.
- **The reader and lies**: a deception the reader already knows to be false (a lie, reading a forged document) is not believed by the reader. If the opposite value was disclosed earlier at a position by a non-deceptive disclosure, it is excluded.
- Character knowledge follows story time, and reader knowledge follows telling order (`discourse_ord`). The x axis of the metrics is the latter.

## Patterns

- Pattern definitions are `patterns/*.sql`. One file is one pattern, and its body is a SELECT where `$1` = the work. The runner (`internal/pg/patterns.go`) only moves the results into `pattern_match`. Judgment is done by the pattern SQL and the reader model.
- The author's declaration `pattern <id> [at <scene>]` is a `pattern_decl`. If declared and there is no match, `pattern-missing` (warn). If there is a match but no declaration, it is only reported.
- The `pattern_result` view shows the matches of the most recent run joined with the declarations. The metrics and `dieg patterns` read it.
- List and meaning → [checks](checks.md#pattern-library-patternssql-10-patterns)

## Metrics (014)

Every function takes `(work text)` and returns 0 rows for an empty work. The x axis is the `discourse_ord` of shown scenes.

| Function | Content |
|---|---|
| `metric_presence` | attendance and act count per scene |
| `metric_asymmetry` | the number of true facts that characters in the scene do not know (or believe the opposite of) |
| `metric_open_setups` | number of open setups |
| `metric_active_deceptions` | number of deceptions in progress |
| `metric_tension` | open setups + active deceptions + dramatic-irony and mystery matches covering that position, each term normalized by its maximum and combined as a weighted sum. The weights are in the `metric_weights` table |
| `metric_anachrony` | telling order vs. story time. Points whose time is not constrained are excluded |
| `metric_pace` | the scene's story-time span (seconds) vs. act count. If the end is a point with no time, the span is NULL |
| `graph_nodes`, `graph_edges` | character graph. Edge = number of scenes attended together + number of TOLD |

- The pattern term of `metric_tension` is non-zero only if `dieg patterns` has been run first.
- `metric_anachrony` returns 0 rows for a work with no absolute times. That is not an error.

## Read-only role sl_ro (015, 016)

- For dashboards (Grafana). The migration only creates it as `NOLOGIN`, and the `ro-init` service in `deploy/compose.yaml` sets the password with `ALTER ROLE` (no password is kept in the repository).
- It is granted table `SELECT` and function `EXECUTE` only for a list of names (`metric_*` · `graph_*` · `lint_short` and the read-only helpers those functions call). Every function is `SECURITY INVOKER`, and helpers that build dynamic SQL have no execute right.
- Limit: grants cover the current tables. A new table gets only SELECT, following default privileges.

## AGE graph

The last step of `derive` (`project_graph`) reads the tables and rebuilds one graph per work (`g_<work>`).

| Vertices | Edges |
|---|---|
| `Entity` · `Point` · `Fact` · `Setup` | `BEFORE` (point order) · `TOLD` (speaker→recipient) · `BELIEVES` (`from`, `until`) · `PRESENT_AT` · `CAUSES` · `FORESHADOWS` · `PARENT_OF` / `SPOUSE_OF` · `HOLDS` |

- The graph is used where the path is the answer (delivery paths, order reachability, genealogy, causal chains). Every other check is SQL.
- The graph name in cypher queries is a constant (`g_chibi`, etc.). There is no wrapper that takes a work id yet.

## Cost

Everything is local Docker Compose (PG + Grafana). The only places that cost money are the LLM calls of the renderer and the writing pipeline.
