# Measurement: AI writes directly (A) vs `dieg write` (B)

We compared two methods with the same item, the same model (`claude -p`, claude-opus-5-5, effort high, subscription CLI), and the same target scene count.

- **A (direct)**: the same method as the demo (`examples/idea-to-novel`). The AI writes an outline, then writes the scenes one at a time. For each scene it sees only the outline and the previous scene.
  The prompts are the demo's `a1-outline.txt` and `a-scenes.sh`, ported with only the scene count changed (`tools/bench/main.go`).
- **B (Diegesis)**: `dieg write <idea> -o b --rounds 3`. Write code → `check`/`lint` diagnostics fed back → verified render per scene (1 render + 1 re-extraction, at most 3 attempts).
- The target scene count (`장면 7개.`, "7 scenes.") is written at the end of the item file. B takes the item as is; A gets the same item plus the same count in its requirements.

## Results

Errors = the number of issues raised by one blind judgment (X/Y randomized) that I then verified against the source text with quotations. Tokens = the sum over `calls.jsonl`.
The claude CLI puts most of the prompt in the cache fields, so **total input = input + cache_read + cache_write**.

| Item | Scenes | Errors A / B | Total input A / B | output A / B | Calls A / B | Minutes A / B | B rounds | B scenes failing verification |
|---|---|---|---|---|---|---|---|---|
| lighthouse | 7 | **3 / 1** | 39,478 / 137,862 (3.5×) | 44,082 / 96,892 (2.2×) | 8 / 34 | 9.6 / 20.3 | 2 (clean) | 4/7 (s3 s4 s5 s6) |
| will | 6 | **2 / 2** | 33,280 / 116,526 (3.5×) | 37,364 / 82,464 (2.2×) | 7 / 28 | 8.1 / 18.1 | 2 (clean) | 2/6 (s3 s5) |
| mine (the long one) | 20 | not judged | 130,547 / 92,900* | 107,325 / 94,767* | 21 / 20* | 24.1 / 17.8* | 2 (clean) | 1/5 (s3) |
| ferris | 8 | not run (budget) | | | | | | |

\* mine's B is the code for all 20 scenes + **only the first 5 scenes rendered** (see "What was cut" below). It is not the value for rendering all 20 scenes.

The four raw token fields (summed per arm):

| Item | Arm | input | cache_read | cache_write | output |
|---|---|---|---|---|---|
| lighthouse | A | 16 | 0 | 39,462 | 44,082 |
| lighthouse | B | 68 | 32,608 | 105,186 | 96,892 |
| will | A | 14 | 0 | 33,266 | 37,364 |
| will | B | 56 | 26,494 | 89,976 | 82,464 |
| mine | A | 42 | 0 | 130,505 | 107,325 |
| mine | B* | 40 | 18,342 | 74,518 | 94,767 |
| judge lighthouse | | 2 | 0 | 34,453 | 14,032 |
| judge will | | 2 | 0 | 30,629 | 12,363 |

Prose length (characters excluding whitespace, UTF-8 characters): lighthouse A 14,038 / B 7,857, will A 12,170 / B 7,386. A is 1.6–1.8× longer (close to the requested "around 2500 characters", about 2.0k per scene).
Errors per 10,000 characters: lighthouse A 2.14 / B 1.27, will A 1.64 / B 2.71.

**Summary.** On the two short items, B was **more expensive** every time (total input tokens 3.5×, cache-weighted input about 2.7×, output 2.2×, calls 4×, time 2×). It won once on errors (3:1) and tied once (2:2).

"Total input" is a token quantity, not a cost. Weighting cache reads at 0.1× and cache writes at 1.25× (input + 0.1·cache_read + 1.25·cache_write) gives lighthouse A 49,344 / B 134,811 (2.7×), will A 41,597 / B 115,175 (2.8×), mine A 163,173 / B* 95,022.
B's code was clean after 2 rounds both times, but in render verification 33–57% of scenes failed after 3 attempts. For failed scenes, the last render still went into the novel.

## Long item (mine, 20 scenes): input tokens per scene

