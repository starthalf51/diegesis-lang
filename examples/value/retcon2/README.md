# Reset benchmark (plan 7) — when a setting changes mid-serial, does Diegesis (B) leave fewer contradictions than direct writing (A)?

# Hypothesis

When one fact in the middle of a story is changed, B (`dieg revise`: the AI fixes the code, PG verifies, and only the changed scenes are re-rendered) has
**fewer remaining contradictions, costs no more, and touches less unrelated text** than A (the AI re-reads and rewrites the text directly). In the earlier spikes (`examples/value/retcon`, `examples/value/minedit`), B had fewer residual contradictions but higher cost and collateral change.
We re-measure with the same method whether what plan 7 put into B (the PG state digest in the render prompt with staleness decided by a state hash, in-scene sentence-level patch render + card verification falling back to a full render, a new lint warning) changes this.

# Pre-registered record (written before the real calls, not edited after the run)

## 6 resets (the request sentences are literally identical for A and B. List file: `retcons.json`)

The material is the same as the earlier spike: the finished stories of `examples/serial/run3` (A = `a/eps/e01~e12.md`, B = `b/`). For each reset **both arms start fresh from the original** (not cumulative). Only characters common to both stories are used (A changed Kang Min-jae to Im So-jeong and B changed to Yun Tae-o, so those characters are not used).
R1–R3 are **literally the same sentences** as the earlier spike (for comparison; the test `TestRetcon2ListKeepsR1toR3` checks this). R4–R6 are new (the earlier spike's R4 "Su-a is Yeong-ho's granddaughter" was not run and is not used here either).

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

# Method

- **A's revision method**: the same code as the earlier spike (`arm_A` in `tools/serial/retcon.go`; `serial`'s `PickPrompt` → `RewritePrompt` for each chosen episode → `SummaryPrompt`). Model opus/medium. Input is the outline, cumulative summary, per-episode directions, and the 12 episodes as of the end of run3.
- **B's revision method**: `dieg revise <work> "<request>" --skip s32,s33,s34,s35 --render-effort low` with the current defaults (plan 7: card verification, PG state digest + state-hash staleness, sentence patch render + full render fallback, coding opus, render haiku low, extraction sonnet). Newly added: `--skip` (Rerender skips those scenes), `state_stale=`/`skipped=` in the summary line, `DIEG_CALL_CAP` (hard call cap).
- **State-hash baseline (`dieg stamp`)**: run3's B prose was rendered before plan 7, so `.hash` has only the IR hash. As is, it is treated as "old hash = state unknown → not stale", so plan 7's state staleness would never work. So we load the original story into PG once and append each scene's state-digest hash to `.hash` (0 model calls, prose unchanged). This is the value a post-plan-7 pipeline would have left at render time. Each reset starts from this stamped baseline. The stamp is applied only once (skipped if the marker file `base/.stamped` exists).
- **Judgment (referee)**: the same prompt as the earlier spike (`rcJudgePrompt`), sonnet once per arm and reset. I check judge items against the source one by one and classify confirmed/borderline/rejected (`verification.md`). A single verifier.
- **Collateral change**: the same deterministic code as the earlier spike (`collateral`, character edit distance, units where the pre-set keywords above occur in the pre-revision source = affected units, characters changed outside affected units = collateral change). Units are A episodes / B scenes.
- **Cost**: the same formula as the earlier spike (weighted input = input + 0.1×cache read + 1.25×cache write; price weight = Σ model factor × (weighted input + 5×output), opus 1, sonnet 0.6, haiku 0.2, `examples/serial/README.md`). Judgment calls are counted only in call counts, not in cost.
- **Procedure and budget**: real-call cap **240**. A budget guard right before each call, and every call is recorded in `calls.jsonl`. A and B alternate per reset (R1 A→B→judgment, R2 …). If the budget is short, we stop after finishing one whole reset (reset start condition: used + 57 ≤ 240). Inside dieg, `DIEG_CALL_CAP` blocks calls that exceed the remaining budget − 2 for judgments. First, under the `noclaude` shim, we run unit tests and the whole fake-completer dry run (`-rc-dry`), then run in one process without the shim.
- **n = 6, 12 episodes.** 12 episodes is a scale that fits entirely in A's context and a length that favors A's cumulative-summary method. There is one verifier, me. The results are appended below.

# Run record

