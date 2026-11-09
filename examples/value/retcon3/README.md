# Retcon benchmark re-run (plan 8): summary

Diegesis v0.7.0, 6 retcons (R1-R6) of a 12-episode story, one run per arm, one verifier. B = `dieg revise` (Diegesis), A = direct AI edit.

| | A (direct edit) | B (`dieg revise`) |
|---|---|---|
| Confirmed residual + new contradictions | 6 (8 with boundary items) | 3 (5 with boundary items) |
| Unrelated text changed (characters) | 2,247 | 47 |
| Priced weighted cost | 1,339,803 | 1,247,242 (0.93x of A) |
| Calls (judging excluded) | 33 | 221 (6.7x) |

- The pre-registered rule is met ("evidence for revision"), but a cost advantage is NOT established. 0.93x depends on A editing nothing in R6; treating R4 like R6 gives 1.12x.
- Caveat: 1,069 of A's 2,247 unrelated characters come from a keyword-dictionary gap in R3 e04; excluding it gives about 1,178 vs 47, so the conclusion holds.
- Limitations: n=1 per arm, one verifier, 12 episodes (favours A), A's cost varies a lot (R2: 3.7x the previous run). R6 A may be under-counted (judge possibly missed e08:41); this favours A.

---

# Reset benchmark re-measurement (plan 8) — v0.7.0, R1–R6, both arms fresh

**Summary:** 6 resets on v0.7.0 (one run per arm, one verifier): confirmed residual + new contradictions B 3 vs A 6, unrelated text changed 47 characters vs 2,247 (1,069 of A's characters came from a keyword-dictionary gap in R3 e04; without it, about 1,178 vs 47, the conclusion unchanged). Price-weighted cost is B ≈ A (0.93×). But this value leans on A having changed nothing in R6; if R4 is handled the same as R6, it is 1.12× (reviewer 1.13×). B makes 6.7× the calls. Under the pre-registered rule it is "revision evidence present", but a cost advantage is not established. (Note the 12-episode length and the 3.7× variation of A's cost in R2.)

# Pre-registered record (honest provenance: the 6 resets and the verdict rule are byte-identical to retcon2 commit cc8eb45. This README file itself was created after the first real calls, and the rule of counting 1 per location was newly decided in this run)

This carries over the pre-registered record of `examples/value/retcon2/README.md` as is. The request sentences are in `retcons.json` (byte-identical to retcon2).
Differences this time: (1) **A is also run fresh** (no reuse). (2) B uses the v0.7.0 defaults: PG state digest + state-hash staleness, sentence patches for state-stale scenes using the old/new state line difference, card verification, render haiku low (`--render-effort low`), the existing-warning baseline (block only on new diagnostics), `--skip s32,s33,s34,s35`. (3) Real-call cap **320**. (4) Computation and judgment use the same code as retcon2 (`tools/serial retcon`).

## 6 resets (requests verbatim; pre-set keywords = definition of affected units)