| Scene | A input | A output | B render input | B extract input | B output |
|---|---|---|---|---|---|
| outline / code | 1,359 | 5,025 | code-r1 14,131 · fix-r2 27,111 | | 51,889 + 13,749 |
| s1 | 3,535 | 5,017 | 4,869 (2 attempts) | 5,594 | 5,727 |
| s2 | 6,522 | 4,536 | 2,789 | 3,038 | 3,261 |
| s3 | 6,633 | 4,926 | 8,451 (3 attempts, failed) | 7,762 | 9,122 |
| s4 | 6,447 | 4,387 | 5,916 (2 attempts) | 6,916 | 6,625 |
| s5 | 6,618 | 3,944 | 3,036 | 3,287 | 4,394 |
| s6–s20 | 5,724–6,918 (mean 6,629) | 3,9xx–7,684 | not rendered | | |

How to read it:

- **A's context does not accumulate.** The demo's A method gives each scene only "outline + previous scene". So per-scene input is flat at 6.4–6.9k for s2–s19 (s20 is 5.7k).
  The hypothesis that A gets expensive on long works through context accumulation is **not tested by this A design.** A's total cost is proportional to the scene count.
  (We did not run an A that puts all scenes in context. Its input would grow with the square of the scene count, but that is outside this measurement.)
- **B's per-scene input is also flat** (1 render 2.8–4.9k + re-extraction 3.0–5.6k). Retries make it 2–3×, though.
- **B's fixed cost is the code stage.** Two code-writing calls for 20 scenes take input 41,242 and output 65,638. That is 61% of A's total output (107,325) for the same item.
  The output is 1.5–2× that of the code stage on the short items (lighthouse 35,441 / 42,265, will 33,526 / 30,752).
- **If B had rendered all 20 scenes (extrapolation, not measured):** multiplying the mean of the 5 rendered scenes (per scene input 10.3k, output 5.8k, 3.6 calls) by 20 and adding the code stage gives
  total input ≈ 248k, output ≈ 182k, calls ≈ 74. That is 1.9× / 1.7× / 3.5× A (131k / 107k / 21 calls). B does not get cheaper as scenes grow. This extrapolation probably puts B too low, because the verification failure rate of the 5 rendered scenes (1/5) was lower than on the short items (4/7, 2/6).

## Error quotations

The full list and verdicts are in `*/judge/verification.md`, the judge's raw output in `*/judge/run1.txt`, and the quotation cross-check in `*/judge/quotes.md`.

1. **A: the outline's hidden truth leaks into scenes (lighthouse A3, knowledge).** In s4, Eun-sol rubs the impression on the torn page, but
   "끝자리는 삼촌의 볼펜 글씨에 가려 보이지 않았다" ("the last digits were hidden under her uncle's ballpoint writing"), and "'차단기'가 나왔다. 그 뒤는 뭉개져 있었다." ("'Breaker' came out. What followed was smudged.").
   In s7 she holds "그저께 밤 당직실에서 연필을 눕혀 문지른 종이" ("the paper rubbed with a pencil laid flat in the duty room the night before last") and reads all of "20:15. 회전 정지, 차단기 내려짐. 도철? 선착장 확인." ("20:15. Rotation stopped, breaker down. Do-cheol? Check the pier.").
   It is the outline timetable's sentence verbatim. A2 of the same item has the same shape: the "words he repeated for three days" of the uncle turn, on the cliff path, into the place of the truth (the breakwater).
   B has a structure that blocks this kind of error. Lies are checked with `lies` + `false` facts, and knowledge with `info-flow`; this B prose had no error of this kind.
2. **B: an error present in the code but outside the checks (will B3, knowledge).** Jeong-suk has never seen or heard of the inner envelope, yet in s4 she says
   "속봉투를 가지고 마루로 나와라. 거기서 말하마." ("Bring the inner envelope out to the hall. I'll tell you there."). Exactly as in the code:
   ```
   jeongsuk says daon: "속봉투를 가지고 마루로 나와라. 거기서 말하마."
   ```
   It is a quotation with no fact attached, so `info-flow` cannot see it. The inner envelope is known only to Da-on and Su-a in s3 (`setup inner_envelope promise: open_together`).
   The code had 0 check errors and 0 lint warnings.
