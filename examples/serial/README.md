# Serial measurement — 12-episode plan, 2 revisions (Plan 4 Task 6)

We wrote the same serial item side by side with A (the model writes directly) and B (Diegesis: the AI writes `.dieg.md`, PG verifies, and the renderer writes the prose).
The harness is `tools/serial`. After episode 4 and after episode 8 we gave both arms the same revision request, and compared by one blind judgment.
Because of the call budget, **both arms were cut at episode 11** (episode 12 not included). All figures below are for episodes 1–11 + 2 revisions.

## Results

| Item | A (direct) | B (Diegesis) | B/A |
|---|---:|---:|---:|
| Cumulative continuity errors (confirmed after verification) | **4** | **3** | 0.75 |
| └ Range including borderline | 4~6 | 3~4 | |
| └ Per 10,000 characters (confirmed / including borderline) | **0.89** / 1.34 | **1.24** / 1.66 | 1.39 |
| Calls | 33 | 168 | 5.1 |
| Actual input tokens (input + cache read + cache write) | 329,448 | 767,087 | 2.3 |
| Cache-weighted input (input + 0.1×read + 1.25×write) | 411,794 | 777,491 | 1.89 |
| └ opus (claude-opus-5-5) | 411,794 (33 calls) | 414,122 (16 calls) | 1.01 |
| └ sonnet (claude-sonnet-5-5) | 0 | 363,369 (152 calls) | — |
| Output tokens | 185,283 | 239,441 | 1.29 |
| └ opus / sonnet | 185,283 / 0 | 86,702 / 152,739 | |
| Body length (episodes 1–11, characters) | 44,824 | 24,153 | 0.54 |
| Revision 1 (supporting character leaves / replaced): calls / weighted / residual inconsistencies | 3 / 32,418 / **1** | 39 / 115,813 / **3** | |
| Revision 2 (secret truth changed): calls / weighted / residual inconsistencies | 7 / 123,461 / **0** | 31 / 105,481 / **1** | |
| Per revision average: calls / weighted / residual | 5 / 77,940 / 0.5 | 35 / 110,647 / 2 | |
| B scenes shipped without passing verification | — | **11 / 28** | |

- The shared `plan` (once, not counted for either arm) and the judgment (once, opus, weighted 96,768) are outside the table.
- No price weighting (`-price`) was given. The costs in the table are per-model tokens.
- Residual inconsistencies are values where a person verified the judge's per-episode marks (the judge's original marks: A 1, 1; B 3, 1). The verification record is [`run1/judge/verification.md`](run1/judge/verification.md).
- 11 of B's 28 scenes were shipped without passing verification (0 hallucinations, 0 leaks, missing ≤ 1) within 3 attempts (`b.log`, `b/novel/*.report.json`). The harness permits exit 3. Of the scenes s15, s16, and s17 where revision 1's residual appeared, s16 and s17 are scenes that did not pass (s15 passed).

## Cumulative cost per episode (cache-weighted input, in thousands)

| Stage | A cumulative calls | A cumulative | B cumulative calls | B cumulative | B/A |
|---|---:|---:|---:|---:|---:|
| Episode 1 | 3 | 16.1 | 7 | 32.8 | 2.04 |
| Episode 2 | 5 | 38.4 | 14 | 69.3 | 1.80 |
| Episode 3 | 7 | 60.5 | 20 | 124.3 | 2.05 |
| Episode 4 | 9 | 84.2 | 33 | 173.8 | 2.06 |
| Revision 1 | 12 | 116.6 | 72 | 289.6 | 2.48 |
| Episode 5 | 14 | 141.7 | 79 | 327.6 | 2.31 |
| Episode 6 | 16 | 164.1 | 89 | 398.3 | 2.43 |
| Episode 7 | 18 | 186.1 | 103 | 482.1 | 2.59 |
| Episode 8 | 20 | 210.3 | 110 | 525.6 | 2.50 |
| Revision 2 | 27 | 333.7 | 141 | 631.1 | 1.89 |
| Episode 9 | 29 | 359.3 | 148 | 674.9 | 1.88 |
| Episode 10 | 31 | 386.1 | 157 | 723.4 | 1.87 |
| Episode 11 | 33 | 411.8 | 168 | 777.5 | 1.89 |

```
Cumulative weighted (k)  0        200       400       600       800
A  ep 1   |#
A  ep 6   |########
A  ep 11  |#####################
B  ep 1   |##
B  ep 6   |####################
B  ep 11  |#######################################
```

A's per-episode increment grew gently from 12k at episode 1 to 26k at episode 11 (A receives not the full text of earlier episodes but the cumulative summary and just the previous episode). B's per-episode increment is uneven at 33k–84k, with no downward trend. At this length (11 episodes, 45k characters for A), A's input growth did not catch up with B. If there is a crossover, it is in a much longer serial (this measurement does not tell).

## Error quotations

