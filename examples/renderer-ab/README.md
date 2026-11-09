# Renderer and model A/B (Plan 5, Task 5)

We compared the single renderer (`render.RenderVerified2`) with the layered renderer (`narrate.RenderLayered`), and compared sonnet with haiku as the render model. The goal is to choose the default renderer and the default render model. In particular we looked at whether rendering with haiku is acceptable, since it hits usage limits less and costs fewer tokens.

- The harness is `tools/renderer-ab` (`go run ./tools/renderer-ab -out examples/renderer-ab`). Every call went through `render.Meter` and was logged to `<arm>/calls.jsonl`.
- Artifacts are the prose `<arm>/<work>_<scene>.<backend>.md`, the per-run record `runs.json`, the aggregates `summary.json` and `table.md`, and the progress log `run.log`.
- The render-role effort is medium for both sonnet and haiku. haiku accepts `--effort` (checked once beforehand). Re-extraction was fixed at `claude-sonnet-5-5`/low in all arms.
- The backend is ko-haera. Only the style swap was measured with ko-hapsho.
- Prompts and the verifier were not changed during the experiment.

## Actual call count

**118 calls** (budget ≤ 120). The harness made 117, and the haiku `--effort` pre-check made 1. The line count of `calls.jsonl` sums to 117. There were 0 call errors.

## Scenes and budget adjustment (differences from the brief)

The brief set the 4 scenes below. The layered beat count is the scene's beats as used by `PlanScene`.

| Scene | Acts | Beats |
|---|---|---|
| macbeth/1.3 | 26 | 11 |
| chibi/45.88 | 24 | 10 |
| haejin/s3 (serial, ep02) | 12 | 4 |
| haejin/s19 (serial, ep07) | 12 | 4 |

The brief budgeted layered at about 9 calls per scene (3 beats). But the corpus scenes have 10–11 beats, so one layered run costs about 25 calls even with no retries. Running the three arms used for the verdict (S-son, L-son, L-hai) on all 4 scenes would take at least about 150 calls, which does not fit the budget.

So we changed the run order to be per scene.

- Scenes were ordered by fewest beats, and for each scene the three arms were run together.
- A scene was started only if the three arms' estimates fit in the remaining budget. So the scenes in the verdict are ones where all three arms ran the same scene.
- As a result, **only the 2 serial scenes (s3, s19)** entered the verdict. chibi and macbeth were skipped because at the 85-call mark they needed an estimated 82 and 88 calls respectively.
- With the remaining budget we made reference runs outside the verdict: S-son and S-hai on chibi, and S-son on macbeth; macbeth S-hai was skipped because of the budget.

## Results (2 verdict scenes, ko-haera)

| Arm | n | Pass | Mean calls | sonnet tokens in/out | haiku tokens in/out | Weighted tokens/scene | Register | Chars (target 2500) | Beat retries | Hallucinated new_event/fact form |
|---|---|---|---|---|---|---|---|---|---|---|
| S-son (single+sonnet) | 2 | 1/2 (50%) | 6.0 | 36252/12725 | 0/0 | 149816 | 1.00 | 690 | – | 2/1 |
| L-son (layered+sonnet) | 2 | 0/2 (0%) | 18.0 | 74205/27413 | 0/0 | 316905 | 0.99 | 1444 | 6 | 3/1 |
| L-hai (layered+haiku) | 2 | 1/2 (50%) | 17.0 | 41697/4074 | 30409/50226 | 233870 | 1.00 | 1469 | 5 | 1/0 |
| S-hai (single+haiku) | 2 | 1/2 (50%) | 4.0 | 12428/1842 | 13069/16526 | 80306 | 1.00 | 1048 | – | 2/0 |

- Token in is input+cache_read+cache_write. The sonnet tokens include the re-extraction (sonnet) of all arms.
- Weighted tokens are computed with relative weights sonnet 3:15, haiku 1:5 (in:out). These values are assumptions and can be changed with `-weights`.
- Chars are characters excluding whitespace, averaged over the 2 scenes.