3. **B: a time invented by the renderer (will B1, timeline).** In the code, s5 is `when: after s4 by 10m..30m` and s4 is evening.
   The rendered prose of s5 says "안쪽은 새벽의 서늘한 기운이 그대로 남아 있었다" ("inside, the cool air of dawn still lingered") and also "내일 재에서, 할머니께 같이 인사드려요." ("Tomorrow at the memorial, let's greet Grandmother together."),
   and s6 refers to that conversation as "어젯밤에 고모랑 무슨 얘기 했어?" ("What did you talk about with Aunt last night?"). lighthouse B1 (climbing the stairs with the logbook that should be upstairs "tucked under her arm")
   is likewise a fact the renderer invented outside the code. Of B's 3 confirmed errors, 2 came from the renderer and 1 from the code.
   (will B2, the date being at the "top line" vs at the "end of the letter", is a borderline case of the same degree as the A3 we dropped, so it was dropped for symmetry.)

## Scenes that failed verification (B, verbatim)

Only the failed scenes are copied from `*/b.log` (the stderr of dieg write). All made 3 attempts and the failed last render went into the novel.

```
lighthouse s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:eunsol|perceive|flashlight_left 은솔은 날이 새면 등롱실에 올라가 아버지의 일지를 확인하기로 결심한다.] leaked=[]
lighthouse s4: attempts=3 pass=false missing=[] hallucinated=[Eunsol hides her father's log inside the front of her jacket. extra-act:eunsol|perceive|motor_works extra-act:eunsol|read|last_page] leaked=[]
lighthouse s5: attempts=3 pass=false missing=[] hallucinated=[extra-act:eunsol|perceive|boxes_hidden extra-act:jeongseok|assert|smuggling extra-act:mikyung|perceive|boxes_hidden] leaked=[]
lighthouse s6: attempts=3 pass=false missing=[] hallucinated=[extra-act:mikyung|perceive|radio_works] leaked=[]
will s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:daon|read|jeongsuk_absent extra-act:sua|read|jeongsuk_absent 다온이 셋째 항아리 바닥에서 아무도 손대지 않은 봉해진 속봉투를 찾아낸다.] leaked=[]
will s5: attempts=3 pass=false missing=[] hallucinated=[extra-act:daon|assert|raised_by_jeongho extra-act:daon|assert|raised_by_jeongho extra-act:daon|perceive|daon_birth extra-act:jeongsuk|act|open_together extra-act:jeongsuk|assert|daon_birth] leaked=[]
mine s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:chulsu|assert|four_trapped extra-act:chulsu|perceive|cavein extra-act:dongsu|perceive|four_trapped 오만길의 다리가 무너진 돌에 깔렸다.] leaked=[]
```

(Glosses of the Korean fragments: lighthouse s3 "Eun-sol decides to go up to the lantern room at daybreak and check her father's logbook."; will s3 "Da-on finds the sealed inner envelope, untouched by anyone, at the bottom of the third jar."; mine s3 "Oh Man-gil's leg was pinned under the collapsed stone.")

All failures are hallucinated (re-extraction found acts or facts not in the code). leaked is 0. The lighthouse s3 finding ("go up to the lantern room ... and check the logbook") has the same root as the B1 the judge found. The verifier marked that scene as failed but could not fix the render.
lighthouse s1, s2, s7, will s1, s2, s4, s6, and mine s1, s2, s4, s5 passed (mine s2 had 1 missing, within the criterion).

## Limitations

- **3 items, 2 judged, one run, one model.** These are cases, not statistics. The error differences (3:1, 2:2) are within the recall of a single judgment.
- **The judgment is a single run.** The demo used a 3-vote majority. Errors the judge missed were not counted. Errors I found separately are not included either.
  Of the 7 findings against A, 2 (lighthouse A4, will A3) were dropped because I was not sure. Of the 4 findings against B, 1 (will B2) was dropped by the same criterion. Both judgments drew X=B, so the position (X/Y) did not change.
- **The lengths differ.** A received "around 2500 characters" per scene but wrote about 2.0k characters excluding whitespace per scene, which is close to the request. B does not fix the prose length. A is 1.6–1.8× longer.
  Longer text has more places for errors and more tokens. Per-10,000-character figures are given alongside.