1. **B revision 1 residual** — B removed Min-jae in episode 2. "나는 급한 회사 일이 생겨서 오늘 오후 배로 육지에 나가. 다시 섬에는 돌아오지 않을 거야." ("Something urgent came up at work, so I'm going to the mainland on this afternoon's boat. I won't be coming back to the island.") Yet in episodes 6–8 Min-jae keeps investigating. "서윤은 휴대전화를 꺼내 민재의 번호를 눌렀다." ("Seo-yun took out her phone and dialed Min-jae's number.") / "민재는 원무과 창구 앞에 섰다." ("Min-jae stood in front of the administration counter.") B's `continue --brief` followed the shared per-episode directions (fixed before the revision, "episode 6: … Min-jae … finds out there is no admission record") as they were. The two arms' inputs were **not the same**. A's prompt has the harness rule "the `confirmed revisions` in the summary take priority over the outline", and at every revision the request is recorded at the top of the summary under `confirmed revisions` (`tools/serial/direct.go` aHead, the summary prompt). B's `--brief` has no such priority rule (`tools/serial/b.go` runBEpisode). A swapped to Chae-won following this rule. Meanwhile B's code declared `fact minjae_leaves "… 섬에 돌아오지 않는다" true` ("… will not return to the island") in episode 2, but Diegesis has no check on whether a departed character stays departed, so it could not stop Min-jae in episodes 6–8. This part is a gap in Diegesis itself.
2. **A1 (A, confirmed)** — the replacement story appears twice. Episode 2 "민재는 오늘부터 육지에서 교육이 있다며 새벽 첫배로 나갔다고 했다. 대신 올라온 사람은 보건지소의 임채원이었다." ("They said Min-jae had gone out on the first boat at dawn because of training on the mainland from today. The one who came up in his place was Im Chae-won of the health sub-center.") ↔ episode 4 "한서윤 씨죠. 보건지소에 새로 온 임채원이에요." ("You're Han Seo-yun, right? I'm Im Chae-won, newly arrived at the health sub-center.") / "오늘 첫배로 나갔어요. 어머님이 육지에서 쓰러지셨대요." ("He went out on the first boat today. They say his mother collapsed on the mainland.") This clash arose when A rewrote episode 2 at revision 2.
3. **B3 (B, confirmed)** — within the single episode 9, Baek Yeong-ho's confession changes. "나는 도경태의 지시를 받고 등대 불을 두 시간 동안 일부러 껐다." ("On Do Gyeong-tae's orders I deliberately turned off the lighthouse light for two hours.") ↔ "내가 잠든 사이 등대 불이 꺼진 일은 등대 근무 장부에 적혔다." ("The matter of the lighthouse light going out while I slept was written in the lighthouse duty ledger.") B's code declared the former statement a false fact (something made up to hide shame), but the reader cannot see the code and the prose has no signal that it is a lie. By the source-text-based rule it is counted as confirmed. Revision 2's episode 9 residual is the same passage. Verification passed this scene. Fact-level verification cannot catch "does a lie read as a lie".

Suspected judge misreading: A3 — the judge read episode 9's "달력의 십일 칸 … 사흘이 남았다" ("the eleventh box of the calendar … three days left") as 10 days. Counting three days left (14 − 3 = 11) and episode 10's "십이 칸" ("twelfth box") and episode 11's "십삼 칸" ("thirteenth box"), it is 11. But episode 8 uses the same phrase "십일" to mean 10 days, so a reader could be confused. Treated as borderline.

## Limitations

- **One item, one judgment.** The error difference (confirmed 4 vs 3) is a one-case difference in a single sample. It could flip with a different judge or another run.
- **B defects the judge missed (not counted).** B's episode 4 grew to 4 scenes after revision 1, and discovers the new nail on the thirteenth stair as if for the first time three times. Reading only one side further would create asymmetry, so it was not included in the count.
- **Cut at episode 11.** Within the budget of 219 (see below) we could not secure the reserve (22 calls) for B's stage, so we stopped before episode 12 (`stopped_by`: 204 used + 22 reserved). A wrote through episode 12 (35 calls), but the table and judgment are only episodes 1–11. Episode 12's payoffs (finding the ledger page, the vote) are outside the judgment for both arms.
- **The lengths differ.** They received the same length instruction ("about 5000 characters per episode"), but B's prose is 54% of A's. Dividing errors by 10,000 characters, B has more (confirmed 1.24 vs 0.89, including borderline 1.66 vs 1.34). Per character, B's cost is about 3.5× A's (weighted input 32.2 vs 9.2 per character).
- **Model mix.** A is opus (effort medium) only; B is coding opus (medium) + render and extraction sonnet. Per-model tokens are in the table separately. A real price comparison needs a price list, and subscription usage is measured in tokens. We did not apply price weighting.
- **Verification-failed scenes were shipped.** 11/28 of B's scenes were shipped without passing verification. Under a policy of shipping only passes, B would drop scenes or need more calls.
- **Style is outside the judgment.** The checklist looks only at continuity. B's skeleton-mode prose has many lines that recite facts straight into dialogue ("등대 꼭대기 바닥에 쌓인 먼지 위에 얼마 되지 않은 발자국이 찍혀 있다." — "There are few footprints on the dust piled on the floor at the top of the lighthouse." — comes out as dialogue), and repeats the same information ("며칠 뒤면 … 아버지 기일" — "in a few days … the anniversary of Father's death") in every scene.
- **Each arm chose the revision target.** The revision request sentences are fixed in the harness (`revisions.txt`), and which supporting character and which secret to change is not specified. Both arms removed Min-jae (A → Im Chae-won, B → Yun Tae-o). For the secret, A changed it to "the father died by falling from the lighthouse, not by the Haejin-ho", and B to "the light went out while Baek Yeong-ho slept drunk".
- **Run record.** The first attempt (attempt 0) stopped right in the middle of the plan call because of the background-job time limit (2 hours), and was deleted. The recorded calls were 0, but since it was aborted before a response we counted at most 1, and gave the main run a budget of 219 instead of 220. The main run (run1) recorded calls: plan 1 + A 35 + B 168 + judgment 1 = **205**. Both attempts total ≤ 206.
- `b/rounds/revise-r1*` was overwritten by revision 2's output (both revisions used the same label). Revision 1's coding source text was not kept (the call record `b/calls.jsonl` remains).