| ID | Kind | Request | Pre-set keywords (definition of affected units) |
|---|---|---|---|
| R1 | Who caused it | 그날 밤 등대 렌즈를 깨뜨린 사람은 도경태가 아니라 백영호이며(불을 다시 켜려다 혼자 램프에 부딪혀 깼다), 렌즈가 깨진 경위를 말하거나 짐작하는 모든 장면을 이에 맞게 고쳐라. ("The person who broke the lighthouse lens that night was not Do Gyeong-tae but Baek Yeong-ho (trying to relight it, he bumped into the lamp alone and broke it); fix every scene that says or guesses how the lens was broken.") | `렌즈\|파편\|유리 조각\|깨뜨` |
| R2 | Prop location | 정수아가 두꺼운 유리 조각을 주운 곳은 등대 아래 바위틈이 아니라 선착장 방파제 끝이며, 그 장소가 나오는 모든 장면을 이에 맞게 고쳐라. ("The place where Jeong Su-a picked up the thick glass shard was not among the rocks below the lighthouse but at the end of the pier breakwater; fix every scene where that place appears.") | `조각\|바위틈\|벼랑 밑\|벼랑 바로` |
| R3 | When a secret was learned (moved earlier) | 서윤은 도경태의 병원 알리바이가 거짓이라는 것을 진료부를 보기 훨씬 전인 4화(등대에 오른 날)에 이미 알게 되며(누구에게서 알았는지는 정해도 된다), 4화 이후 서윤이 그 알리바이를 여전히 믿는 것처럼 나오는 모든 부분을 이에 맞게 고쳐라. ("Seo-yun already learns in episode 4 (the day she climbed the lighthouse), long before seeing the medical record, that Do Gyeong-tae's hospital alibi is false (you may decide who told her); fix every part after episode 4 where Seo-yun appears to still believe that alibi.") | `알리바이\|병원\|맹장\|진료부\|흉터` |
| **R4** | **Prop owner** | 두꺼운 유리 조각의 주인은 정수아가 아니라 분교 교사 도하은이며, 하은이 오래전 바닷가에서 주워 간직해 온 것을 수아가 빌려 갖고 있다. 이 조각을 누가 주웠고 누가 갖고 있는지를 말하거나 짐작하는 모든 장면을 이에 맞게 고쳐라. ("The owner of the thick glass shard is not Jeong Su-a but the branch-school teacher Do Ha-eun; Ha-eun found it long ago on the shore and kept it, and Su-a is borrowing it. Fix every scene that says or guesses who found or holds this shard.") | `조각\|보물` |
| **R5** | **Order of two events** | 서윤은 수아의 유리 조각 이야기를 듣거나 조각을 보기 전에 먼저 등대에 올라 계단 열세 번째 칸의 새 못을 본다. 즉 3화의 유리 조각 이야기와 4화의 등대 오르기의 순서가 바뀌어, 서윤은 등대 계단을 오른 뒤에야 수아의 유리 조각을 알게 된다. 두 사건의 선후에 기대는 모든 장면(서윤이 그때까지 아는 것, 말하는 것, 떠올리는 것)을 이에 맞게 고쳐라. ("Before hearing Su-a's glass-shard story or seeing the shard, Seo-yun first climbs the lighthouse and sees the new nail on the thirteenth stair. That is, the order of episode 3's shard story and episode 4's lighthouse climb is swapped, and Seo-yun learns of Su-a's shard only after climbing the lighthouse stairs. Fix every scene that depends on the order of the two events (what Seo-yun knows, says, and recalls up to then).") | `조각\|계단\|열세\|새 못\|휜 못\|올랐\|올라` |
| **R6** | **A supporting character's identity** | 류지환은 개발사 현장 책임자가 아니라 도경태의 조카(이장 누이의 아들)이며, 개발사와 이장 사이를 잇는 중개인으로 섬에 와 있다. 류지환의 신분·소속·이장과의 관계가 나오거나 짐작되는 모든 장면을 이에 맞게 고쳐라. ("Ryu Ji-hwan is not the developer's site manager but Do Gyeong-tae's nephew (the village head's sister's son), and is on the island as a broker between the developer and the village head. Fix every scene where Ji-hwan's status, affiliation, or relationship with the village head appears or is guessed.") | `류지환\|지환\|개발사` |

All are facts that first appear or are set in episodes 2–5 and later episodes rely on them (the shard: both stories have it in episode 3 / the alibi: episode 2 / the lighthouse climb: episode 4 / Ryu Ji-hwan: in A episodes 3, 5, 6, 8, 9; in B s1, s7, s22 — the number of appearances differs per story). We did not pick resets whose effects are only in episode 12.

## Verdict rule (fixed beforehand, as in the brief)

- **"Revision evidence present"** = all four of the following are met:
  1. Sum of confirmed residual + new contradictions B < A,
  2. In no reset is (residual + new contradictions) B ≥ A + 2,
  3. Collateral change (characters) sum B ≤ A,
  4. Total weighted cost B ≤ 1.0 × A (price weight; code rounds are not excluded, only the judgment is).