Per-run results:

| Arm | Scene | Pass | Calls | Attempts per beat | Final adopted | Failure reason |
|---|---|---|---|---|---|---|
| S-son | s3 | ✗ | 6 (3 renders) | – | – | missing 1 + fact form 1 (`seoyun`'s song_line is perceive instead of assert) + new_event 2 |
| S-son | s19 | ✓ | 6 (3 renders) | – | – | – |
| L-son | s3 | ✗ | 20 | 1,3,3,1 | unstitched | fact form 1 (`extra-act:seoyun\|assert\|memorial_day`) + new_event 1 |
| L-son | s19 | ✗ | 16 | 1,1,1,3 (beat 4 failed) | stitched | new_event 2 |
| L-hai | s3 | ✓ | 19 | 2,2,3,1 | stitched | – |
| L-hai | s19 | ✗ | 15 | 1,2,1,1 | unstitched | new_event 1 |
| S-hai | s3 | ✓ | 2 (1 render) | – | – | – |
| S-hai | s19 | ✗ | 6 (3 renders) | – | – | new_event 2 |

Results of the reference runs (outside the verdict, single renderer):

| Arm | Scene | Pass | Calls | Chars | Failure reason |
|---|---|---|---|---|---|
| S-son | chibi/45.88 | ✗ | 6 | 743 | new_event 3 (flees, steals the letter, feigns drunkenness) |
| S-hai | chibi/45.88 | ✗ | 6 | 1111 | missing 2 + new_event 2 |
| S-son | macbeth/1.3 | ✗ | 6 | 1380 | missing 2 (`holds(king,macbeth)` ×2) + new_event 2 |

## Beat retries

- Beat retries were 6 for L-son (in 3 of 8 beats) and 5 for L-hai (in 4 of 8 beats).
- The only beat that exceeded the cap (BeatRetries 2) is beat 4 of L-son s19. The scene was assembled anyway.
- The style layer passed all 4 times (StyleOK).
- The stitched version was adopted in 2/4. In the other 2, the stitched version did worse in the comparison, so we went back to the pre-stitch text.

## new_event breakdown (separating extractor noise from layering side effects)

The 11 hallucinations in the verdict scenes are 8 new_event and 3 fact-form. Matching the 8 new_event items against the source IR, **all of them restated acts that are already in the IR**.

| Arm | new_event | The corresponding act in the IR | Type |
|---|---|---|---|
| S-son s3 | Seo-yun reaches for the sewing box and withdraws her hand | `seoyun does reach sewing_box` | fact-less does |
| S-son s3 | The three leave Mal-sun's house | `exit all` | entrance/exit |
| L-son s3 | Seo-yun touches the sewing box lid and is stopped | `seoyun does reach sewing_box` | fact-less does |
| L-son s19 | Ha-eun hides the ticket deep in her coat | `haeun does hide ferry_ticket` | fact-less does |
| L-son s19 | Seo-yun closes the notebook and leaves, and Ha-eun follows | `exit seoyun`, `exit haeun` | entrance/exit |
| L-hai s19 | Ha-eun hides the ticket deep in a drawer | `haeun does hide ferry_ticket` | fact-less does |
| S-hai s19 | Ha-eun and Seo-yun leave the classroom together | `exit …` | entrance/exit |
| S-hai s19 | Ha-eun hides the ticket deep in her inner pocket | `haeun does hide ferry_ticket` | fact-less does |

- By arm: S-son has 1 fact-less does and 1 entrance/exit, L-son 2 and 1, L-hai 1 and 0, S-hai 1 and 1.
- Fact-form: S-son 1, L-son 1, L-hai 0, S-hai 0.
- **All new_events in the verdict scenes are noise from the extractor/comparison side.** `render.Diff` removes fact-less acts and presence acts from the expected list, but the re-extractor reports those acts as new_events. Of these 8, none is a side effect that appeared only in layered. In s19, "hides the ticket" appeared identically in three arms: L-son, L-hai, and S-hai.
- The new_events of the reference scenes (chibi, macbeth) (flees, steals the letter, the witches vanish, Ross and Angus arrive) appear to have the same shape (entrances/exits or fact-less does). However, we did not match them one by one against the IR.
- Counterfactual count (not used in the verdict): if new_events that restate IR acts are removed from the hallucinations, the pass rates are S-son 1/2, L-son 1/2, L-hai 2/2, S-hai 2/2.
- Fixing the verifier is outside this task's scope, so we left it as is.

## Style swap cost (L-hai, haejin/s3 → ko-hapsho)

We kept the L-hai beat prose for s3 (the [1] and [2] results) and re-ran only [3] style, [4] stitching, and the final comparison with ko-hapsho. The harness uses entry points split into `narrate.Beats` + `narrate.Finish`.

| Method | Calls | Render model in/out | Re-extraction (sonnet) in/out | Weighted tokens | Pass | Register |
|---|---|---|---|---|---|---|
| Swap only ([3], [4] + final) | **3** (haiku 2 + sonnet 1) | haiku 7044/9211 | 3997/436 | 71630 | ✓ | 1.00 (hapsho register) |
| Same scene, full L-hai (haera) | 19 | haiku 17256/28925 | 20148/1950 | 251575 | ✓ | 1.00 |
| Same scene, full S-son (hapsho) | 6 (3 renders) | sonnet 10894/5565 | 8437/1607 | 165573 | ✓ | 1.00 |

Changing the style in layered costs about 1/6 of the calls of a full render (3 vs 19) and about 28% of the weighted tokens. Compared with rendering the same scene as single in hapsho, calls are half and weighted tokens about 43%. The figures above are single measurements.

## Prose comparison (opening of haejin/s3, per arm)

English glosses in brackets are mine.

**S-son** (pass ✗, 662 chars)
> 방 안에는 바닷바람에 절은 나무 냄새가 배어 있었다. 오말순은 윗목에 앉아 무릎을 천천히 흔들며 입속으로 무슨 가락을 굴리고 있었다. 가락은 이내 말이 되어 흘러나왔다.
>
> "열한 물때에 불이 잔다."
>
> [The room was steeped in the smell of wood cured by sea wind. Oh Mal-sun sat at the warm end, slowly rocking her knees, turning some tune over in her mouth. The tune soon became words. "At the eleventh tide the fire sleeps."]

**L-son** (pass ✗, 1409 chars)
> 오말순은 바늘귀에 실을 꿰다 말고 입속으로 가락을 굴렸다. 처음엔 숨소리에 가까웠던 것이 차츰 말마디를 갖췄다. "열한 물때에 불이 잔다, 열한 물때에 불이 잔다. 우리 어머니가 부엌에서 그렇게 불렀다. 뒷말은 다 잊고 이 대목만 남았구나."
>
> [Oh Mal-sun stopped threading the needle and turned a tune over in her mouth. What was at first close to breathing gradually took on words. "At the eleventh tide the fire sleeps, at the eleventh tide the fire sleeps. My mother sang it that way in the kitchen. I forgot all the rest, and only this part is left."]

**L-hai** (pass ✓, 1477 chars)
> 한서윤은 방 한쪽 낮은 자개장 위에 놓인 오말순의 반짇고리를 본다. 뚜껑이 반쯤 열려 있고, 그 안에서 실패 몇 개가 누렇게 바랜 천 조각 사이로 드러나 있다. 창으로 들어온 오후 햇빛이 바늘쌈지 끝에 얇게 걸려 있다. 방 안은 조용하다.
>
> [Han Seo-yun looks at Oh Mal-sun's sewing box on the low mother-of-pearl chest at one side of the room. The lid is half open, and a few spools show among yellowed scraps of cloth. The afternoon sun through the window hangs thinly on the tip of the needle case. The room is quiet.]

**S-hai** (pass ✓, 1200 chars)
> 말순 할머니의 집은 마을 끝 언덕 아래 낮은 슬레이트 지붕 아래에 있었다. 마루에 앉으니 바다에서 올라온 바람이 문풍지를 가늘게 울렸다. 서윤은 무릎에 두 손을 모으고 할머니가 찻잔에 물을 따르는 손끝을 보았다.
>
> [Grandmother Mal-sun's house stood under a low slate roof below the hill at the end of the village. Sitting on the wooden floor, the wind from the sea set the paper window thinly humming. Seo-yun folded her hands on her knees and watched the fingertips of the grandmother pouring water into the teacups.]

**L-hai → ko-hapsho swap** (pass ✓)
> 방 한쪽 낮은 자개장 위에 오말순의 반짇고리가 놓여 있습니다. 뚜껑이 반쯤 열려 있고, 그 안에서 실패 몇 개가 누렇게 바랜 천 조각 사이로 드러나 있습니다.
>
> [On the low mother-of-pearl chest at one side of the room sits Oh Mal-sun's sewing box. The lid is half open, and a few spools show among yellowed scraps of cloth.]

L-hai is somewhat dry, with a run of present-tense short sentences (the style layer does not change the tense of haiku's beat prose). S-son, S-hai, and L-son narrate in the past tense. The full prose is in each arm's folder.

## Verdict (applying the brief's rules)

- **Default renderer:** the arms with the highest pass rate are S-son, L-hai, and S-hai, all at 50%. On a tie we pick the one with fewer weighted tokens, which is S-hai (80306). So **the default renderer is `single`** (no change).
- **Default render model:** in the single renderer, haiku (S-hai) is 50% and sonnet (S-son) is 50%. Within −10 pp, so **the default render model is `claude-haiku-5-5`**.
- The decision is reflected in `write.DefaultOpts`: `Renderer: "single"` (unchanged), `RenderModel: "claude-haiku-5-5"` (changed from sonnet). The re-extraction model stays sonnet.
- `dieg render --verify` uses the same defaults. The `--render-model` and `--renderer` flags are unchanged, and the README and refs copies were fixed too.

- This decision does not flip with the weights. Single beats layered in calls, raw tokens, and output tokens, and the haiku rule does not look at tokens. However, haiku's cost advantage depends on the weights, because haiku's output is +44% more than sonnet's.
- Applying the follow-up fix (in the new_event judgment, drop the author's fact-less acts that are in the IR from hallucinations) should raise the pass rate (counterfactual: L-hai 2/2, S-hai 2/2). The measured figures above are left as they are.

## Limitations

- **n = 2.** The verdict rests on only 2 serial scenes. Pass rate moves in steps of 50 pp, so the verdict is anecdote-level. Layered could not be measured on the corpus scenes (10–11 beats).
- **Noise decided the verdict.** Most failures (new_event 8/8) are the extractor re-reporting acts that are already in the IR as new_events. With the noise removed, the two haiku arms are equal to or better than the two sonnet arms. Still, the sample is small.
- **Length.** The single renderer wrote only 27–55% (662–1380 chars) of the 2500-character target. Layered wrote about 58% (1444–1477 chars). The length instruction is not followed well enough by either.
- **The cost weights are assumptions** (sonnet 3:15, haiku 1:5). The subscription CLI's cache tokens are mixed into in.
- haiku's output tokens are 2–3× sonnet's for the same character count (they appear to be reasoning tokens). The weight is low, so weighted tokens are still fewer. However, if usage limits are counted by output token count, this advantage shrinks.
- Layered made 15–20 calls per scene even on 4-beat scenes (single made 2–6). Under a limit counted in calls, layered is at a disadvantage.