## Success criteria verdict

| Criterion | Verdict | Basis |
|---|---|---|
| (a) B cumulative errors < A | **Met by raw figures, not met by the length-corrected basis** | Raw figures: confirmed B 3 < A 4 (including borderline 3~4 vs 4~6). Per 10,000 characters: confirmed B 1.24 > A 0.89, including borderline 1.66 > 1.34. Because B's body is 54% of A's, at the same volume B would produce more errors. One item, one judgment. |
| (b) B total cost (weighted) ≤ A | **Not met on weighted input tokens; undetermined on price because there is no price list** | Per-model cache-weighted input: B's opus alone is 414,122, equal to A's 411,794, and sonnet 363,369 is added on top. In output, B's opus is 86,702, less than A's 185,283. Under the assumption (output unit price = input unit price × 5 for both models, sonnet unit price = opus × s), A = 411,794 + 5×185,283 = 1,338,209 and B = 847,632 + s × 1,127,064. So B ≤ A if s ≤ about 0.44. Actual unit prices need a price list, and subscription usage is measured in tokens. |
| (c) Revision residual inconsistencies B < A | **Not met** | After verification B 4 (revision 1: 3 + revision 2: 1) vs A 1 (1 + 0). The cost per revision is also higher for B (average calls 35 vs 5, weighted 110,647 vs 77,940). |

For reference (not a re-score) — sensitivity: all 3 of B's residuals in revision 1 are due to the stale per-episode directions from before the revision. The cause is the priority rule that only A had (error quotation 1 above). Removing these 3, (c) is B 1 vs A 1 (counting B3 as confirmed).

**Conclusion:** For this item and this length, Plan 4's hypothesis (that in a serial B is better on errors, cost, and revision residuals) did not hold. On errors, only the raw figures favor B, and A is better when corrected for length. A is also better on weighted input tokens and revision residuals. Cost on a price basis depends on the sonnet/opus unit price ratio and is undetermined.

## Harness lessons

- After a revision, rebuild the per-episode directions for both arms. Otherwise give `continue` the same priority rule as A ("confirmed revisions take priority over directions").
- Make `dieg revise`'s round labels differ per revision (now both are `revise-r1`, so one overwrites the other).
- Add a check in Diegesis for whether a departed character stays departed (stays-gone). Even with a declaration like `minjae_leaves`, nothing is blocked now.

## Files

- `idea.txt` — the item (9 characters, 2 secrets, 1 lie, twelve days until the vote, 3 foreshadowings, 12 episodes × 2 scenes)
- `revisions.txt` — the 2 fixed revision requests (the same sentences as the harness)
- `run1/` — run outputs. `summary.json`, `metrics.csv`, `plan/`, `a/`, `b/`, `b.log`, `judge/`
- `run1/judge/verification.md` — judgment verification
- We also committed 3 outputs over 500 lines as run data (`SHA256SUMS-uncommitted.txt` was removed).

Reproduction:

```
export PATH=<go-install>/bin:$PATH
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./tools/serial -idea examples/serial/idea.txt -out examples/serial/run1 -budget 219
```

---

# run2 — Plan 5 Task 8 re-measurement

After fixing run1's causes in Plan 5, we ran again with the same item (`idea.txt`) and the same revision requests (`revisions.txt`, the harness's fixed sentences). The outputs are in `run2/`.

**What changed between run1 and run2 (several changes are mixed):**
- B render model: run1 used a sonnet render, run2 uses a **haiku render** (`claude-haiku-5-5`). Extraction is sonnet in both, and in run2 effort low. Coding is opus/medium, the renderer is single, and skeleton mode is on.
- Verifier: run1 is the **old verifier**. run2 does not count authored events that have only an act and no fact as hallucinations (ADR-019 amendment).
- We fixed the target and length in the render prompt. We put a stays-gone rule into the coding prompt (a departed character does not reappear).
- Harness: rebuild the per-episode directions right before each revision (replan, not counted for either arm). Give both arms the same priority rule block ("confirmed revisions take priority over directions"). Revision labels differ per revision.
- A's model and effort are unchanged (opus/medium). But the harness changes above (the per-episode directions changed by the pre-revision replan, the shared priority rule block) changed A's prompt too. Strictly, A is not under the same conditions either.

So the run1 → run2 difference is the sum of the changes above. It cannot be read as the effect of any one.

## Results (episodes 1–10 + 2 revisions)