- Dry run: under the `noclaude` shim, `go vet`, `go test`, and the full fake-completer run (`-rc-dry`, all 6, 0 calls). The commands the real run issues are as follows (printed by the dry run).
  - `dieg stamp <out>/base/b` (once, no model calls)
  - `dieg revise <out>/rN/b "<request>" --skip s32,s33,s34,s35 --render-effort low` (N = 1..6, requests as in the table above)
  - For flags, only the new names `-rc-list`, `-rc-skip-from`, `-rc-b-reserve` were added, which do not overlap the existing spike (`retcon`, `-rc-out/-rc-src/-rc-budget/-rc-only/-rc-recompute/-rc-dry`). No other spike process was running.
- Ran as one process without the shim (`go run ./tools/serial retcon -rc-out examples/value/retcon2 -rc-list examples/value/retcon2/retcons.json -rc-budget 240`). Because of the 10-minute limit we waited for one background process.
- **The first attempt failed at the `dieg stamp` stage with 0 calls.** Plan 7's state digest (`internal/pg/digest.go`) counted even non-scene events (`taeo_departs`, `taeo_gone`: events that are not on screen, `discourse_ord` 0 for both) in the "immediately preceding scene" lookup and raised the error "subquery returned multiple rows". We fixed it by adding `AND x.shown` to the two preceding lookups and added a regression test (`TestDigestTwoOffstageEvents`, failing before the fix). This bug would have made every `dieg revise` with the digest enabled fail on a story like run3 that has two or more off-screen events.

# Results (3 resets / 6 planned)

