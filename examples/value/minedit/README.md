# Minimal-edit render (Arm M) — what if `dieg revise` fixes the old prose instead of re-rendering?

In the retcon spike (`examples/value/retcon`, commit daa3d62), `dieg revise` (B) left few residual contradictions (A 4 vs B 0) but re-rendered the affected scenes from scratch, so it made many calls (B 84 vs A 15) and changed even unrelated sentences (collateral change 13,569 characters vs 1,250). This spike changes only the render stage.

# Hypothesis

Instead of rendering the affected scenes from scratch, give haiku **the old prose + the new scene spec + the exact old→new spec difference** and tell it "fix only the sentences that touch the difference and do not change a single other character". Then B's accuracy (0 residual contradictions) is kept while collateral changes and the call count both drop.

The pre-registered criterion "minimal edit has an effect" = **all** of the following four are met:
1. Collateral change ≤ 30% of a full re-render
2. Render+verify calls ≤ 50% of a full re-render
3. Confirmed residual + new contradictions ≤ the full re-render's sum
4. Card pass rate is not lower

If only some are met, "partial effect"; otherwise "no effect".

# Method

- **Reused:** the B results of retcon spike R1–R3 (the revised story `rN/b/story`, old prose = the run3 B novel `examples/serial/run3/b/novel`, the full re-render prose `rN/b/novel`). The coding rounds were not re-run. The code-side cost (2 opus calls `rev1-code-r1`/`fix-r2`) is the same for M and B; the comparison gives both the values with it excluded and included.
- **Affected scenes = scenes whose render request hash changed** (exactly the set `dieg revise` re-renders). We loaded the revised story into PG (`dieg load`), saved the scene request JSON in `ir/`, and saved the old story (the copy B started from) the same way. We confirmed that the render hashes computed from the saved IR match all the `.hash` values saved in the retcon spike on both sides (old 31/31, new R1 32/32, R2 32/32, R3 33/33). The affected scenes are 10 in R1, 17 in R2, and 7 in R3.
- **Difference (diff):** the scene request is unfolded into "item: content" lines (title, place, previous plot, characters, acts, viewpoint, what the viewpoint character believes, what the reader knows, what must not be said, directives), and the lines only on the old side and only on the new side are extracted as a multiset difference (`sceneDiff`, deterministic). A changed line comes out as a delete+add pair. Per-scene differences: `r*/diff/`.
- **Prompts (spike only; production prompts were not touched):** a Korean system prompt (the minimal-edit rules + the target-register phrasing of backend `ko-haera` taken as is from `Backend.Prompt`) + a user prompt (old prose, new scene spec = the user part of the production `Backend.Prompt`, change list). Full prompts: `r*/prompts/`.
- **Calls:** per scene, haiku (effort low, the same as the production render default) once → card verification with the production `render.ScoreCard` (defaults, tier2 absent-only, model sonnet low) → if it does not pass, attach the card feedback and fix once more, then stop (this was never needed in this run). Labels are `render-sK-aN`/`extract-sK-aN`, the same as dieg, so the same cost formula applies.
- **Judgment:** one sonnet call per reset, with the same prompt (`rcJudgePrompt`) and method as the retcon spike. The input is M's fixed scenes + untouched scenes + the scenes B newly created (see Limitations below). I classified the items as confirmed/borderline/rejected by checking them against the source text (`verification.md`).
- **Measurement:** cost uses the same formula as the retcon spike (weighted input = input + 0.1×cache read + 1.25×cache write; price weight = Σ model factor × (weighted input + 5×output), opus 1, sonnet 0.6, haiku 0.2). Collateral change is **the retcon spike's code as is** (`collateral()`, `editDist`, the pre-set keywords, characters changed outside the affected unit), and we recomputed B's values with the same function and confirmed that the spike's values (2,796 / 6,814 / 3,959) reproduce.
- **Procedure:** built under the `noclaude` shim, ran unit tests and a fake-completer test (`-me-dry`, with full prompts checked), then removed the shim and ran as one process (`serial minedit`). The budget guard applied right before each call and every call is in `calls.jsonl`. The flags are `-me-*`, so they do not collide with the existing spike (`-rc-*`, subcommand `retcon`).

# Important prior constraint: a call cap of 60 cannot run all 3 resets

The 34 affected scenes (10+17+7) need one edit + 1–2 card calls per scene (B has the same structure), so at least 68 calls + 3 judgments. The cap is 60. So we **ran R1 and R3 to the end (judgment included), and with the remaining budget tested R2 only on its first 6 scenes (pilot, no judgment).** **R2 as a whole was not measured.** Totals and the verdict are based on R1+R3. R3's new scenes (s32, s33) and s32 in R1 and R2 have no old prose, so they are not edit targets. In both arms the render cost of these scenes was excluded.

# Table (per reset: M / full re-render B / A)

The "B" column is only B's render+verify calls for **the same affected scenes** (new-scene renders excluded). Code rounds are common to M and B.