Within the budget of 220 (see the run record below), B's episode 11 stage could not secure the reserve, so **both arms were cut at episode 10** (`stopped_by`: 191 used + 22 reserved of 211). run1 was cut at episode 11. The run1 figures in the table are the run1 README's episodes 1–11 values. The run1 cumulative at the same episode-10 point is written separately below the table.

| Item | run1 A | run1 B | run2 A | run2 B | run2 B/A |
|---|---:|---:|---:|---:|---:|
| Episodes covered | 1~11 | 1~11 | 1~10 | 1~10 | |
| Cumulative continuity errors (confirmed after verification) | 4 | 3 | **8** | **3** | 0.38 |
| └ Range including borderline | 4~6 | 3~4 | 8~9 | 3~4 | |
| └ Per 10,000 characters (confirmed / including borderline) | 0.89 / 1.34 | 1.24 / 1.66 | **2.12** / 2.39 | **1.00** / 1.34 | 0.47 |
| Calls | 33 | 168 | 31 | 155 | 5.0 |
| Cache-weighted input (sum) | 411,794 | 777,491 | 370,272 | 761,348 | 2.06 |
| └ opus | 411,794 (33 calls) | 414,122 (16 calls) | 370,272 (31 calls) | 435,648 (17 calls) | 1.18 |
| └ sonnet render | — | 187,227 (76 calls) | — | — | |
| └ haiku render | — | — | — | 155,067 (69 calls) | |
| └ sonnet extraction | — | 176,142 (76 calls) | — | 170,633 (69 calls) | |
| Output tokens (sum) | 185,283 | 239,441 | 155,130 | 363,297 | 2.34 |
| └ opus / render / extraction | 185,283 / — / — | 86,702 / 123,853 / 28,886 | 155,130 / — / — | 76,092 / 263,540 / 23,665 | |
| Body length (characters) | 44,824 | 24,153 | 37,712 | 29,896 | 0.79 |
| Revision 1: calls / weighted / residual (confirmed) | 3 / 32,418 / 1 | 39 / 115,813 / 3 | 6 / 90,314 / **1** | 32 / 117,614 / **1** (1~3) | |
| Revision 2: calls / weighted / residual (confirmed) | 7 / 123,461 / 0 | 31 / 105,481 / 1 | 4 / 57,006 / **0** | 35 / 116,661 / **0** (0~1) | |
| Per revision: calls / weighted / residual | 5 / 77,940 / 0.5 | 35 / 110,647 / 2 | 5 / 73,660 / 0.5 | 33.5 / 117,138 / 0.5 | |
| B scenes shipped having passed verification | — | 17 / 28 (61%) | — | 20 / 29 (69%) | |

- The run1 cumulative at the same episode-10 point is as follows (`run1/metrics.csv` ep10 row). A has 31 calls, weighted 386,147, output 172,602. B has 157 calls, weighted 723,427, output 222,252. Compared at the same point, run2's B has 5% more weighted input and 63% more output (haiku render's output is about twice sonnet's). A decreased slightly in both.
- **Per-model values were recounted from call labels.** The harness's `summary.json` `ByModel` groups `render-` and `extract-` all as sonnet (`tools/serial/main.go` `modelOf`, run1 defaults). So run2's "sonnet 138 calls" is actually 69 haiku renders and 69 sonnet extractions. The table above is divided by the labels in `run2/b/calls.jsonl` (`render-*` → haiku, `extract-*` → sonnet, the rest → opus). We did not fix the harness during the run.
- Outside the table: the shared `plan` 3 times (1 at the start + 2 pre-revision replans, weighted 7,305, not counted for either arm). The judgment once (opus, weighted 94,885, output 47,633). A's episode 11 work (2 calls) was excluded since it is after the cut.
- Errors and residuals are values where a person verified the judge's marks (the judge's original marks: errors A 9, B 4; residual A 1, 0; B 3, 1). The verification record is [`run2/judge/verification.md`](run2/judge/verification.md), and it used the same symmetric rule as run1 (confirmed if it is a contradiction from the source text alone).
- 9 of B's 29 scenes (s7, s7b, s9, s10, s13, s14, s16, s18, s19) were shipped without passing verification (0 hallucinations, 0 leaks, missing ≤ 1) within 3 attempts (`run2/b/novel/*.report.json`). 6 of these are scenes newly written or fixed during revisions.

## Price condition formula (the same assumptions as Plan 4)

The assumptions are the same as Plan 4. In all models, output unit price = input unit price × 5, and unit prices are relative to opus (sonnet = s, haiku = h). Input uses the cache-weighted value.

```
A = 370,272 + 5 × 155,130                                  = 1,145,922
B = (435,648 + 5 × 76,092)                    opus share     =   816,108
  + s × (170,633 + 5 × 23,665)                sonnet extract = s × 288,958
  + h × (155,067 + 5 × 263,540)               haiku render   = h × 1,472,767
B ≤ A  ⇔  288,958·s + 1,472,767·h ≤ 329,814
```

- B's opus share alone is 71% of A. Extraction and rendering have to fit in the remaining 29% for B ≤ A.
- Because haiku render's output is large, h's coefficient is large. Even if extraction is free (s = 0), h ≤ 0.224 is required. For example, with s = 0.6, h ≤ 0.106; with h = 0.2, s ≤ 0.122.
- run1's condition was s ≤ about 0.44 (render and extraction both sonnet).
- Actual unit prices need a price list, and subscription usage is measured in tokens. Price weighting (`-price`) was not given. Because `modelOf` groups haiku with sonnet, the current harness's `-price` cannot do this calculation exactly.