**Only R1–R3 were measured. R4–R6 could not be run**: 229 calls were used through R3 (cap 240), and one reset costs 64–75 calls for B alone, so the start condition (used + 57 ≤ 240) was not met. We stopped at a reset boundary (A and B pair up).
Excluded scope: s32–s35 were not rendered in either arm (`skipped=s32,s33,s34,s35`, A could not choose episode 12) and not judged. However, in R3 the **new scene s36** that the reset inserted in episode 4 (1,038 characters) was rendered. Since it is a new scene created by the reset, it is excluded from collateral change and noted separately (the same handling as s32 in the earlier spike's R3).

## Table (A / B)

| | R1 who broke the lens | R2 glass shard location | R3 alibi moved earlier | Total (3) |
|---|---|---|---|---|
| **Confirmed residual** | 1 / 0 | 2 / 1 | 0 / 0 | **3 / 1** |
| **Confirmed new contradictions** | 0 / 0 | 0 / 0 | 1 / 0 | **1 / 0** |
| Borderline (not counted) | 1 / 3 | 0 / 1 | 1 / 2 | 2 / 6 |
| **Calls** (judgment excluded) | 3 / 69 | 4 / 64 | 8 / 75 | **15 / 208** |
| Weighted input | 29,197 / 546,977 | 55,251 / 405,598 | 147,736 / 467,252 | **232,184 / 1,419,827** |
| Output tokens | 11,174 / 104,505 | 15,482 / 72,302 | 43,004 / 111,720 | 69,660 / 288,527 |
| Price weight (opus 1, sonnet 0.6, haiku 0.2) | 85,067 / 590,691 | 132,661 / 340,586 | 362,756 / 471,159 | **580,484 / 1,402,436** |
| B ÷ A (price weight) | 6.94× | 2.57× | 1.30× | **2.42×** |
| **Collateral change chars** | 0 / 113 | 0 / 168 | 945 / 225 | **945 / 506** |
| Total changed chars | 445 / 744 | 424 / 609 | 3,448 / 708 | 4,317 / 2,061 |
| Units changed / total (A episodes, B scenes) | 1/11 · 18/31 | 2/11 · 16/31 | 6/11 · 19/31 | |
| Changed among keyword-affected units | A 1/6 · B 12/14 | A 2/6 · B 8/10 | A 5/8 · B 10/14 | |
| Newly created scenes (excluded from collateral) | — / 0 | — / 0 | — / s36 1,038 chars | |
| **Scenes B re-rendered** | 29 | 28 | 31 (existing 30 + new 1) | 88 |
| └ sentence patch / full render | 29 / 0 | 28 / 0 | 30 / 1 (s36) | **87 / 1** |
| └ due to IR / due to state | 26 / 3 | 23 / 5 | 27 / 3 | **76 / 11** |
| B code rounds | 4 | 2 | 2 | |

- "Due to IR" = scenes whose render request hash (IR) changed, "due to state" = scenes whose request is unchanged and only the PG state digest hash changed (`state_stale=` in the `dieg revise` summary line; subtracting this number from `changed` gives IR). The new scene s36 is neither, it is an "addition".
- Weighted input, price weight, collateral change, and affected units use the same code and formulas as the earlier spike (`collateral`, `costOf`). The 6 judgment calls (sonnet) are not included in cost.
- Confirmed/borderline classification and basis: [`verification.md`](verification.md). Judge text: `rN/judge/`. Revised text: `rN/a/eps/` (only the revised episodes), `rN/b/novel/`, `rN/b/story/`. Machine-readable results: `results.json`.

## R1–R3 alongside the earlier spike (A, B full re-render, M minimal edit)

The earlier spike values come from `examples/value/retcon/README.md` and `examples/value/minedit/README.md` (B then was the pre-plan-7 `dieg revise`, calls are net values excluding the s32 waste, code rounds included). "B plan7" is this run's value. The story is the same run3, but the earlier spike removed episode 12 and s32 from the B copy, while this time we left the code and blocked only the render.

| | Arm | Calls | Price weight | Confirmed residual / new contradictions | Collateral change chars |
|---|---|---|---|---|---|
| **R1** | A (earlier) / A (this time) | 4 / 3 | 138,174 / 85,067 | 1 / 0 · 1 / 0 | 0 / 0 |
| | B full re-render (earlier) | 24 | 230,802 | 0 / 0 | 2,796 |
| | M minimal edit (earlier) | 24 | 246,021 | 0 / 0 | 86 |
| | **B plan7 (this time)** | 69 | 590,691 | 0 / 0 | 113 |
| **R2** | A (earlier) / A (this time) | 3 / 4 | 86,615 / 132,661 | 3 / 0 · 2 / 0 | 0 / 0 |
| | B full re-render (earlier) | 41 | 230,962 | 0 / 1 | 6,814 |
| | M minimal edit (earlier) | (pilot only; no judgment) | — | — | — |
| | **B plan7 (this time)** | 64 | 340,586 | 1 / 0 | 168 |
| **R3** | A (earlier) / A (this time) | 8 / 8 | 372,985 / 362,756 | 0 / 1 · 0 / 1 | 1,250 / 945 |
| | B full re-render (earlier) | 19 | 207,377 | 0 / 0 | 3,959 |
| | M minimal edit (earlier) | 20 | 214,642 | 0 / 0 | 119 |
| | **B plan7 (this time)** | 75 | 471,159 | 0 / 0 | 225 |

A uses the same method and the same input, but the values differ a little because of model output differences (in R2, the earlier A's residual of 3 was the confirmed value from the judge at that time; this time it is 2 in episode 9).

## What can be read from this B

- **Collateral change dropped greatly** (B total 506 characters; earlier B 13,569 characters, A 945). This is because the in-scene sentence patch fixes only the changed sentences. For the first time B is smaller than A.
- **Calls and tokens rose greatly** (calls 208 vs 15, price weight 2.42×). The reason is that the render targets grew: in each reset 28–31 scenes became render targets again (10–17 in the earlier spike). **Cause (confirmed by the reviewer; my first note, "the coding AI's arbitrary expression edits", was a symptom):** plan 7's new lint warnings (`relative-time-fact`, `retell-known`) fire **even on the untouched run3 story**, and `ReviseLoop` blocks if there is even one warning (`loop.go` `Blocking`, the `revise.go` verdict). So in every reset the second code round (`fix-r2`) rewrote the fact statements `vote_scheduled` ("twelve days later") and `demolition_schedule` unrelated to the reset, and the retelling (`says`) lines in s5, s11, s17, and s21. On top of that, `previously` (`009_render.sql`) puts the fact statements of the previous 3 scenes into `RenderHash`, so when one scene changes, the following 3 also become stale. The reviewer's simulation reproduced the IR-stale set exactly. Scenes stale only by the state hash were 3–5. A patch once + a card extraction once per scene makes about 2.2 calls per scene. (Fix and re-measurement are in "Re-run" below.)
- Accuracy: B has confirmed residual 1 and new contradictions 0; A has 3 and 1. A's residuals are where the pick call missed the affected episodes (R1 episode 11, R2 episode 9), and B's one residual is a sentence in s11 that the patch missed.

# Verdict

Apply the pre-registered rule to the 3 measured resets (R1–R3).

| Condition | Value | Met |
|---|---|---|
| 1. Sum of confirmed residual + new contradictions B < A | B 1 < A 4 | Yes |
| 2. In no reset is B ≥ A + 2 | R1 0 vs 1, R2 1 vs 2, R3 0 vs 1 | Yes |
| 3. Collateral change sum B ≤ A | 506 ≤ 945 | Yes |
| 4. Total weighted cost B ≤ 1.0 × A | 1,402,436 ÷ 580,484 = **2.42×** | **No** |

**Verdict: revision evidence weak.** B is better on accuracy (residuals, new contradictions) and collateral change — for the first time since the earlier spike, B also met the collateral change condition. But cost is 2.4× A and calls are 14× (208 vs 15). Only 3 of the 6 resets were measured, so this too is an n=3 verdict.

# Call count

- Used **229 / cap 240**: A 15 (pick 3 + episode rewrites 9 + summary 3), B 208 (`rN/b/calls.jsonl`: code rounds 8, patch/render/extract 200), judgment 6. All are in `calls.jsonl` (B calls have the `rN-B-` prefix). Failed calls 0.
- **Waste 0.** s32–s35 were not rendered (`--skip`; the 8-call waste from recreating s32 in the earlier spike did not occur). The 11 unused calls were left because they were too few to fit one more reset. Call guard: used + 1 + reservation ≤ 240 right before every call, and inside dieg `DIEG_CALL_CAP` allows the remaining budget − 2 for judgments.
- Attempt 1 (`dieg stamp` failure) had 0 model calls.
- **PG loads:** the original story was loaded once for the baseline stamp, and once more because of attempt 1's failure (the bug above) (2 in total; not model calls). `dieg revise` loads by itself to verify the candidate (per reset). For cause investigation there was one diagnostic that loaded the baseline and R1's result and compared the scene requests (0 model calls).

# Limitations

- **n = 3 (6 planned).** R4–R6 (new owner, order, identity) could not be measured because of budget. These three are the new kinds and were not measured, so we do not know whether "the same holds for new kinds of resets". The verdict rule was written for 6 and applied to 3.
- **12 episodes, a length that favors A's summary method.** At 12 episodes, A finds most affected episodes by reading only the summary and the previous episode (A calls 3–8). How A's misses grow and how B's cost structure changes on a longer serial cannot be known from this experiment.
- **The verifier is one person, me.** The judge (sonnet, once) and my cross-check can differ, and my judgment enters the borderline classification (B 6 vs A 2). If B's 3 borderline R1 items were counted as new contradictions, B's total would be 4 (equal to A) and condition 1 would break. The keyword audit cannot see contradictions that miss the keywords.
- **The two arms' stories differ.** They are stories written separately from the same idea, so the number of units a reset reaches differs (keyword-affected units A 6–8 / B 10–14). Units also differ — A an episode (about 4,000 characters), B a scene (about 1,300 characters) — so directly comparing collateral change characters has limits.
- **The baseline stamp is a configuration I made.** run3 prose has no state hash, so we attached the original story's state hash (`dieg stamp`; 0 IR hash mismatches). Without this stamp, state staleness does not work. Even with it, only 3–5 scenes were stale solely by the state hash, so that feature's gain was small in these resets.
- **B's cost was heavily driven by code edits unrelated to the reset (cause above: existing warnings block + the `previously` hash).** These two were fixed in the re-run. The first run's opus code rounds alone were 763,227 (1.31× A), so the first run could not meet condition 4 even with free rendering. One replication only (variance not measured).
- **The style and register of patch renders were not judged.** The card verification looks only at whether facts are included. A patched sentence can become awkward in context.
- **s32–s35 are excluded** (judgment scope s1–s31 + R3's new scene s36). We did not check whether B's episode 12 code matches the reset. We prevented A from choosing episode 12 (we did not pick resets whose effects are only in episode 12).
- **Tool changes:** `dieg revise --skip`, `dieg stamp`, `DIEG_CALL_CAP`, the summary line `state_stale=`/`skipped=`, and the state digest SQL fix (the `shown` filter) were added in this work.

# Files

- `README.md`, `retcons.json` (6 resets), `verification.md`, `calls.jsonl` (229 rows), `results.json`, `run.log`
- `base/b/` — the run3 B baseline with the state-hash stamp (made with no model calls; each reset starts fresh on top of this)
- `r1/ r2/ r3/` — `a/eps/` (revised episodes), `a/calls.jsonl`, `b/` (the revised work: `story/`, `novel/`, `calls.jsonl`), `b.log` (dieg output, including the `revise:` summary line), `judge/` (`{a,b}.prompt.txt`, `.out.txt`, `.quotes.md`), `judge.calls.jsonl`
- Tools: `tools/serial/retcon.go` (`-rc-list`), `cmd/dieg/stamp.go`, `internal/write/revise.go` (`Stamp`, `StateStale`, `Skip`)

Reproduction:

    export PATH=<go-install>/bin:$PATH
    export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
    go run ./tools/serial retcon -rc-out examples/value/retcon2 -rc-list examples/value/retcon2/retcons.json -rc-budget 240

# Correction (after reviewer cross-check)

- **Borderline classification correction:** in the first run, B's R1 s21 is a **confirmed new contradiction** (right after writing that the attendant's name on the ledger page is smeared and unreadable, it asserts "Baek Yeong-ho, who broke it as written in the ledger"; the two clash within the same scene). Then the first-run sum is **B 2 (R2 residual 1 + R1 new contradiction 1) vs A 4**. The "B 1" and "borderline 3" in the table above are values before this correction.
- **Sensitivity:** if judge items are counted the same per-location way as for A (B R1 s21, s24, and s27 each as a new contradiction), B is 4 vs A 4, so condition 1 (B < A) breaks, and in R1, B 3 ≥ A 1 + 2, so condition 2 also breaks. The first run's accuracy advantage was sensitive to this classification.
- **Code round cost:** B's 8 opus code rounds alone (`rev*-code`, `rev*-fix`) in the first run were 763,227 (R1 373,307 / R2 155,280 / R3 234,640), **1.31×** A's total 580,484. So even removing the render targets unrelated to the reset (churn), condition 4 could not have been met with the first run's number of code rounds unless render cost were made 0. The next levers are coding output (re-emitting whole files) and caching.

# Re-run (B only, after the product fix)

The fixes (commit `0d29773`): (d) `ReviseLoop` first lints the pre-revision story once (no model calls) and makes only diagnostics (rule + message) not in that baseline the targets to block or fix. (b′) The staleness hash on the generation path is changed to `genHash`, which omits `previously` (`render.RenderHash` and the corpus path are unchanged; `TestCorpusPromptsUnchanged` passes). Old-format `.hash` files are rewritten with the new hash by `dieg stamp` (re-stamped from the original run3: 31 scenes, 30 differed from the old hash).
The same reset sentences, the same judge prompt, the same confirmation method, the same collateral change/cost code. **A was not re-run: we use the first run's A results and A judgment output as they are (same input).** One process, executed after a dry run, budget 100 (`rerun/calls.jsonl`, **96** used: B 93 + judgment 3; failed 0, waste 0). The reset start condition was remaining budget ≥ 36.

| | R1 | R2 | R3 | Total (3) |
|---|---|---|---|---|
| A (first run, reused) calls / price weight | 3 / 85,067 | 4 / 132,661 | 8 / 362,756 | 15 / 580,484 |
| **B re-run calls** | 16 | 29 | 48 | **93** |
| B weighted input / output | 185,824 / 38,008 | 209,464 / 28,561 | 267,738 / 46,673 | **663,026 / 113,242** (A 232,184 / 69,660) |
| **B price weight** | 229,394 | 146,293 | 216,196 | **591,883** |
| B ÷ A | 2.70× | 1.10× | 0.60× | **1.02×** |
| (B first run price weight / calls) | 590,691 / 69 | 340,586 / 64 | 471,159 / 75 | 1,402,436 / 208 |
| Of which B code rounds (calls / price weight) | 2 / 171,060 | 1 / 57,096 | 1 / 83,533 | 4 / 311,689 (first run 8 / 763,227) |
| **Confirmed residual / new contradictions** (judge + my check) | 0 / 0 | 0 / 0 | 0 / 0 | **0 / 0** (A 3 / 1) |
| Borderline (not counted) | 0 | 0 | 1 | 1 |
| **Collateral change chars** | 0 (A 0) | 0 (A 0) | 33 (A 945) | **33** (A 945) |
| Total changed chars | 363 | 144 | 113 | 620 |
| Scenes changed / 31 | 5 | 7 | 3 | |
| Newly created scenes | — | — | s11b 815 chars | |
| **Scenes re-rendered** | 7 | 14 | 21 (existing 20 + new 1) | 42 |
| └ sentence patch / full render | 7 / 0 | 14 / 0 | 20 / 1 (s11b) | **41 / 1** |
| └ due to IR / due to state | 5 / 2 | 6 / 8 | 1 / 19 | **12 / 29** |
| B code rounds | 2 | 1 | 1 | |

- The new scene's number came out as s11b rather than s36, but it is R3's new scene (episode 4). The number is below 32 and it was rendered.
- Check (end of `verification.md`): R1 and R2 judge items 0, and the keyword audit residual is 0 too (the first run's s21, s24, s27 "as written in the ledger" and s11 "rock crevice" disappeared). R3's one judge new contradiction was described as a location mismatch between s11b and s9, but s11b is placed after s11 (end of episode 4), so the judge's order premise is wrong → borderline (the scene starts at "the entrance of the first-floor stairs" but ends with "went down the stairs", so the space is ambiguous), not counted.
- **How to read it:** calls fell 208 → 93, price weight 2.42× → 1.02×, and render targets 88 → 42. In R3, 19 scenes are stale due to state but the patches changed only 33 characters, so those 19 patches were nearly wasted (the staleness the state hash reports is too broad; the next lever).

## Re-run verdict (same pre-registered rule)

| Condition | Value | Met |
|---|---|---|
| 1. Sum of confirmed residual + new contradictions B < A | B 0 < A 4 | Yes |
| 2. In no reset is B ≥ A + 2 | R1 0 vs 1, R2 0 vs 2, R3 0 vs 1 | Yes |
| 3. Collateral change sum B ≤ A | 33 ≤ 945 | Yes |
| 4. Total weighted cost B ≤ 1.0 × A | 591,883 ÷ 580,484 = **1.02×** | **No (narrowly)** |

**Verdict: revision evidence weak (narrowly).** B leads on accuracy and collateral change, and cost has become almost the same as A's (1.02×; calls are still 6× at 93 vs 15). Condition 4 breaks by a 2% difference, so it is near the verdict threshold and within this measurement's noise (B only once, A also only once). It is, however, n=3.

## Re-run limitations

- **n = 3 (6 planned), 12 episodes, one verifier** remain. For A we used the first run's values as they are (the same input, but A's output can differ per call; A's R1 and R2 also differed between the earlier spike and this one).
- The re-run B differs from the first-run B starting from the code rounds (code calls 8 → 4). We could not separate how much of the improvement is due to the product fix and how much to the coding AI's call-to-call variation.
- Since `previously` was removed from the staleness hash, when a previous scene's facts change but only the "previous plot" line of a later scene differs, it is not re-rendered. Narration that relied on that prompt line may remain stale (we did not find any in the judgment this time).
- PG loads: 1 stamp in the re-run, and a baseline lint and candidate-verification load for each `dieg revise`.

# Re-run files

`rerun/` — `calls.jsonl` (96 rows), `results.json`, `run.log`, `base/b/` (stamped with the new hash), `r1/ r2/ r3/` (`b/` revised work, `b.log`, `judge/b.*`). The first run's A results are in `r1/ r2/ r3/a/` and `judge/a.*` above. Reproduction: add `-rc-out examples/value/retcon2/rerun -rc-only 1,2,3 -rc-b-only -rc-b-reserve 35 -rc-budget 100` to the command above.

# Correction 2 (after the final review)

- **The 29 state-stale patches in the re-run were effectively empty patches.** For scenes where only the state digest changed, the old and new scene requests are the same, so `SceneDiff` was empty, the patch prompt's "change list" was just `(none)`, and the model emitted `{"edits":[]}`. The card passed and the scene got a new stamp. The statement in the table above, "R3's 19 scenes stale due to state, 33 changed characters", is the result of the state change not being propagated, and not only of state staleness being broad. Therefore **the 1.02× is a value that puts the cost of actually propagating state changes too low.** The fix: when saving a scene, leave the state text (`<scene>.<backend>.state.txt`) next to `.hash`, and for scenes stale only by state, patch with the difference between the old and new state lines in the change list. Old stamps with no stored state text get a full render. Measuring again is planned (the figures in this document are from before the fix).
- Also corrected in the same review: the gap between the first run's 1.02× and 2.42× cannot be attributed to the baseline fix alone (the staleness hash `genHash` also changed, and there is run-to-run variation). The scenes re-rendered per reset are 7, 14, and 21 of 31 (the earlier "28–31" in the body is the first run's figure).