| | R1 the one who broke the lens | R3 the viewpoint that knew the alibi | R1+R3 total |
|---|---|---|---|
| Affected scenes | 10 | 7 | 17 |
| **Render+verify calls** M / B | **22 / 22** (edit 10 + card 12 / render 10 + extract 12) | **18 / 15** (7 + 11 / 7 + 8) | **40 / 37** (108%) |
| Weighted input M / B | 119,398 / 69,009 | 80,433 / 43,024 | 199,831 / 112,033 |
| Output tokens M / B | 39,265 / 36,531 | 29,988 / 29,200 | 69,253 / 65,731 |
| Price weight M / B | 92,742 / 77,523 | 70,729 / 57,184 | 163,471 / 134,706 (121%) |
| **Collateral change chars** M / B | **86 / 2,796** (3.1%) | **119 / 3,959** (3.0%) | **205 / 6,755 (3.0%)** |
| Total changed chars M / B | 916 / 9,906 | 332 / 6,543 | 1,248 / 16,449 |
| Scenes changed collaterally | s22, s23, s29 (30, 36, 20 chars) | s12, s13, s14, s21 (34, 28, 30, 27 chars) | |
| **Confirmed residual** M / B | 0 / 0 | 0 / 0 | 0 / 0 |
| **Confirmed new contradictions** M / B | 0 / 0 | 0 / 0 | 0 / 0 |
| Borderline (not counted) M / B | 1 / 1 | 1 / 1 | 2 / 2 |
| **Card pass rate** M / B | 10/10 / 10/10 | 7/7 / 7/7 | 17/17 / 17/17 |
| Retries (2nd edit) | 0 | 0 | 0 |
| Cost: with code (shared 2 opus calls) M / B | 246,021 / 230,802 (24 / 24 calls) | 214,642 / 201,097 (20 / 17 calls) | 460,663 / 431,898 (44 / 41 calls) |
| A (spike value) price weight / calls | 138,174 / 4 | 372,985 / 8 | 511,159 / 12 |
| M ÷ A | 1.78× | 0.58× | **0.90×** |
| B ÷ A | 1.67× | 0.54× | 0.84× |

- The judgment values in the B column are the retcon spike's as they are (confirmed residual 0, new contradictions 0, borderline R1 1, R3 1). The spike README's B net cost (R1 230,802, R3 207,377) includes the render and extract calls of new scene s32 (R3), so it differs from the B value above by about 6k only in R3.
- The collateral change total compares only R1+R3: M 205 characters vs B 6,755. It has a different scope from the spike's three-reset total (B 13,569 characters, A 1,250), so we do not compare directly. M's 205 characters is even smaller than A's R1+R3 collateral change (1,250 characters, all in R3).
- Judge items: M R1 introduced 2 (borderline 1, rejected 1), R3 introduced 1 (borderline 1), residual 0. All are sentences the revised IR requires and are also in B's prose. Basis and quotations are in `verification.md`. If all judge items were counted, M's new contradictions would be 3 (then criterion 3 breaks — also stated in the verdict below).
- M scenes that passed the card but kept 1 missing: R3 s12, s14, s18 (the production criterion allows missing ≤ 1). No scene failed on the first attempt, so the retry path was never actually used.

## R2 pilot (first 6 scenes s6–s11, no judgment, excluded from totals)

| | M | B (same 6 scenes) |
|---|---|---|
| Calls | 13 (edit 6 + card 7) | 13 |
| Weighted input / output / price weight | 53,463 / 17,227 / 40,876 | 34,442 / 17,160 / 37,122 (M 1.10×) |
| Per-scene edit distance from the old prose | 22–77 chars (mean 44) | 719–1,191 chars (mean 890) |
| Card pass | 6/6 | 6/6 |

R2's collateral change (changes outside the keyword impact) is not compared because it is a pilot (the other 11 affected scenes are still B's prose as is).

# Verdict: **partial effect**

| Criterion | Value (R1+R3) | Met |
|---|---|---|
| 1. Collateral change ≤ 30% | 205 / 6,755 = **3.0%** (97% reduction) | Yes |
| 2. Render+verify calls ≤ 50% | 40 / 37 = **108%** | **No** |
| 3. Confirmed residual + new contradictions ≤ B | 0 ≤ 0 (3 > 0 if all judge items are counted) | Yes (confirmed basis) |
| 4. Card pass rate not lower | 17/17 = 17/17 | Yes |