- If only some are met, **"weak"**; if none, "no evidence".
- Table: per reset, residuals, new contradictions, calls, weighted tokens, collateral change, number of re-rendered scenes (due to IR / due to state), number of patch/full renders.

## Excluded scope (verdict recorded beforehand)

- B's s32–s35 exist only as code in run3 and were never rendered. **In neither arm do we render or judge anything after s32.** B blocks the render with `dieg revise --skip s32,s33,s34,s35` (the code is left as is so PG verification stays intact; if we delete episode 12 as in the earlier spike, the AI fills the blanks and creates waste). For A, since that range corresponds to episode 12, the selection stage is prevented from choosing episode 12 (episodes 1–11 only).
- Judgment scope: A is episodes 1–11, B is the rendered s1–s31.

## Counting method (identical for both arms, recorded beforehand)

If the same error appears in several places, it is counted as **1 per location** (for A the position within an episode, for B the scene). The same method is used for both arms. Confirmed = judge items that I confirmed against the source text; borderline items are not counted (same as retcon2).

## Method summary

The same as retcon2: material `examples/serial/run3`, in each reset both arms start fresh from the original, judgment sonnet once per arm and reset, collateral change and cost formulas identical (judgment calls are excluded from cost and included in call counts). The call guard runs right before each call, every call is recorded in `calls.jsonl`, A→B alternate per reset, and if the budget cannot cover a whole reset we stop cleanly and write down the scope. The retcon2 first-run and re-run values are placed side by side in the results table.


# Run record

- Dry run: under the `noclaude` shim `go vet` and `go test -count=1 ./...` passed, and the full fake-completer run (`-rc-dry`, 6 resets, 0 calls). The actual commands are the same as retcon2: `dieg stamp <base/b>` once, `dieg revise <rN/b> "<request>" --skip s32,s33,s34,s35 --render-effort low` (N=1..6).
- Run: as a process without the shim, `go run ./tools/serial retcon -rc-out examples/value/retcon3 -rc-list examples/value/retcon3/retcons.json -rc-budget 320 -rc-b-reserve 60`. `dieg stamp`: stamped 31/35 scenes (IR mismatch 0). Log `run-1.log`.
- **Stopped at R4 (tool problem)**: A's pick call chose only "[12]" (the excluded episode 12), the `no episode` error ended the process (`results-r1-3.json`, `run-1.log`; 1 call consumed, wasted). Minimal fix (`tools/serial/retcon.go` arm_A): in `-rc-list` mode, if the pick is empty, proceed as "A changes nothing" instead of erroring (the summary update call is also skipped). Then we ran the process once more with `-rc-only 4,5,6` (`run-2.log`; the earlier process had already ended, no concurrent runs). The R4 pick was called again (a new call) and this time chose normally. In R6, A's pick again chose only episode 12, so A changed nothing (1 call).
- Calls **267 / 320** (A 33, B 221, judgment 12, waste 1; 269 if the 2 re-judgments are added). All 6 completed. The table's calls and costs do not include the 2 re-judgments (judgment calls are excluded from cost).
- `calls.jsonl` (269 rows: main run 267 + re-judgments 2), B's calls in `rN/b/calls.jsonl`, A's in `rN/a/calls.jsonl`.

# Results (all 6)

Confirmed = judge items checked against the source text (single verifier). The basis for confirmation is in "Check" below. The counting method is the pre-registered record above (1 per location, identical for both arms).

