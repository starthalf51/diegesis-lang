# Hypothesis ② — changing a setting mid-serial (retcon) is safer and cheaper with Diegesis

This is one of the validation spikes that check whether writing with Diegesis (the AI codes → PG verifies → the renderer writes prose), i.e. arm B, has a clear reason to exist.

**Hypothesis ②.** Changing one fact in the middle of a story is safer and cheaper with B.
- B: the IR and the hash backlog mechanically find the affected scenes, and only those scenes are re-rendered.
- A: must re-read and rewrite the text, and misses things.

**Status: run complete (3 resets / 4 planned).** The "Design (recorded beforehand)" section was written before the real calls and not edited after the run (see "Limitations" below for R4 not run and the handling of the B copy). The results are appended after the design.

# Design (recorded beforehand)

## Material and method

- Material: the finished stories of `examples/serial/run3`. A = `a/eps/e01~e12.md`, B = `b/` (the Diegesis story `story/`, prose `novel/`).
- **A's revision method** = the same prompts and order as A's revision in `tools/serial` (`direct.go` `armA.revise`): choose affected episodes once with `PickPrompt` → `RewritePrompt` once per chosen episode (earlier episodes use the revised version) → update the cumulative summary once with `SummaryPrompt`. Input is the outline, cumulative summary, final per-episode directions (plan + replan-1 + replan-2), and the 12 episodes as of the end of run3. The model is opus/medium, same as the harness. The code is `tools/serial/retcon.go` (it calls the same prompt functions as they are).
- **B's revision method** = the same `dieg revise <work> "request" --render-effort low` as the harness's `runBRevise` (card verifier at the dieg default, render haiku effort low, coding opus, extraction sonnet). No `--scenes` cap.
- The request sentence is **literally identical** for both arms. For each reset, each arm applies it independently to **a fresh copy of the final run3 story** (not cumulative).
- **Removed from the B copy:** s32–s35, which were never rendered (pipeline cutoff). `dieg revise` sees scenes with no render record (hash) as "new scenes" and renders them, so left as is, 4 scene renders unrelated to the reset would be mixed into cost and collateral change. So we removed `story/ep12/` and `story/ep11/s32.dieg.md` from the copy (`dieg check` passes after removal). B is judged only on s1–s31.
- **Judgment scope:** A is episodes 1–11, B is scenes through episode 11 (s1–s31, everything rendered). Neither looks at episode 12 (B has no episode 12). A can revise up to episode 12 and that cost is counted.
- PG: `dieg revise` loads the `yeoul` work itself to verify the candidate story. The 4 resets each reload the same work name with different content, so "load only once" cannot be kept (each revision has different content). There is no separate load.

## 4 resets (request sentences, same for A and B)

The characters of the two stories are the same from the idea (`examples/serial/idea.txt`). In revision 1, A changed Kang Min-jae to Im So-jeong and B changed to Yun Tae-o and Kim Hae-sol, so we use only the **common characters other than these** (Seo-yun, Mal-sun, Do Gyeong-tae, Ha-eun, Baek Yeong-ho, Ryu Ji-hwan, Mun Gi-cheol, Su-a). All facts first appear in episodes 2–5 (the memorial service, the glass shard, climbing the lighthouse), and later episodes rely on them.

| ID | Kind | Request |
|---|---|---|
| R1 | Who caused it | 그날 밤 등대 렌즈를 깨뜨린 사람은 도경태가 아니라 백영호이며(불을 다시 켜려다 혼자 램프에 부딪혀 깼다), 렌즈가 깨진 경위를 말하거나 짐작하는 모든 장면을 이에 맞게 고쳐라. ("The person who broke the lighthouse lens that night was not Do Gyeong-tae but Baek Yeong-ho (trying to relight it, he bumped into the lamp alone and broke it); fix every scene that says or guesses how the lens was broken.") |
| R2 | Prop location | 정수아가 두꺼운 유리 조각을 주운 곳은 등대 아래 바위틈이 아니라 선착장 방파제 끝이며, 그 장소가 나오는 모든 장면을 이에 맞게 고쳐라. ("The place where Jeong Su-a picked up the thick glass shard was not among the rocks below the lighthouse but at the end of the pier breakwater; fix every scene where that place appears.") |
| R3 | When a secret was learned (moved earlier) | 서윤은 도경태의 병원 알리바이가 거짓이라는 것을 진료부를 보기 훨씬 전인 4화(등대에 오른 날)에 이미 알게 되며(누구에게서 알았는지는 정해도 된다), 4화 이후 서윤이 그 알리바이를 여전히 믿는 것처럼 나오는 모든 부분을 이에 맞게 고쳐라. ("Seo-yun already learns in episode 4 (the day she climbed the lighthouse), long before seeing the medical record, that Do Gyeong-tae's hospital alibi is false (you may decide who told her); fix every part after episode 4 where Seo-yun appears to still believe that alibi.") |
| R4 | A supporting character's relationship | 정수아는 백영호와 남남이 아니라 그의 손녀이며, 수아가 나오거나 백영호의 가족·혼자 사는 처지가 언급되는 모든 장면을 이에 맞게 고쳐라. ("Jeong Su-a is not a stranger to Baek Yeong-ho but his granddaughter; fix every scene where Su-a appears or Baek Yeong-ho's family or living alone is mentioned.") |