Key point: **collateral change almost vanished (97% reduction), but neither calls nor tokens went down.**
- Why calls did not drop: nearly all calls are one edit/render + one card/extract per scene. The number of affected scenes (17) is the same and the card verification is done identically, so the call count is structurally the same (M 40 vs B 37; M's card tier2 took a little more). With this design 50% or lower cannot come out. To cut calls, one must reduce the number of affected scenes (edit only scenes whose spec difference is at sentence level) or batch the verification.
- Why tokens rose: the input gains the old prose + change list, so weighted input is about 1.8× (199,831 vs 112,033). Output did not drop because the whole fixed prose is rewritten (69,253 vs 65,731). Switching to sentence-level patches (output only the changed sentences) may reduce output, but we did not test it this time.
- Accuracy was kept: confirmed residual 0, confirmed new contradictions 0. The card passed everything on the first attempt.

## Cost comparison with A (code cost included)

- R1+R3: M 460,663 / 44 calls vs A 511,159 / 12 calls. **M is 0.90× A in price weight** (B is 0.84×). But in R1 A is much cheaper (M 1.78×), and only in R3 is M cheaper (0.58×; A rewrote 6 episodes). In calls, M 44 vs A 12 is 3.7×.
- **All three including R2 were not measured.** The extrapolation, multiplying the pilot's per-scene cost (6,813) by 17 scenes and adding B's code cost 104,123, is about 220k for R2, about 680k for all three vs A 597,774 (about 1.14×). So with R2 included M appears to become more expensive than A, but this is an extrapolation.
- That is, "does B now beat A": by total cost it won in R1+R3, but that is thanks to R3's A cost (372,985) and not M's render-side savings (M's render+verify was actually 21% more expensive than B's). The dominant cost per reset is the opus code round (about 140k–150k).

# Call count

- **Actual calls 55 / cap 60.** R1: edit 10 + card 12 + judgment 1 = 23. R3: edit 7 + card 11 + judgment 1 = 19. R2 pilot: edit 6 + card 7 = 13. All in `calls.jsonl` (labels `rN-M-…`, judgment `rN-judge-M`). Failed calls 0 (`err` all empty), retries 0.
- **Waste 0.** The R2 pilot (13 calls) was not used in the judgment but is a measurement used as per-scene comparison data. The remaining 5 calls were not used.
- Tests (unit tests, the fake completer over the whole run `-me-dry`) made 0 real calls. The mock run output was not kept in the repository.
- PG loads: once per story (old 1 + revised 3). On the first attempt, an IR file name error caused one more load of the old story (2 loads of the old story in total, not model calls). Later loads read the `ir/` cache.

# Limitations

- **R2 not measured.** Because of the call cap of 60, only 2 of the 3 were judged (n = 2). R2 is the reset where A missed affected episodes and so the most important case, yet there is no M judgment. Criterion 3 was compared against the R1+R3 sum (0), not B's three-reset sum (0 residual + 1 new contradiction, occurring in R2).
- **One judgment, one checker (me).** The 3 items the judge (sonnet, once) raised are all sentences the revised IR requires, so I classified them as borderline/rejected. If this classification differs, criterion 3 breaks; "partial effect" stays, but the accuracy claim weakens. The same sentences are in B's prose too, yet B's judge did not flag them, so judgment variation is large.
- **New scenes used B's prose as is.** s32 in R1 and R2, and s32 and s33 in R3, have no old prose and cannot be edited. B's render was put into M's full text as is, and the render calls of these scenes were excluded from both costs.
- **The affected scene set is the same.** M edited the hash-changed scenes that B re-rendered as they are. These include scenes whose spec difference is just one line of "previous plot", and such scenes could be skipped without an edit call. That would cut calls, but we did not test it this time.
- **Whether the edit is "minimal" depends on the collateral-change definition.** It is a value counted by the keyword definition (fixed beforehand). The edit distance is at character level and does not distinguish an edit that replaces a whole sentence from a one-character fix.
- **The card only checks whether facts are included.** A rewritten sentence that is awkward or has a wobbling register still passes. Register and style were not judged this time.
- **haiku once each.** The variance of edit quality was not measured (one replication).
- This spike was tested only at 12-episode scale. In a long serial with many affected scenes, the collateral-change savings may be larger.

# Files

- `README.md` (this document), `verification.md`, `calls.jsonl` (55 real calls), `results.json`, `run.log`
- `ir/{base,r1,r2,r3}/` — scene request JSON cache (PG `render_scene` output after `dieg load`; `index.json` gives the order)
- `rN/diff/` (per-scene old→new spec differences), `rN/prompts/` (full prompts), `rN/edited/` (M's edit results and card reports), `rN/m/` (M's working directory: `novel/` = M novel scenes, `calls.jsonl` = M calls with the same labels as dieg), `rN/judge/` (`m.prompt.txt`, `m.out.txt`, `m.quotes.md`)
- Tools: `tools/serial/minedit.go`, `tools/serial/minedit_test.go`, `tools/serial/main.go` (one line for the `minedit` subcommand)

Reproduction:

```
export PATH=<go-install>/bin:$PATH
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./tools/serial minedit -me-out examples/value/minedit -me-dump                      # IR cache (PG load, no model calls)
go run ./tools/serial minedit -me-out /tmp/me-dry -me-ir examples/value/minedit/ir -me-dry -me-budget 100   # fake completer test
go run ./tools/serial minedit -me-out examples/value/minedit -me-budget 60 -me-plan 1,3,2:6                  # real (subscription claude CLI)
```
