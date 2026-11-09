> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Compiler structure

> Written in Go. A hand-written recursive-descent parser, with no parser generator. The compiler goes only as far as parsing, name resolution, type checking and emitting raw facts. Time solving, knowledge derivation, lint, patterns and metrics are done by PostgreSQL. → [storage](storage.md)

## Pipeline

```
.dieg.md files
  │ split      front matter / ```sl blocks / prose. Original line numbers are preserved (internal/syntax)
  │            (a fence is recognized only at column 0)
  │ desugar    front matter → language-statement AST. The mapping table is in file-format.md
  │            Overlap with the same fact in a block is a dup-decl error
  │ lexer      indentation → INDENT/DEDENT. Every token carries file:line:col
  │ parser     recursive descent. After an error it recovers line by line and collects all errors
  ▼ AST
  │ sema (internal/sema)
  │   resolve  import-cycle detection (import-cycle); name resolution: entity, scene, beat and setup IDs, `@` points in time, cross-module references, aliases
  │   types    argument kinds per verb, declaration kinds (e.g. `tag` only on a fact)
  │   presence computes scene attendance boundaries: exit of someone not there, etc.
  │   emit     emits points in time, time edges, acts, belief declarations and reader directives as IR records
  ▼ IR (JSON, internal/ir — the schema is 1:1 with the PG tables)
  │ load       loads into PG in one transaction (replacing per work) → calls derive
  ▼
PostgreSQL  derive: solve_time · resolve_roles · derive_beliefs · project_graph(AGE)
            └ lint (rules/*.sql) · patterns (patterns/*.sql) · metric_* · render_scene
```

- The compile stages need no DB. `dieg check` finishes in seconds, so the LLM porting loop (generate → error → fix) turns quickly.
- The file extension is `.dieg.md`, and the code-fence tag is `sl`.
- Every build must have exactly one `work` declaration (it defines the time origin). Without it, the error is `no-work`.

## Time model

It is expressed as an **STN** (simple temporal network). The compiler only emits constraints as `time_edge` (`min ≤ t_to − t_from ≤ max`); solving is done by PG's `solve_time` / `point_dist`.

| Source | Constraint |
|---|---|
| `when after X by a..b` | `a ≤ t − X.end ≤ b` |
| `when before X` | `t.end ≤ X.start` |
| `when at 208-11` | absolute interval relative to the origin |
| `lasts a..b` | start→end edge `[a, b]` of a scene, event or account. The only statement that can bound elapsed time in a work without dates |
| order of statements in a scene | `X.start ≤ beat1 ≤ beat2 ≤ X.end` |
| `born` / `dies` | life edge (birth ≤ death) |
| `distance p q a..b` | input to the travel check (the `distance` table) |

- A contradiction (a negative cycle) is a `time-cycle` diagnostic. If it is waived, that edge is dropped from solving and only the diagnostic remains. Double time (Othello) has `time-cycle` as its expected diagnostic.
- In a work with no absolute times, `latest` is ∞. So judgments that need relative distance use `point_dist`.

## Knowledge derivation (PG)

| Statement | Recipients | Records left |
|---|---|---|
| `A tells B: P` | B. Everyone else present also overhears (only B for `whispers`) | B and those present believe P; the cause is this speech act |
| `A says: P` | everyone present | same |
| `A lies B: P` | same | the recipients believe P, plus a `deception` row |
| `A aside` / `thinks` | the reader only | disclosed to the reader |
| `A sees …` | A | belief from perception |
| `A reads art` | A | all propositions carried by the document |
| `A writes art` | — | `uses(A, P)`. The writer must have believed P before writing. If the writer definitely knows ¬P, a `deception` row |
| `A does X because P` | — | `uses(A, P)` |
| `X doubts P` (at time t) | — | withdraws the belief rows for P that were heard or read, from t (`until_pt`) |
| a speaker's truthful speech | — | `uses(speaker, P)` |

Judgment is the linter's job. Derivation only produces believes and uses.

## Language features

| Statement | Meaning | Judged by |
|---|---|---|
| `attr <entity>.<key> = <value> [src …]` | Attribute. The value is a number, string or ID. Tied to a point in time, it changes over time | `attribute-conflict` |
| `lasts a..b` | Length constraint | `time-cycle` |
| `fact … tag <name>` | Fact classification. `identity` = an identity fact | chosen by the `recognition` pattern |
| `pattern <id> [at <scene>]` | Author declaration: this narrative pattern is present | `pattern-missing` |
| `reveal P [at X]` / `withhold P until X` | Control of disclosure to the reader | reader model |
| `enacts` | Performance or re-enactment (a play within the play): not an assertion | no belief derivation |

The full grammar is [`spec/grammar.ebnf`](../spec/grammar.ebnf). `attr` and `pattern` are not reserved words (if the next word is a verb, believes or doubts, it is an act or belief of the same name).

An example that uses an attribute together with time constraints:

<!-- check -->
```sl
module demo.attr

work demo
  unit day
character hamlet "Hamlet" major
place castle

scene 1 at castle
  when after start
  lasts 1h..3h
  enter hamlet
  attr hamlet.age = 20 src 1.2
  exit all
scene 2 at castle
  when after 1 by 300d..400d
  enter hamlet
  attr hamlet.age = 21 src 2.1
  exit all
```

## Reading errors

The diagnostic format is `file:line:col: severity rule: message`. For example, if you misspell `hamlet` as `hamlt` in the example above, `dieg check` reports `error undefined-ref: undefined actor "hamlt"`. If there is any error, the exit code is 1.

## IR

- One normalized JSON. Arrays per table, 1:1 with the [storage](storage.md) schema. Every row carries `src: {file, line, col}`.
- Stable IDs: `work/module/local-id`. They do not change when files are rearranged.
- Tests: compiler unit tests run without PG, and PG judgment tests need a Docker PG (`internal/pg`).

## CLI (`dieg`)

```
dieg db migrate                         apply migrations
dieg check <dir> [--json]               parsing and semantic analysis only. No DB needed
dieg build <dir> -o ir.json
dieg load  <ir.json|dir>                load into PG (including derive and the AGE projection)
dieg lint  <work|dir|ir.json> [--json]  run rules/*.sql
dieg patterns <work|dir> [--json]       run patterns/*.sql, refresh pattern_match
dieg stats <work> [--metric name] [--json]   query metric_*
dieg test <corpus-dir> [--seed N] [--mutants N]   acceptance verdict: expected diagnostics, queries, patterns + mutation test
dieg render <work|dir> <scene> --backend <id> [--verify] [-o file]   → rendering.md
dieg render-check <corpus...> [--force | --force-extract] [--only <work>/<scene>.<backend>]
dieg write / continue / revise / stamp  idea → code → novel pipeline (LLM calls)
```

- With `--json`, diagnostics are emitted as an array with the same content.
- In the design but not implemented: `explain`, `fmt`, `show`, `verify-prose`, `import/export story-skills`, `lint --rules` (the rules are bundled in `rules/` via `embed.go`).

## What it does not do

- Incremental compilation — a work is compiled whole.
- LSP — the diagnostic format is `file:line:col`, so a thin wrapper can be added later.