(The same sentences as `retcons` in `tools/serial/retcon.go`. Reset R1 reverses the setting that both stories wrote as "Do Gyeong-tae broke the lens in a scuffle at the lighthouse".)

## Measurement

- **(a) Residual contradictions.** One judgment per arm and reset (sonnet, a prompt in the same style as the harness judgment: reset sentence + full text after revision → the places that still conflict as `residual` and the contradictions newly created by the revision as `introduced`, as JSON with source quotations). I check each judge item against the source one by one and record confirmed/rejected with quotations in `verification.md`. The table counts **only confirmed** residuals. `introduced` is compared against whether the error was already in the pre-revision source, and only new ones are counted (separate row).
- **(b) Cost.** Call count and weighted tokens. Exactly the run3 README formula: weighted input = input + 0.1×cache read + 1.25×cache write, price weight = Σ(model factor × (weighted input + 5×output)), opus 1, sonnet 0.6, haiku 0.2. The model classification by call label is also the same as the harness `modelOf`. Judgment calls are not put into cost and are counted only in the call-count budget.
- **(c) Collateral change.** Unit = episode for A, scene (rendered prose file) for B. We count the character (rune) edit distance between before and after for each unit (deterministic, `editDist`). **Affected units** are units where the pre-set keywords occur in the **pre-revision source**: R1 `렌즈|파편|유리 조각|깨뜨`, R2 `조각|바위틈|벼랑 밑|벼랑 바로`, R3 `알리바이|병원|맹장|진료부|흉터`, R4 `수아|영호`. Collateral change = characters changed outside the affected units. (An A episode is long, so rewriting one episode also changes unrelated sentences inside it. That is counted as collateral change too — it is the "rewriting risk" this hypothesis speaks of.) Total changed characters and the number of changed units are also noted.

## Verdict criteria (fixed beforehand)

- **Revision evidence present:** B's confirmed residual total < A's total, **and** in no reset is B residual ≥ A residual + 2, **and** B's collateral change is smaller.
- **Revision evidence weak:** residuals lean toward B but not all the conditions above are met.
- **Revision evidence absent:** otherwise.
- Cost is always written alongside. **12 episodes is a scale that fits entirely in A's context, and that is the region that favors A** (A sees only the summary + the previous episode, but even scanning everything per reset costs little). It does not stand in for results on a longer serial.

## Budget and procedure