- **The two arms made different stories.** The difference in story complexity is mixed into the error counts.
- **A's context does not accumulate** (above). "A gets more expensive the longer the work" cannot be seen with this design.
- **Revision requests were not run.** The claim "strong at revision" is not covered by this measurement (the demo's revision experiment is the only evidence, and B did not win there either).
- **For the long item only tokens were measured, and B rendered only 5/20 scenes.** B's 20-scene cost is an extrapolation.
- **B's error source.** 2 of the 3 confirmed errors were facts the renderer invented outside the code. Verification render flags them (failure) but cannot fix them, and failed renders still go into the novel.
- Output tokens include the model's thinking tokens (the CLI does not split them out). This is why one A scene (~2.0k characters of prose) is 4–7k output tokens.

## Verdict per claim

| Claim | This result |
|---|---|
| Diegesis is more consistent | **Mixed.** Won once at 3:1 and tied once at 2:2. Adjusted per 10,000 characters it is B 1.27 vs A 2.14, and B 2.71 vs A 1.64. A's errors were contradictions within the story itself (attributes, leaked truth), while B's errors mostly came from the renderer. B's code converged to 0 diagnostics both times, but knowledge inside dialogue with no fact attached (will B3) is outside the checks. This data cannot support "more consistent". |
| Diegesis is cheaper | **Not supported.** On the short items B was 3.5× in total input, 2.2× in output, and 4× in calls. The causes are code writing (14k tokens of reference material + 25–52k tokens of code output) and retries in verification render (a failed scene is 3 renders + 3 re-extractions). |
| Diegesis gets better the longer the work | **Not supported (cost).** A's per-scene input is flat (6.6k), and B's is flat per scene too. B's code stage grows with the scene count (20-scene code output 65.6k). The extrapolated 20-scene B is 1.9× (input) / 1.7× (output) of A. Consistency was not measured on the long item. |
| Strong at revision | **Not measured this time.** |

## What was cut (budget ≤ 150 calls)

- Budget estimate: for one short item, A 8 + B 30–45 + judge 1. The first item's B took 34 calls.
- **Cut 1 (long item):** B was run with `--scenes 5`. It wrote the code for all 20 scenes (rounds and tokens recorded) and rendered only the first 5. A wrote all 20 scenes. No judgment was made (tokens only).
- **Cut 2:** the third short item `ferris` (8 scenes) was not run. The remaining 30 calls cannot hold A 9 + B ≈30 + judge 1. The item file is kept at `ideas/ferris.txt`.
- B's `--rounds` was reduced to 3 (default 4). All three items were clean at round 2, so this does not affect the results.
- Actual calls: **120** (lighthouse A 8 · B 34 · judge 1, will A 7 · B 28 · judge 1, mine A 21 · B 20). There were no separate trial calls. Failed calls: 0.

## Files

- `ideas/*.txt` — items (no source work). The last line is the target scene count.
- `<item>/a/` — `outline.md`, `scenes/s<i>.md`, `prompts/` (the prompt sent on each call), `calls.jsonl`
- `<item>/b/` — `dieg write` output as is: `story/` (final code), `rounds/` (raw output and code per round), `diags.txt`, `novel/` (scene prose + verification reports `*.report.json`), `novel.md`, `calls.jsonl`
- `<item>/b.log` — stdout+stderr of `dieg write` (summary lines, per-scene verification results)
- `<item>/judge/` — `mapping.txt` (X/Y assignment), `run1.txt` (judge's raw output), `quotes.md` (quotation cross-check), `verification.md` (my verification), `calls.jsonl`
  - The judge prompt `prompt.txt` is over 500 lines and was not committed. It can be rebuilt from the checklist (`judgeHead` in `tools/bench/judge.go`, the same text as the demo's `judge-build.sh`) + the two novels, and its sha256 is kept in `SHA256SUMS-uncommitted.txt`.
- Artifacts were not touched. The compiler, rules, renderer, and prompts were not changed during measurement either.

## Re-running

```sh
export PATH=<go-install>/bin:$PATH SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go test ./tools/bench                       # TestBenchDryRun: run one item to the end with a fake Completer
go run ./tools/bench a -idea examples/bench/ideas/lighthouse.txt -n 7 -out examples/bench/lighthouse
go run ./tools/bench b -idea examples/bench/ideas/lighthouse.txt -rounds 3 -out examples/bench/lighthouse
go run ./tools/bench judge -n 7 -out examples/bench/lighthouse
go run ./tools/bench b -idea examples/bench/ideas/mine.txt -rounds 3 -scenes 5 -out examples/bench/mine
go run ./tools/bench sum examples/bench/lighthouse examples/bench/will examples/bench/mine
```

A and the judgment do not re-ask for files that already exist (resumable). B runs from scratch every time.