| | R1 | R2 | R3 | R4 | R5 | R6 | Total |
|---|---|---|---|---|---|---|---|
| **Confirmed residual + new contradictions A / B** | 1 / 0 | 0 / 1 | 2 / 1 | 2 / 0 | 0 / 0 | 1 / 1 | **6 / 3** |
| (residual / new contradictions) | A 1/0, B 0/0 | A 0/0, B 1/0 | A 1/1, B 0/1 | A 2/0, B 0/0 | 0/0, 0/0 | A 1/0, B 1/0 | |
| Calls A / B (judgment excluded) | 3 / 13 | 11 / 30 | 8 / 48 | 6 / 39 | 4 / 58 | 1 / 33 | **33 / 221** |
| Price weight A / B | 84,740 / 148,049 | 486,680 / 150,143 | 362,857 / 229,789 | 238,138 / 311,035 | 162,122 / 234,221 | 5,266 / 174,005 | **1,339,803 / 1,247,242** |
| B ÷ A | 1.75× | 0.31× | 0.63× | 1.31× | 1.44× | 33× | **0.93×** |
| Collateral change chars A / B | 0 / 0 | 953 / 0 | 1,069 / 8 | 225 / 0 | 0 / 2 | 0 / 37 | **2,247 / 47** |
| B patch / full render | 6 / 0 | 14 / 0 | 20 / 1 (s11a) | 17 / 0 | 24 / 1 (s11a) | 14 / 0 | **95 / 2** |
| B scenes re-rendered: due to IR / due to state | 4 / 2 | 6 / 8 | 1 / 19 (+1 new) | 8 / 9 | 4 / 20 (+1 new) | 4 / 10 | **27 / 68** |
| B code rounds | 1 | 1 | 1 | 3 | 1 | 1 | |

(In R5, B deleted s6 and inserted s11a anew. The number of state-stale scenes is the count of `state_stale=` in the summary line; IR = changed − state.)

## Alongside retcon2 (R1–R3)

| | A calls/price/residual+new/collateral | B calls/price/residual+new/collateral |
|---|---|---|
| retcon2 first run | 15 / 580,484 / 3+1 / 945 | 208 / 1,402,436 / 1+0 (2 after correction) / 506 |
| retcon2 re-run (B only) | (first-run A reused) | 93 / 591,883 / 0+0 / 33 |
| **retcon3 (R1–R3)** | 22 / 934,277 / 2+1 / 2,022 | 91 / 527,981 / 1+1 / 8 |