## Success criteria verdict (run2)

| Criterion | Verdict | Basis |
|---|---|---|
| (a) B cumulative errors < A | **Met in this sample, but within n=1 noise** | Confirmed B 3 < A 8, including borderline 3~4 < 8~9. Per 10,000 characters B 1.00 < A 2.12 (including borderline 1.34 < 2.39). The cause of the flip from run1 is almost entirely on A's side. A, whose model did not change, got worse from 0.89 to 2.12 per 10,000 characters (confirmed 4 → 8), and B only improved slightly from 1.24 to 1.00. A single arm's run-to-run wobble (+4) is nearly the same as the A–B difference (5). So we do not conclude from this result that B is better at continuity. |
| (b) B total cost (weighted) ≤ A | **Not met at the weights the project assumed** | Cache-weighted input is B 761,348 vs A 370,272 (2.06×). B's opus alone is 435,648, more than A. Output too is B 363,297 vs A 155,130. With Task 5's weights (s = 0.6, h = 0.2, output = input × 5), B = 816,108 + 173,375 + 294,553 = **1,284,036** and A = 1,145,922. B is about 1.12× A. The general condition is `288,958·s + 1,472,767·h ≤ 329,814`. If s ≥ h, for B ≤ A both s and h must be at most about 0.187 of opus. If h ≥ 0.224, B > A regardless of s. |
| (c) Revision residual inconsistencies B < A | **Not met (tie)** | Confirmed is B 1 (revision 1: 1 + revision 2: 0) vs A 1 (1 + 0), and including borderline B 1~4 vs A 1. It narrowed from run1's B 4 vs A 1 (the stale-directions problem in revision 1 went away after the replan and the same priority rule). But it is not B < A. The cost per revision is also higher for B (calls 33.5 vs 5, weighted 117,138 vs 73,660). |

**Conclusion:** In this sample, run2's B has fewer continuity errors than A (both raw figures and length-corrected). But the cause of the flip is A's wobble, and its size is similar to the A–B difference. So it is a result within n=1 noise. Revision residuals became equal to A's. Cost is about 2× A in weighted tokens, and about 1.12× A at the weights the project assumed. There is no basis to say today's Diegesis beats direct writing.

**Next plan candidate:** Consider lowering the haiku render effort to low. As a gate, first do a small pass-rate A/B. If haiku output falls to about 1,600 per call, B ≈ A at the assumed weights (B ≈ 1,130,896 vs A 1,145,922). At the same time, fix `modelOf` in `tools/serial` to count the haiku render as haiku (now it is grouped with sonnet, so `ByModel` and `-price` are wrong).

## Observations

- **stays-gone was kept.** B's Min-jae does not reappear after exiting in episode 4. In the same request, A brought back Min-jae, whom it had exited in episode 1, in episode 4 (A1, revision 1 residual).
- **B's revision-1 successor split into two.** In episode 4 Seo Tae-il, and from episode 5 Yun Yu-jin, take that role without introduction (verification B revision 1, episode 4). Placing the exit scene in episode 4 may also be an unreflected revision depending on how one reads the earlier episode's role-replacement request (2 borderline cases).
- **B's error types.** B1 (the remaining days do not drop from twelve) and B2 (hearing the same words "for the first time") come from skeleton prose that repeats the same fact in every episode. If the code has no progression of dates, the renderer rewrites "twelve days" as is. B3 is a case where a character the code declared appears in prose without introduction. Fact-level verification cannot see "does the reader know this character".
- **The haiku render has large output.** Render output tokens are 263,540 over 69 calls (3,819 per call). run1 sonnet was 123,853 over 76 calls (1,630 per call). Yet the body grew only 36% to 2,990 characters per episode over run1 (2,196). A good part of the output tokens does not remain in the body (presumed thinking and retry volume, but not confirmed).
- **Style is outside the judgment.** As in run1, B prose recites information straight into dialogue and repeats the same sentence in every scene (for example, "섬 전체를 리조트 개발사에 팔지 정하는 주민 투표가 열이틀 뒤에 열려" — "the residents' vote to decide whether to sell the whole island to the resort developer is held twelve days from now" — appears in episodes 1, 2, 4, 6, and 8).

## Limitations (run2)

- One item, one judgment. Seeing that A's confirmed errors doubled from run1's 4 to run2's 8, the variation of this one judgment is large.
- run1 was cut at episode 11 and run2 at episode 10. The run1 column in the table is on an episode-11 basis, so for direct comparison use the episode-10 point values above. Episode 12 (the vote, foreshadowing payoff) is again outside the judgment.
- The length instruction was the same, but B's body is 79% of A's (run1 was 54%). Episodes 4 and 8 are long, at 7,228 and 6,423 characters, because scenes grew during revisions, and the rest are 1,600–2,600 characters.
- 9/29 verification-failed scenes were shipped (under a policy of shipping only passes, B would drop scenes or need more calls).
- The run1 → run2 difference is the result of the render model, verifier, prompts, stays-gone, and harness changing together.

