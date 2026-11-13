> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Checks — layer placement, linter, patterns

> Principle: an **error** is raised only when the IR would lose meaning. Every contradiction inside the story is reported as a **difference (warn)** or a **report (info)**. Judgment is SQL inside PG. → [storage](storage.md) The lint errors are `reader-unknown-term`, `pov-offstage` and `pov-mind-leak`: a word the reader cannot understand yet, or a point of view that cannot exist, is a defect of the telling, not a story difference, so it blocks like a compile error.

## Layer placement

| Check | Layer | Level |
|---|---|---|
| Syntax (`syntax`) | parser | error |
| Undefined reference (`undefined-ref`), verb argument kind (`arg-kind`), unknown role (`role-unknown`) | compiler resolve · types | error |
| Import cycle (`import-cycle`) | compiler resolve | error |
| Same information in front matter and block (`dup-decl`) → [file-format](file-format.md) | compiler desugar | error |
| Wrong target or reason of a waiver (`waiver-target`, `waiver-reason`) | compiler | error |
| Exit of someone not there (`offstage-exit`) | compiler presence | warn |
| Overlapping holding spans of one role `role-overlap`, ambiguous speaker `role-ambiguous` | PG `resolve_roles` (derive diagnostic) | warn |
| Time-constraint cycle `time-cycle` | PG `solve_time` (derive diagnostic) | warn |
| Two holders of one role at the same time | DB exclusion constraint (`role_holder`) | error |
| All other story contradictions | lint rules `rules/*.sql` | warn / info (`reader-unknown-term`, `pov-offstage`, `pov-mind-leak`: error) |
| Narrative patterns | `patterns/*.sql` | report (only a missing declaration is warn) |

`stale-prose` and `rendered-edited` (rendered prose is stale or was hand-edited) exist only in the design and are not implemented.

## Lint rules (`rules/*.sql`, 21)

19 are `layer: lint` (run by the linter), and `time-cycle` alone is `layer: derive` (it only shows a diagnostic that derive already produced; the linter does not run it).

| Rule | Level | Target | Meaning |
|---|---|---|---|
| `info-flow` | warn | act | A character says a proposition they have never learned before |
| `letter-unknown-info` | warn | act | A document carries a proposition its writer did not believe before writing |
| `because-unbelieved` | warn | act | `because P`, but the actor does not believe P at that point |
| `belief-contradiction` | warn | act | Truthfully asserts a proposition opposite to a fact the character perceived directly |
| `undelivered-message` | warn | act | The message of `sends m to t with P` was never delivered to t |
| `dead-actor` | warn | act | A dead character speaks or acts |
| `stays-gone` | warn | act | A character declared to have left the story acts or appears again |
| `object-location` | warn | act | An object is handled in a scene at a place other than where it is declared to be |
| `retell-known` | warn | act | The same speaker tells the listener, in the same words, a fact the listener already believes |
| `payoff-without-setup` | warn | payoff | The setup is missing or does not come before the payoff |
| `unpaid-promise` | warn | setup | A promise setup was never paid off |
| `attribute-conflict` | warn | attribute | The same attribute of the same entity is asserted with incompatible values |
| `fact-conflict` | warn | fact | Two true facts have the same predicate and first argument but different remaining arguments |
| `relative-time-fact` | warn | fact | Fact text has a relative time baked in (e.g. "열이틀 뒤", "twelve days later") |
| `reveal-false` | warn | reveal | A fact declared false is disclosed to the reader |
| `pattern-missing` | warn | pattern | A pattern the author declared was not detected |
| `account-variance` | info | act | The facts contained differ between accounts of the same event |
| `inferred-deception` | info | act | A document judged to have been written knowing it was false — check whether it is the author's mistake |
| `reader-unknown-term` | error | act | A declared `term` surface reaches the reader before a scene at or before it in the telling has `render explain` for it |
| `term-re-explained` | warn | scene | A scene's `render explain` names a term an earlier scene in the telling already explains (use `recap`) |
| `scene-no-turn` | warn | scene | In a work that uses `design` blocks, a shown scene has no `turn` (silent when the work has no design block) |
| `pov-offstage` | error | scene | `render pov X` names a character who never enters the scene (no `enter`; `mention` does not count, a later `exit` is not considered) |
| `pov-mind-leak` | error | act | In a `render narration first-person` scene, a `thinks` act belongs to someone other than the pov character |
| `time-cycle` | warn | when | Time constraints form a cycle (derive) |


- `info-flow`, `letter-unknown-info` and `because-unbelieved` are the same check split by the kind of target act. One defect yields one diagnostic. `lies` and false documents are excluded from this check (deception is not an information-flow violation).
- Knowing is judged by `lint_known`. A belief withdrawn by doubt (`until_pt`) counts as not known. If the order is not settled (`precedes` = possible), the level drops to info.
- `attribute-conflict` looks at static conflicts (e.g. wound location shoulder vs. leg) and at elapsed-time contradictions in numeric attributes such as age. For age, it uses `point_dist` to weigh the upper and lower bounds on the gap between points separately, so it can decide with only one bound.
- `status` and `at` are exempt from attribute-conflict (stays-gone and object-location read them per point). A status takes effect from its position in the source, and leaving is not an event within a scene.

## Rule file format

Rules live in `rules/` and are embedded in the binary (`rules/embed.go`). One file is one rule.

```sql
-- rule: letter-unknown-info
-- severity: warn
-- layer: lint | derive
-- target: act            (where the waiver directive attaches)
-- summary: …
SELECT … AS target, …
```