Summing R1–R3 only: A 22 calls, 934,277, collateral 2,022; B 91 calls, 527,981, collateral 8 (B ÷ A 0.57×). This time A rewrote 9 episodes in R2 (retcon2's A rewrote 2), so the cost differed greatly: A's variance is large.

## Check (cross-check of judge items; every quotation is in the source in `rN/judge/*.quotes.md`)

- Confirmed: R1 A episode 11 ledger (a sentence of the old setting), R2 B episode 24 (rock crevice and breakwater end together in the same scene), R3 A episode 7 memo question mark (established in episode 4 yet unconfirmed) + episode 9 new contradiction (midday garlic vs evening beans), R3 B new contradiction (the new episode 4 scene and the immediately following scene's "decided to go up / let's go up"), R4 A two places in episode 9 (answers that she picked it up under the cliff), R6 A episode 3 self-introduction (A did not fix it), R6 B episode 7 "류 소장님" ("Manager Ryu") (still addressed as manager even after just stating he is a broker).
- Borderline (not counted): R3 A the other two items in episode 7 (the sentence that she had known, and 'only then'), R3 B s18 "틀렸을 수도 있어요" ("may have been wrong") (she also says it is not true in the same scene), R3 B s11a "사실이 아닐지도 모른다고 생각했다" ("thought it might not be true") (the code is `believes not hospital_alibi`; Seo-yun's cautious reaction). Rejected: R5 B s11 "머릿속에서 맞춰졌다" ("it fit together in her head") (nothing points to the shard). **Correction (reviewer)**: I first wrote the R5 B item "s11a was placed before s9–s11" as "the judge's premise is wrong", but it was we who were wrong. `bUnits` built the judge input with s11a placed before s9 because `Atoi("11a")=0` (retcon2's s11b has the same bug). We fixed the sort (`sceneLess`, test `TestSceneLessSuffix`) and re-judged only B for R3 and R5 in the correct order (`judge/b2.*`, 2 calls; the old `judge/b.*` is kept as evidence). Re-judgment: R3 B new contradiction 1 (s11a "올라가 보기로 했어요", "I decided to go up and look", after she has already gone up) confirmed + borderline 2; R5 B's one residual item was rejected. Details in `verification.md`.
- Sensitivity: counting all borderline items gives A 8 / B 5. The conclusion (B < A) does not change.

# Verdict (the pre-registered rule)

| Condition | Value | Met |
|---|---|---|
| 1. Sum of confirmed residual + new contradictions B < A | 3 < 6 | Yes |
| 2. In no reset is B ≥ A + 2 | at most 1 vs 0 in R2 (difference +1) | Yes |
| 3. Collateral change sum B ≤ A | 47 ≤ 2,247 | Yes |
| 4. Total weighted cost B ≤ 1.0 × A | 1,247,242 ÷ 1,339,803 = **0.93×** | Yes |

**Verdict: under the pre-registered rule, "revision evidence present" (all 4 conditions met). But a cost advantage is not established.** B ÷ A has three values: **0.93×** (the run as is), **0.80×** (excluding R6; A's R6, where A changed nothing, is 5,266, and removing it favors B), **1.12×** (applying the same rule as R6 even though R4's first pick failed, i.e. treating it as A changing nothing: A = 1,339,803 − 238,138 + first pick 7,222 = 1,108,887, B ÷ A = 1,247,242 ÷ 1,108,887; the reviewer's calculation of 1.13× differs by rounding). The empty-pick rule was made after R4 failed, so R4 got a re-pick and R6 did not (asymmetry). In that case A's R4 residual would also grow, so accuracy favors B even more. Condition 4 flips depending on the handling. Also note the large variance in A's cost (R2's A 486,680 vs retcon2's 132,661, 3.7×) and the 12-episode length (which favors A's summary method).

# Limitations

- **A's R6 may be under-counted** (possibly the judge missed e08:41). This works in A's favor.
- Caveat: 1,069 of A's 2,247 characters came from a keyword-dictionary gap in R3 e04. Without it, about 1,178 vs 47, and the conclusion stays.
- n = 6, 12 episodes, a single verifier, one run per arm (variance not measured): A's cost differed from retcon2 by up to 3.7× (R2) for the same reset.
- **In A's R6, the pick stage chose only episode 12 and changed nothing** (a pick unusable because of the excluded scope). We counted this as an "A failure" (residual 1). R4's first pick failed for the same reason too and was called once more (the re-pick is a retry outside the protocol; its 1-call cost is noted as waste and not put in the price weight).
- The run was split across two processes (because of a tool fix). The earlier process ended at R4's pick after finishing through R3, and no model-call state remains. Both recorded to the same `calls.jsonl` and the call guard's total continued.
- B makes 6.7× A's calls (221 vs 33). Costs became similar because B's renders are haiku low, with a low token unit price. State-stale scenes (68) outnumber IR-stale scenes (27), so the state patch calls inflated the call count.
- The style of patch renders was not judged. s32–s35 excluded.
- PG loads: 1 baseline stamp + a candidate-verification load per `dieg revise`. No extra diagnostic loads.

# Files

`README.md`, `verification.md`, `retcons.json`, `calls.jsonl` (269 rows), `results.json` (R4–R6), `results-r1-3.json`, `run-1.log`, `run-2.log`, `base/` (the run3 path + the stamped hash list `stamped-hashes.txt`; prose is identical to run3), `r1/ … r6/` (`a/eps/`, `a/calls.jsonl`, `b/story`, `b/rounds`, `b/calls.jsonl`, `b/novel/` = changed and new scenes only + `b/novel-unchanged-hashes.txt` (the stamped hashes of the rest), `judge/`: the old `b.*` (order-bug input) and the re-judgment `b2.*` (R3 and R5)). To reduce size we deleted all of `base/b`, `rN/b/novel.md` (it is in the judge input as is), and the duplicate `run.log`.