## Run record (run2)

- The harness, prompts, renderer, verifier, and rules were not fixed during the run. Branch `plan5`, HEAD `0e8fcb8`.
- **Attempt 0 (discarded):** To avoid the background-job time limit (about 2 hours), we launched the harness as a detached process. But the stdout and stderr redirect files were not created. The harness also streams dieg's stderr to its own stderr (`realDieg`'s `io.MultiWriter`), but that handle was broken, so right after B's episode 1 `cmd.Run` raised an error. The recorded calls are plan 1 + A 3 + B 5 = **9**, and there was no call in flight. It is an execution environment problem, not a harness bug. The outputs were deleted.
- **Main run (run2):** We launched again with the redirect set up via `cmd.exe`, and gave a budget of 220 − 9 = **211**. It started at 17:28 and ended at 19:01 (about 1 hour 33 minutes). The recorded calls are plan 3 + A 33 + B 155 + judgment 1 = **192**. Both attempts total **201 ≤ 220**.
- The harness's stdout and stderr are in `run2/serial.out` and `run2/serial.err`.
- We also committed 3 outputs over 500 lines (`run2/a/prompts/rev1-sum.txt`, `run2/b/novel.md`, `run2/judge/prompt1.txt`) as run data. `dieg.exe` is covered by `.gitignore` (`*.exe`).

Reproduction:

```
export PATH=<go-install>/bin:$PATH
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./tools/serial -idea examples/serial/idea.txt -out examples/serial/run2 -budget 211
```

---

# run3 — Plan 6 Task 6 (card verifier + render effort low)

We ran with the same item as run2 (`idea.txt`), the same revision requests (`revisions.txt`), and the same harness rules (alternating runs, budget reservation, cutting at the same episode, pre-revision replan, always judge). The price assumptions are also the same as run2 (s = 0.6, h = 0.2, output = input × 5). The outputs are in `run3/`.

**What changed from run2 to run3:**
- B's `dieg write/continue/revise` uses the **card verifier** (the dieg default, `--verifier card`). run2 had the fact-level verifier.
- B is given `--render-effort low` (`tools/serial/b.go` `withEffort`, fake-dieg test `TestBRenderEffortLow`). run2's haiku render had the default effort.
- The layered renderer was removed from the code (run2's B did not use it either).
- Everything else (the item, the episode count plan, the revision schedule after episodes 4 and 8, budget reservation, alternating runs, cutting at the same episode, the judge, the pre-revision replan) is unchanged. A's model and effort are also unchanged (opus/medium).
- Differing by chance: this time we wrote **all 12 episodes** within the budget (run2 was cut at episode 10). Both arms are at episode 12. So values compared directly with run2 are noted separately as being at the episode-10 point or on the same-episode basis. For the judgment, the blind mapping is the reverse of run2 (X = A, Y = B).

**n = 1.** One item, one run, one judgment. The differences below may be within noise.

## Results (episodes 1–12 + 2 revisions)

| Item | run2 A | run2 B | run3 A | run3 B | run3 B/A |
|---|---:|---:|---:|---:|---:|
| Episodes covered | 1~10 | 1~10 | 1~12 | 1~12 | |
| Cumulative continuity errors (confirmed after verification) | 8 | 3 | **8** | **9** | 1.13 |
| └ Including borderline | 8~9 | 3~4 | 8~9 | 9~11 | |
| └ Per 10,000 characters (confirmed / including borderline) | 2.12 / 2.39 | 1.00 / 1.34 | **1.71** / 1.92 | **2.32** / 2.84 | 1.36 |
| Body length (characters) | 37,712 | 29,896 | 46,858 | 38,733 | 0.83 |
| Calls | 31 | 155 | 41 | 101 | 2.46 |
| Cache-weighted input (sum) | 370,272 | 761,348 | 563,611 | 792,320 | 1.41 |
| └ opus | 370,272 (31 calls) | 435,648 (17 calls) | 563,611 (41 calls) | 562,959 (20 calls) | |
| └ haiku render | — | 155,067 (69 calls) | — | 95,910 (37 calls) | |
| └ sonnet extraction | — | 170,633 (69 calls) | — | 133,451 (44 calls) | |
| Output tokens (sum) | 155,130 | 363,297 | 228,143 | 265,747 | 1.16 |
| └ opus / render / extraction | 155,130 / — / — | 76,092 / 263,540 / 23,665 | 228,143 / — / — | 124,618 / 120,333 / 20,796 | |
| Revision 1: calls / weighted / residual (confirmed) | 6 / 90,314 / 1 | 32 / 117,614 / 1 | 6 / 100,469 / **0** | 21 / 102,327 / **1** (1~2) | |
| Revision 2: calls / weighted / residual (confirmed) | 4 / 57,006 / 0 | 35 / 116,661 / 0 | 10 / 189,457 / **1** | 10 / 64,053 / **1** | |

- Per-model values were counted from the `run3/b/calls.jsonl` labels (`modelOf` already maps `render-` → haiku this time). The sums equal the per-model values in the harness `summary.json` `B_total_including_cut_work`.
- Outside the table: the shared `plan` 3 times (weighted 9,491), the judgment once. Both arms were cut at episode 12, so there is no after-cut work.
- Errors and residuals are values where a person cross-checked the judge's marks (the judge's original marks: errors A 9, B 11; residual A 0, 1; B 2, 1). The record is [`run3/judge/verification.md`](run3/judge/verification.md), and the same symmetric rule as run1 and run2 was used.
- The run3 cumulative at the same episode-10 point as run2 (`run3/metrics.csv` ep10 row): A calls 37, weighted 517,503, output 206,941. B calls 91, weighted 701,495, output 222,525. run2's episode-10 B is calls 155, weighted 761,348, output 363,297. At the same point, B is calls −41%, weighted input −8%, output −39%. However A at the same point also has weighted input +40% and output +33% (A's per-episode volume grew over run2). It means A is not a same-condition baseline.