- For each result row, the runner attaches the `src` of `target`, puts it in `diagnostic`, and checks it against `waiver` to mark whether it is waived.
- `dieg lint <work> --rules <dir>` runs the `*.sql` rules in a project directory (same header format, same file-name = rule-ID requirement) in addition to the built-in ones. A rule ID that collides with a built-in rule or repeats inside the directory is an error; built-in rules cannot be overridden. A directory with no `.sql` file is also an error. Subfolders under `--rules` are read recursively; per-genre selection is not implemented. Project rules must be `layer: lint` (derive-layer rows come only from load-time logic, so any other layer is rejected).

## Pattern library (`patterns/*.sql`, 10 patterns)

A pattern is the **detection of a structure**, not a violation. One file is one pattern, and its body is a SELECT where `$1` = the work. `dieg patterns` loads the results into `pattern_match` and you view them through the `pattern_result` view → [storage](storage.md#patterns)

| Pattern | Layer | Meaning |
|---|---|---|
| `chekhov-gun` | foreshadow | A promise setup is paid off |
| `prophecy-fulfilled` | foreshadow | A proposition asserted by a prophet was not true then and becomes true later |
| `dramatic-irony` | information | The reader knows P is true, and a character in the same scene does not know P or believes the opposite (per scene, only characters in the scene) |
| `mystery` | information | A true P is withheld from the reader, and in the meantime a scene asks about or doubts P |
| `red-herring` | information | The reader believes a false fact for a while and later receives the opposite |
| `reversal-reveal` | information | A fact the reader believed is overturned, and it had been the basis of a setup or an action |
| `deception-chain` | deception | Deception → the recipient believes → an action that uses that belief as its `because` |
| `counter-deception` | deception | The recipient of a deception doubts it or already believes the opposite — not fooled |
| `recognition` | identity | The set of characters who believe a `tag identity` fact grows in telling order, and grows through `sees` at least once |
| `bookend` | structure | The opening and closing scenes share characters, places and facts more than the opening shares with the middle (`metric_structure` bookend >= 0.2) |

- If the author declares `pattern <id> [at <scene>]`, a match is expected. If there is none, `pattern-missing`.
- The expected number of patterns per corpus is in `expect.yaml`, and `dieg test` checks it.
- Known limits: some patterns are loose in excluding state predicates or in order checks (reversal-reveal; counter-deception has an unbounded doubt window; `pattern_covers` is loose over long spans).

## Waivers (nolint)

A waiver directive attaches to the next statement. A reason is required.

The waiver reason in the `sl` block below is a Korean story-data string ("intended contradiction — Ben saw it wrong"); it is left as written.

<!-- check -->
```sl
module demo.waiver

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true

scene 1 at hall
  when after start
  enter anna, ben
  ben sees door_locked
  #! allow belief-contradiction: "의도된 모순 — 벤이 잘못 본 것이다"
  ben says anna: not door_locked "It is open." src 1.1
  exit all
```

- An empty reason is a compile error (`waiver-reason`).
- A waived diagnostic is not deleted but kept with `waived=true`. So "the list of intended inconsistencies in this work" becomes an artifact in its own right. `dieg test` treats an undeclared waiver as a failure.
- A work-wide waiver is `allow rule-id: "…"` in the `work` block.
- An **error** cannot be waived (today `reader-unknown-term`, `pov-offstage`, `pov-mind-leak`). A matching `#! allow` leaves it unwaived: `dieg lint` prints it with `(error: cannot be waived)` and exits 1, and the `write`/`convert` loops still block on it (also with `--allow-warn`). warn and info waivers work as above.
- Limit: a `when` waiver targets the whole scene.

## Evaluation — `dieg test`

For a corpus to PASS: 0 compile errors, 0 unexpected warnings (false positives), 0 undeclared waivers, expected diagnostics, expected patterns and declared queries all match, and the mutation detection rate is at or above the threshold. The expected values are in the corpus's `expect.yaml` and are not adjusted to fit tool output.

### Mutation test

Inject one error at a time into the original port, and see whether a rule or declared query that should catch it reacts. Nine operators:

| Operator | Expected rule |
|---|---|
| `drop-enter` | info-flow family |
| `drop-tells` | undelivered-message, info-flow family |
| `move-before` | info-flow family |
| `flip-perceived` | belief-contradiction, info-flow family |
| `drop-setup` / `drop-payoff` | payoff-without-setup / unpaid-promise |
| `kill-early` | dead-actor |
| `shift-duration` / `swap-order` | time-cycle, dead-actor, info-flow family (+ declared-query mismatch) |

- **Pass criteria**: overall mutation detection rate ≥ 0.8, and the **detection rate excluding move-before ≥ 0.7**. These two are the gates.
- Detection counts only new warns near the mutation site (the mutated act, statement or scene, or an act that uses its proposition; for a time mutation, the other end of an edge whose endpoint is the mutated point). The same rule's warnings elsewhere in the work are not counted as incidental detections.
- Query detection: if a mutation changes the result of an `expect.yaml` query, it counts as detected regardless of site. Queries that read diagnostics are excluded.
- Limit: part of the time mutations are caught by queries, not rules. `dieg test` does not report a lint-only detection rate separately.
- Cases where a mutation does not change the story (equivalent mutants) are marked by a person and removed from the tally.

## Ground-truth set (known consistency issues)

Ported from the source texts. We also confirmed the negative control: deleting the sentence that gives the clue to the defect makes the diagnostic disappear.

| Work | Issue | Rule that catches it |
|---|---|---|
| Hamlet | the student (age 20) in 1.2 vs. age 30 in 5.1 | `attribute-conflict` |
| Othello | double time | `time-cycle` |
| Holmes | Watson's wound location shoulder vs. leg | `attribute-conflict` |
| Holmes | Watson's first name John vs. James | `attribute-conflict` |
| Macbeth | the Thane of Cawdor's treason | `belief-contradiction` (1 declared waiver) |