- Real-call cap 120. Each call goes through a budget guard right before it (used + 1 + judgment reservation ≤ 120), and every call is logged to `calls.jsonl` (B's dieg calls are copied from `rN/b/calls.jsonl` written by dieg with the label prefix `rN-B-`). `dieg revise` makes calls inside one process, so we reserve a worst case of 22 before starting.
- One process runs R1 (A→B→judgment) → R2 → R3 → R4 in order. Even if cut off for lack of budget, it is cut at a reset boundary, so the two arms are fair.
- First, with the `noclaude` shim on PATH, we ran the unit tests and the whole flow with the fake completer (`-rc-dry`) to check the command: `dieg revise <work> "<request>" --render-effort low`.

# Results

Only resets R1–R3 ran. **R4 could not run because of the call cap** (113 used; the R4 start condition "used + 30 ≤ 120" was not met; see call count below). The verdict criteria were applied to the 3 as pre-set.

## Table (A / B per reset)

| | R1 who broke the lens | R2 glass shard location | R3 when the alibi was learned | Total |
|---|---|---|---|---|
| **Confirmed residual** (judge items, after verification) | A **1** / B **0** | A **3** / B **0** | A **0** / B **0** | A **4** / B **0** |
| Confirmed new contradictions (created by the revision) | 0 / 0 | 0 / 1 | 1 / 0 | A 1 / B 1 |
| Borderline (not counted) | 0 / 1 | 0 / 0 | 0 / 1 | A 0 / B 2 |
| More residuals found by audit (missed by the judge) | 0 / 0 | A +1 / 0 | 0 / 0 | A +1 / B 0 |
| **Calls** | 4 / 27 (net 24) | 3 / 44 (net 41) | 8 / 21 (net 19) | 15 / 92 (net 84) |
| Weighted input (input+0.1 read+1.25 write) | 52,569 / 166,834 | 31,690 / 210,222 | 147,900 / 142,199 | 232,159 / 519,255 (net 489,142) |
| Output tokens | 17,121 / 54,231 | 10,985 / 66,265 | 45,017 / 48,212 | 73,123 / 168,708 (net 155,871) |
| Price weight (opus 1, sonnet 0.6, haiku 0.2, output ×5) | 138,174 / 243,352 (net 230,802) | 86,615 / 241,765 (net 230,962) | 372,985 / 217,589 (net 207,377) | 597,774 / 702,706 (net 669,141) |
| Units rewritten / total (A episodes, B scenes) | 1/11 | 1/11 | 6/11 | |
| └ changed among keyword-affected units | A 1/6 · B 7/14 | A 1/6 · B 10/10 | A 5/8 · B 3/14 | |
| Total changed chars (edit distance) | 701 / 9,906 | 263 / 16,595 | 4,098 / 6,543 | 5,062 / 33,044 |
| **Collateral change chars** (outside affected units) | A **0** / B **2,796** (s22, s23, s29) | A **0** / B **6,814** (7 scenes) | A **1,250** (episode 4) / B **3,959** (4 scenes) | A **1,250** / B **13,569** |
| Newly created scenes (excluded from collateral) | — / s32 1,966 chars | — / s32 1,425 chars | — / s32 1,280 chars (retcon scene), s33 1,983 chars | |

- "Net" is the value with the render and extract calls of the scenes `dieg revise` recreated (R1 s32, R2 s32, R3 s33) removed, because we emptied the s32 slot (episode 11) in the B copy. R3's s32 is a retcon scene newly inserted in episode 4, so it stays in the net value (it is reset work).
- A can also rewrite episode 12. In R1 it rewrote episode 12 (an affected unit, not collateral; the judgment looked only at episodes 1–11).
- The price weight uses the run3 README's assumptions (s = 0.6, h = 0.2, output = input × 5) and is not an actual price list. The 6 judgments (sonnet) are not included in either side's cost.
- Judge items and the confirm/reject basis: [`verification.md`](verification.md). Judge text: `rN/judge/`. Revised text: `rN/a/eps/` (only the revised episodes; the rest are the same as run3), `rN/b/novel/`, `rN/b/story/`. Machine-readable results: `results.json` (printed at run time), `results2.json` (extra-scene separation and net cost recomputed from files, no calls).

## What happened in each reset

- **R1.** A's pick call chose only episodes 10 and 12. The one-line ledger summary in episode 11 ("lens lower part broken during the scuffle") stayed as the old setting (confirmed 1). B re-rendered 10 scenes in episodes 8–11 (7 of the 14 keyword-affected scenes), and the judge item was left as borderline 1. Re-rendering also changed the text of scenes unrelated to the reset (s22, s23, s29, 2,796 characters).
- **R2.** A's pick call chose only episode 4. The places it missed are episode 3 (the line that first says where the shard was picked up) and episode 9 (two places) (confirmed 3, audit +1). B re-rendered 17 scenes and changed the shard location to the end of the breakwater in all scenes (residual 0). Instead, episode 6 gained "the lighthouse light at the end of the breakwater", which clashed with the lighthouse's location (new contradiction 1). This reset is the closest to the **situation hypothesis ② describes** (A misses, B finds mechanically).
- **R3.** A rewrote 6 episodes, 4–9 (8 calls) and had no residual. The time markings in episodes 4 and 8 clashed as day/evening (new contradiction 1, minor). B inserted a new scene s32 in episode 4 (Tae-o shows that there is no hospital record) and re-rendered seven scenes s12–s14 and s18–s21 (21 calls). In this case B was cheaper than A by price weight (217,589 vs 372,985).

# Verdict

The pre-set criteria are applied to the 3 resets.

| Condition | Value | Met |
|---|---|---|
| B's confirmed residual total < A's total | B 0 < A 4 (adding new contradictions, B 1 < A 5) | Yes |
| In no reset is B residual ≥ A + 2 | R1 0 vs 1, R2 0 vs 3, R3 0 vs 0 | Yes |
| B's collateral change is smaller | B 13,569 chars > A 1,250 chars (per reset too, B is larger or equal) | **No** |

**Verdict: revision evidence weak.** Residual contradictions are fewer for B (0 vs 4), and that difference came from R2, where A's pick call missed the affected episodes. This matches the hypothesis's core claim ("A misses"). But the collateral change condition was not met. B's `revise` re-renders scenes whose render request changed wholesale, so sentences unrelated to the reset change too (13,569 characters, 10, 17, and 7 scenes changed). A writes only the chosen episodes (sentences inside those episodes are also rewritten, but mostly inside affected episodes), and its collateral change was small.

**B costs more.** Calls 92 vs 15 (net 84), weighted input 519k vs 232k, output 169k vs 73k. In price weight too, B is 702,706 vs A 597,774 (1.18×; 1.12× if net 669,141). One reset is 21–44 calls for B and 3–8 calls for A. Price weight favors B only once, in R3 (0.58×).

**12 episodes is a region that fits entirely in A's context, which is a condition favoring A.** A's `pick` chooses episodes by looking only at the cumulative summary, and at this scale the summary was detailed enough that it caught nearly all affected episodes in R1 and R3. It missed only in R2 (the cumulative summary `a/summary.md` that `pick` sees has no entry for the glass shard at all). With a longer serial the summary may lose facts and A's omissions may grow, and B's scene-level impact identification may become more important. This spike could not measure that.

# Call count

- Used **113 / cap 120.** A 15 (pick 3 + scene rewrites 9 + summary 3), B 92 (`rN/b/calls.jsonl` written by dieg: code rounds 6, render/extract 86), judgment 6. All are in `calls.jsonl` (B calls have the `rN-B-` prefix). Failed calls 0 (`err` empty).
- **Waste:** because we emptied s32 (never rendered) in the B copy, the calls where `dieg revise` recreated scenes s32/s33 are R1 3 + R2 3 + R3 2 = 8 (the table's "net" removes these). It was a judgment error at the design stage: after emptying, `dieg check` passed but PG verification caught an "unpaid setup" and the AI filled the slot. Had we not emptied it, the renders of the 4 scenes s32–s35 would have been mixed in, so either way B's cost got some padding.
- 0 real calls in the pre-start unit tests and the full fake-completer run (`-rc-dry`) (`noclaude` shim).
- Guards: "used + 1 + judgment reservation ≤ 120" right before a call, `dieg revise` reserves 22 before starting, and a reset starts when "used + 30 ≤ 120". R4 did not start since 113 + 30 > 120 (running only A would not make a fair comparison, so we did not).

# Limitations

- **R4 not run.** Only 3 of the 4 were measured. Totals and the verdict are based on R1–R3. R4 (replacing a supporting character's relationship) is in the design.
- **n = 3, one judgment each, one human checker.** There are items the judge (sonnet, once) missed (A R2 +1). The keyword audit cannot see contradictions that miss the keywords. It was done to the same scope for A and B, but B's confirmed residual of 0 may also be "not found" rather than "0".
- **The two arms' stories differ.** They are two stories written separately from the same idea, so the number of scenes a reset reaches is not the same (keyword-affected units: R1 A 6/B 14, R2 A 6/B 10, R3 A 8/B 14). B's story scattered the fact across more scenes.
- **The definition of collateral change is the keywords.** The affected units were set by keywords in the pre-revision source (recorded beforehand). If the keywords are loose and many units count as "affected", collateral change shrinks; if tight, it grows. The units also differ: A is an episode (about 4,000 characters), B is a scene (about 1,300 characters). Newly created scenes (4) were excluded from collateral.
- **The B copy is not intact.** To avoid the never-rendered s32–s35 we emptied episode 12 and s32. B was judged only on s1–s31 and A only on episodes 1–11. A can also rewrite episode 12, so the cost includes the episode 12 rewrite (R1).
- **A's `pick` input is the cumulative summary as of the end of run3.** This summary contains traces of the last revision. It is the same method the harness gave at each revision.
- **Are A's and B's judgments at the same place?** The judge prompt is the same, but B's text is headed per scene and A's per episode.
- This spike neither proves nor disproves "B is better at revision". Residual contradictions were fewer for B, and cost and collateral change were smaller for A. The directions split.

# Files

- `README.md` (this document), `verification.md`, `calls.jsonl`, `results.json`, `results2.json`
- `r1/ r2/ r3/` — `a/eps/` (revised episodes), `a/calls.jsonl`, `b/` (the revised Diegesis work: `story/`, `novel/`, `calls.jsonl`), `b.log`, `judge/` (`{a,b}.prompt.txt`, `.out.txt`, `.quotes.md`), `judge.calls.jsonl`
- Tools: `tools/serial/retcon.go` (subcommand `retcon`), `tools/serial/retcon_test.go`

Reproduction:

```
export PATH=<go-install>/bin:$PATH
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./tools/serial retcon -rc-out examples/value/retcon -rc-budget 120
go run ./tools/serial retcon -rc-out examples/value/retcon -rc-recompute   # recompute cost and collateral change from files (no calls)
```