## Effect of the card verifier (B scene level)

Counted from the `scores` field of `run3/b/novel/*.report.json` (31 scenes) and the call labels. run2's reports have no `scores`.

| Item | run2 B | run3 B |
|---|---:|---:|
| Mean per-scene Score | (none) | **0.9976** (min 0.968, 23/31 scenes at 1.0) |
| Scene pass rate (0 hallucinations, 0 leaks, missing ≤ 1, shipped final) | 20 / 29 (69%) | **31 / 31 (100%)** |
| Render attempt pass rate (`pass=` in `b.log`) | 24 / 34 (71%) | **37 / 37 (100%)** |
| Re-render count (calls of `render-sN-a2` or above for the same scene) | **35** | **0** |
| Total render calls | 69 | 37 (including 6 scenes rewritten by revisions) |

- **Note: the two verifiers are not the same test.** The run2 → run3 change in pass rate and re-render count is in large part the result of the verifier definition changing, not evidence that the prose got better. The card verifier cannot fail a scene for hallucination (`Hallucinated` in `internal/render/card.go` is always empty and G2 is only a warning). So run3's "0 hallucinations, 0 leaks, missing ≤ 1" is a value that looked only at omissions, fact kinds, and leaks, while run2's same phrase also looked at hallucinations. The card verifier's own validation (mutation detection, `examples/corpus-ab` scorecard) also covers only omissions, fact kinds, and leaks. A 100% pass rate and 0 re-renders must not be read as prose quality or a hallucination reduction.
- run2's re-render count is the count of the label `render-sN-aK` (K ≥ 2) in `run2/b/calls.jsonl`. run3's 37 render calls are all `-a1`, and are the 31 scenes plus 6 (s2–s5 etc. rewritten at revisions). A revision rewrite restarts from `a1`, so it is not a re-render by this definition. run2's 35 is an exact value, not a cap. (Of the 35, 18 happened inside the revise stage and 17 inside the episode stage.)
- **The card grading calls are 44 for the 37 renders.** It is the value with 7 stage-2 re-asks added (s9, s12, s14, s18, s19, s23, s24) (among the `extract-sN-a1` labels in `run3/b/calls.jsonl`, those that appear twice for scenes not rewritten by revisions). s2–s7, rewritten by revisions, have a render and a grading twice per scene, separate from the 7 above.
- Output per render call at haiku effort low is 3,252 tokens (120,333 over 37 calls). run2 was 3,819 tokens (263,540 over 69 calls). Output per call fell only 15%. The 55% drop in total render output is almost entirely from the lower call count. It fell short of the about 1,600 per call that run2's "next plan candidate" expected.
- **`scores.Length` is 0 for all 31 scenes.** The mean Score of 0.9976 is nearly perfect because Coverage 1.0, Kind 0.992, and Tone 0.994 are nearly perfect. We did not confirm whether this Score measures the quality a person reads. The 100% pass rate also did not lead to a reduction in continuity errors (verdict (a) below).
- Even when verification passed, continuity errors between scenes were not filtered. B's 9 confirmed errors are IR 3 / prose 5 / pipeline 1 / judge 0. The 5 prose ones are drift of props, places, and knowledge across scenes (for example, using the box opened in episode 8 as an unknown box in episode 11), which per-scene checks cannot see. The 1 pipeline one (B11, the unpaid vote) is not a story failure but a pipeline cutoff (see "The portion of B that was judged" below).

## Price condition formula (the same assumptions as run2)

```
A = 563,611 + 5 × 228,143                                   = 1,704,326
B = (562,959 + 5 × 124,618)                    opus share     = 1,186,049
  + s × (133,451 + 5 × 20,796)                 sonnet extract = s × 237,431
  + h × (95,910 + 5 × 120,333)                 haiku render   = h × 697,575
B ≤ A  ⇔  237,431·s + 697,575·h ≤ 518,277
```

- At s = 0.6, h = 0.2, B = 1,186,049 + 142,459 + 139,515 = **1,468,023**, **0.86×** A. run2 (episode 10, the same assumptions) was 1.12×.
- **B did not render s32–s35** (see "The portion of B that was judged" below). Rendering and grading the 4 scenes with run3's call averages (render 37 calls 18,853, grading 44 calls 5,396; weighted input + output × 5) gives 0.2 × 18,853 × 4 + 0.6 × 5,396 × 4 × (44/37) ≈ **+30.5k (about 31k)**. B ≈ 1,498,500, about **0.88×** A. The verdict (met) is unchanged. It is an estimate.
- If extraction is free (s = 0), h ≤ 0.74; if s = h, both must be 0.55 or lower for B ≤ A.
- B's opus share alone is 70% of A. A substantial part of why the ratio went down is that A's cost grew to 12 episodes' worth (1,145,922 → 1,704,326, +49%), and B's absolute cost rose only from 1,284,036 to 1,468,023 (+14%). But since the episode counts differ, this absolute comparison is rough.

## Success criteria verdict (run3)

| Criterion | Verdict | Basis |
|---|---|---|
| (a) B cumulative errors < A | **Not met** (regardless of whether B11 is included) | With B11: confirmed B 9 > A 8, including borderline B 9~11 > A 8~9, per 10,000 characters B 2.32 > A 1.71 (including borderline 2.84 > 1.92). **Without B11 (story errors 8 + pipeline cutoff 1):** confirmed B 8 = A 8 (a tie, not B < A), per 10,000 characters B 2.07 > A 1.71 (including borderline B 8~10 → 2.58 > 1.92). In run2 it was B 3 < A 8. A stayed at 8 and B rose from 3 to 9. B's body grew to 12 episodes (29,896 → 38,733 characters), and same-scene repetition/reversion errors arose in the late episodes (9–11). Because n=1, we assert neither run2's B < A nor this time's B > A. |
| (b) B total cost (weighted) ≤ A | **Met at the assumed weights (not met by the cache-weighted input sum)** | Weighted input is B 792,320 vs A 563,611 (1.41×), output B 265,747 vs A 228,143 (1.16×). With s = 0.6, h = 0.2, output = input × 5, B 1,468,023 ≤ A 1,704,326 (0.86×; adding about +31k for the s32–s35 renders, about 0.88×). The general condition is `237,431·s + 697,575·h ≤ 518,277`. The unit prices are assumptions and not an actual price list. |
| (c) Revision residual inconsistencies B < A | **Not met** | Confirmed B 2 (1 in revision 1, 1 in revision 2) vs A 1 (1 in revision 2). Including borderline, B is 2~3. Calls per revision are also higher for B (15.5 vs 8). |

**Conclusion:** In this sample, the card verifier + effort low removed B's re-renders from 35 to 0 and raised the scene verification pass rate from 69% to 100%, and cut calls and output tokens substantially. But continuity errors between scenes did not drop (B 9 > A 8, if anything). Cost came to 0.86× A at the assumed weights (about 0.88× adding the unrendered s32–s35), but A's volume grew with it, and the weights are assumptions. There is still no basis to say today's Diegesis beats direct writing.

## Limitations (run3)

- n=1: one item, one run, one judgment. As with A's confirmed errors doubling from run1's 4 to run2's 8, the variation of a single judgment is large. This time A stayed at 8.
- run2 and run3 differ in episode count (10 vs 12). A's volume also differed (37,712 → 46,858 characters). A is not under the same conditions either. The changes in errors and cost relative to run2 cannot be read as the effect of the card verifier and effort alone.
- The judge's blind mapping changed (X = A, Y = B). Some of the judge's findings I downgraded to borderline (A 1, B 2). The confirmation is one person's cross-check.
- **B was judged on less story than A.** The B "episode 12" the judge saw is the code's episode 11 (s30, s31). The `--scenes 2` cap the harness gave `dieg continue` (`tools/serial/b.go`) blocked rendering to 2 scenes per stage, so backlog scenes accumulated at each stage (the last `backlog=3` in `b.log`), and s32–s35, which were coded, were not rendered. s35, which holds the vote and `payoff show_at_vote`, is among them. B11 ("the vote is not closed in episode 12") is therefore not a story failure but a pipeline cutoff. B's story errors are confirmed 8 + pipeline 1, and (a) is not met under either way of counting. run2 too had s28 and s29 not rendered with the same cap (backlog 2). `b_missing_scenes: {}` in `summary.json` masked this (it counts only scenes per episode). We added `b_coded_unrendered_scenes` to the harness, and appended to `run3/summary.json`, with no calls, the value computed from the files, `["s32","s33","s34","s35"]`. The `--scenes` cap was not changed and we did not re-run.
- Because `scores.Length` is all 0, we cannot know whether Score reflects length.

## Run record (run3)

- Branch `plan6`. Starting HEAD `c6fd4b7`; one harness change commit (`tools/serial/b.go` `withEffort`, test `TestBRenderEffortLow`) went in before the run.
- With the noclaude shim, `go vet ./...` and `go test -count=1 ./...` passed. With `-dry` we confirmed `--render-effort low` is attached to all of B's dieg command lines. The actual calls in this step are 0.
- The main run was one process (`cmd.exe` redirect, `-budget 220`). Recorded calls are plan 3 + A 41 + B 101 + judgment 1 = **146 ≤ 220**, and there were no discarded attempts. `stopped_by` is empty.
- We also committed 3 outputs over 500 lines (`run3/a/prompts/rev2-sum.txt`, `run3/b/novel.md`, `run3/judge/prompt1.txt`) as run data.

Reproduction:

```
export PATH=<go-install>/bin:$PATH
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./tools/serial -idea examples/serial/idea.txt -out examples/serial/run3 -budget 220
```
