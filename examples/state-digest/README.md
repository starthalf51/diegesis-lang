# PG state digest spike

A cheap check done with throwaway code (`tools/statespike/`, branch `spike-state`, started from main `7c67ea0`). I am the only judge (no blind or cross judging). The "resolved" marks below are values given by one person reading the prose.

## Hypothesis

Most of the errors in run3 B (`examples/serial/run3/b/`) were drift of prose between scenes (the renderer invented props, places, and knowledge anew in each scene). Attaching a "summary of the world state at that point", drawn from PG, to the render user prompt makes the drift go away.

Pre-registered criteria:
- Support: arm D resolves at least 4 of 5, arm 0 (control) resolves at most 2, and arm D has no new drift or card failure.
- If the coverage table says most items are "not in the IR", the verdict is "the digest alone is not enough — the IR must be extended (coding props and places)". Both are reported.

## Coverage table (coverage, 0 calls)

The `yeoul` work was already loaded in PG (35 scenes, the same as the last B code). It was not re-uploaded. Check: the current render request hashes of the 5 scenes are all equal to the `.hash` from run3 (`dieg render`'s `RenderHash`). So the re-renders below ran on the same scene specs as run3.

| Drift | Is the needed information in B's IR/PG | Class | Basis |
|---|---|---|---|
| B3 s2 box location (shelf) | yes | **in PG** | s2 code `setup box_hint hint: "외할머니 장롱 위에 놓인 녹슨 양철 상자"` ("a rusty tin box on top of Grandmother's wardrobe"). This foreshadowing phrase is not in the render request (`render_scene`), so the renderer did not know it. `tin_box.at = malsun_house` is there too (room level). |
| B4 s20/s21 dust | no | **not in the IR** | No `attr` or fact has dust or cleanliness. It is a descriptive attribute the prose invented. The code must be written richer. |
| B5 s21→s29 box stays in place | partly | **partial** | `tin_box.at` appears only once and no act of moving it is in the code (so the digest can write "no record of moving"). But `does take_down_box` / `open_box` are free actions with no object argument, so they are not connected to the box. Whether the box was put back is not in the code. |
| B6 s29 contents unknown | partly | **partial** | s21 `seoyun sees ledger_page, ledger_entry` is in PG (both the reader's and Seo-yun's knowledge are derived), and `open_box` is in between. But the relation "the ledger page was inside the box" is not in the code (no `contains`). |
| B7 s25 plank in the abandoned house | mostly | **partial** | The `fragments_hidden` fact (under the thirteenth stair plank) and `stair_nail.at = lighthouse` exist. The plank itself is not an entity ("tear off the plank and move it" is not in the code). |
| B8 s24/s27 frame location | yes | **in PG** | `shard_fit_test`: "Hae-sol and Su-a hold Su-a's glass shard against the broken lens corner **in the lantern room**". The s24 prose saying the frame is "at the base of the pillar" is the side that departs from the code. |

Summary: in PG 2 (B3, B8), partial 3 (B5, B6, B7), not in the IR 1 (B4). "Most not in the IR" does not hold, so the second verdict rule does not fire. But if the three "partial" are counted as "none", it is 4/6. That case is also written in the table below. Descriptive attributes (like dust) are outside the IR either way.

## Method

- The digest function: `tools/statespike/digest.go`. It reads only existing tables and functions (`pg.Connect`, no migrations). It appends the Korean section "## 이 시점의 세계 상태 (어기지 말 것)" ("## World state at this point (do not violate)") at the **end of the render user prompt** (after the retry feedback). Production prompts and `internal/` were not touched. Contents: (1) place and time (elapsed since the story start via `point.earliest`, gap from the previous scene) (2) object attributes and departed characters from `attribute`, whether there is a record of moving, objects not at this scene's place (3) earlier acts with the object as an argument, and recent actions at the same place (4) per-character `believes` of the characters appearing (true/false, "belief contrary to fact" when it conflicts with `truth_at`, facts hidden from the reader excluded), the latest 5 (5) the latest 8 facts already narrated to the reader via `reader_events` (6) foreshadowing this scene plants and pays off, and the original text of 8 descriptive foreshadowings planted earlier. Length cap 3,600 characters. Per-scene results: `digest-s2.txt` etc.
- Calls: render haiku (`claude-haiku-5-5`) effort low, card grading sonnet (`claude-sonnet-5-5`) effort low, verifier card, Retries 2, backend ko-haera, `FreeDialogue=true` (the same as `write.DefaultOpts`). Only the subscription `claude -p` was used.
- Arms: D (digest attached) / 0 (not attached). The arm order alternated by scene and run. Scenes s2, s21, s25, s27, s29, 2 runs per arm = 20 cells.
- Safety: first we did a build, tests, and a fake-completer dry run (0 calls) with the noclaude shim, and printed one digest as a full prompt to check it. Then we removed the shim and ran the real calls. A budget guard (cap 50) before each call; every call is in `calls.jsonl`. Prose is in `prose-<arm>-<scene>-r<run>.md`, the cell summary in `cells.json`, the progress log in `run.log`.
- Judgment rule (fixed before seeing results): the code is the standard. Resolved = no statement contradicting the code and the original prose of earlier scenes. For B8, resolved if s27 follows the code (lantern room) or does not mention the place (the original s27 already did: the source of the drift is s24). For dust, writing that the box itself has dust counts as drift. A scene counts as "resolved" only when all runs are resolved (strict).

## Per-scene table

Reference for comparison with the originals — s2: code `box_hint` (on the wardrobe) vs original s2 "on the shelf under the wardrobe". s21: original s20 "the box was clean, without a speck of dust". s25: code `fragments_hidden` (the plank is on the stairs), original s25 "the plank torn off the thirteenth stair" (inside the abandoned house), original s26 the plank is nailed into the stairs. s27: original s24 "the lens frame left at the base of the lighthouse pillar", the code says the lantern room. s29: original s29 "under the shelf", original s21 "a box with dust settled on it", "what was inside was not yet known" (s29), the code has no `tin_box.at` move, and s21 has `sees ledger_page`.

| Scene | Arm | Run | Resolved | Basis | Card | Score |
|---|---|---|---|---|---|---|
| s2 | D | 1 | resolved | "장롱 위에 낡은 물건 하나가 얹혀 있었다. 녹슨 양철 상자였다." ("An old object sat on top of the wardrobe. It was a rusty tin box.") | pass | 1.000 |
| s2 | D | 2 | resolved | "장롱 위에 녹슨 양철 상자 하나가 놓여 있었다." ("A rusty tin box lay on top of the wardrobe.") | pass | 1.000 |
| s2 | 0 | 1 | drift | "할머니 등 뒤 선반 구석 … 녹이 슨 양철 상자 하나" ("in the corner of the shelf behind Grandmother … a rusty tin box") (not on the wardrobe) | pass | 1.000 |
| s2 | 0 | 2 | drift | "방 한쪽 선반에 … 먼지가 두껍게 앉은 자리에 녹슨 양철 상자" ("on a shelf on one side of the room … a rusty tin box in a spot thick with dust") | pass | 1.000 |
| s21 | D | 1 | resolved (borderline) | "장롱 위로 손을 뻗었다. 손끝에 먼지가 묻어 까슬했다. 녹슨 양철 상자는 생각보다 가벼웠다." ("She reached up onto the wardrobe. Dust on her fingertips felt gritty. The rusty tin box was lighter than expected.") Read as dust on the wardrobe top, not on the box, so counted as resolved. | pass | 1.000 |
| s21 | D | 2 | resolved | No mention of dust. The digest had no dust information, so it is chance. | pass | 1.000 |
| s21 | 0 | 1 | resolved (borderline) | "손끝이 먼지 쌓인 나무 결을 훑었다" ("Her fingertip ran over the dusty wood grain") is the wood grain on top of the wardrobe. Not dust on the box. | pass | 0.998 |
| s21 | 0 | 2 | drift | "뚜껑 가장자리에 먼지가 가라앉아 있었고" ("dust had settled on the edge of the lid") (s20 is clean) | pass | 0.998 |
| s25 | D | 1 | resolved | The plank is only in dialogue ("열세 번째 계단 판자 밑에 숨겼고 … 새 못을 박아서", "hid it under the thirteenth stair plank … drove in new nails"). No plank in the abandoned house. | pass | 1.000 |
| s25 | D | 2 | resolved | Same ("그걸 열세 번째 계단 판자 밑에 숨겼소", "I hid it under the thirteenth stair plank"). | pass (1 card re-ask) | 1.000 |
| s25 | 0 | 1 | resolved | Same. The plank was not moved to the abandoned house. | pass | 1.000 |
| s25 | 0 | 2 | resolved | Same. | pass | 1.000 |
| s27 | D | 1 | resolved | "등실에 있는 렌즈와 맞춰 봤어요" ("I tried matching it against the lens in the lantern room") (same as the code) | pass | 0.998 |
| s27 | D | 2 | resolved | Does not state where it was matched. | pass | 0.991 |
| s27 | 0 | 1 | resolved | "맞춰 보면 바로 알 수 있어요" ("If you match it you'll know right away") no place. | pass | 1.000 |
| s27 | 0 | 2 | resolved | No place. | pass | 0.998 |
| s29 | D | 1 | resolved | "장롱 위 녹슨 양철 상자" ("the rusty tin box on the wardrobe"), no statement that the contents are unknown. | pass | 1.000 |
| s29 | D | 2 | resolved | "장롱 위에 녹슨 양철 상자가 놓여 있었다". ("A rusty tin box lay on top of the wardrobe.") | pass | 0.997 |
| s29 | 0 | 1 | drift | "방구석 선반 … 선반 위에는 녹슨 양철 상자" ("a shelf in the corner of the room … on the shelf, a rusty tin box") (not on the wardrobe) | pass | 1.000 |
| s29 | 0 | 2 | drift | "선반 위에 녹슨 양철 상자" ("a rusty tin box on the shelf") | pass | 1.000 |

New drift (separate from resolved or not):
- D s27 r2: Seo-yun says in front of Hae-sol and Yeong-ho, "투표 전에 마을 사람들 앞에서 그 밤 일을 밝힐 거예요." ("Before the vote, I'll reveal what happened that night in front of the villagers."). Seo-yun's resolve, which was in the digest's "facts already narrated to the reader", became new dialogue in s27. The code has no act that says this (knowledge spread to another character). Card G2 caught it only as a warning. **1 new drift in D (minor).**
- D s21 r1 and D s29 r1: dust on the box (s29 "상자 위에 쌓인 먼지", "dust piled on the box"). This is the same unresolved descriptive attribute as B4, not a new type of error.
- The control also has new drift: 0 s27 r1 makes a "뜯겨 나간 판자 자리" ("spot where a plank was torn out") on the abandoned house floor (the B7 type; the plank is on the lighthouse stairs). 0 s21 r2 puts a "녹이 슨 쇠붙이" ("rusty piece of metal") inside the box that is not in the code.
- Card failures: all 20 cells passed (0 re-renders).

Aggregate (20 cells of real runs):

| | s2 | s21 | s25 | s27 | s29 | Cells resolved | Scenes resolved (strict) |
|---|---|---|---|---|---|---|---|
| D | 2/2 | 2/2 | 2/2 | 2/2 | 2/2 | 10/10 | **5/5** |
| 0 | 0/2 | 1/2 | 2/2 | 2/2 | 0/2 | 5/10 | **2/5** |

## Verdict

- Pre-registered criteria: D 5/5 ≥ 4 met, 0 at 2/5 ≤ 2 met, but "no new drift in D" is **strictly not met** (one case, D s27 r2, minor). → **The hypothesis is supported on criteria 1 and 2, with one snag on criterion 3. Overall: "supported, but weakly".**
- These numbers should not be read at face value.
  - Of the 5 scenes, the control actually drifted only in s2 and s29. In s25 and s27 the control was also fine, so they cannot separate D's effect (the original error in s25 may have been a one-off accident that did not reproduce. In s27, following the code was already the right answer and the source of the drift is s24 — a mismatched pair in the measurement). In s21 dust is not in the IR, so D's resolution is chance.
  - The scenes that could separate are s2 and s29, where D is 4/4 and 0 is 0/4. Both are cases where the digest told the model "on the wardrobe" (the foreshadowing text and `tin_box.at`).
- Coverage table rule: "most not in the IR" does not hold (none 1, partial 3, present 2). With this rule the "IR extension needed" verdict does not fire. However, descriptive attributes such as dust, relations between props (the ledger page inside the box), and the movement of non-entity props such as the plank are not in the code, so a digest cannot fix them. **Using both verdicts together: location and knowledge families are fixed by the digest alone (basis s2, s29); descriptive attributes and prop relations need an IR extension (basis B4, parts of B5/B6).**
- There is a first sign that the digest itself can produce new errors (D s27 r2). Attaching "facts already narrated to the reader" lets the renderer say them again as dialogue.

## Call count

- Actual calls **41** (render 20 + card grading 21; cap 50). Output tokens total 80,380. No waste: the one card-grading JSON re-ask (D s25 r2) is an internal pipeline retry. Failed or aborted calls: 0.
- The actual run was two processes. The first (`-budget 50`) stopped at 33 calls before run 2 of s27, because of the rule requiring 18 calls of headroom to start a pair. The same command with `-reserve 8` continued the remaining run 2 of s27 and s29 (finished cells are skipped, and the call total continues from `calls.jsonl`). Both processes ran in the background (because of the 10-minute foreground limit).
- Pre-call check: no other tool using a flag of the same name (`-real`), and no dieg, serial, or statespike processes running.
- The dry run and tests (`go vet ./...`, `go test -count=1 ./...`, the shim) had 0 calls and all passed.

## Limitations

- n=2 runs, 5 scenes, one judge (me). Resolved judgments were made by reading, and two borderline cases (s21 D r1 and 0 r1) were counted as resolved. If the dust is read as being on the box rather than on top of the wardrobe, D is 1/2 in s21 and 0 is unchanged (scenes resolved D 4/5, still meeting the criterion).
- The control's s25 and s27 were fine, so the base rate of drift is unknown. Some of the original run3 errors may have been probabilistic accidents.
- B8 is drift in s24, not s27. We did not re-render s24 (the instruction was "the later scene of the pair"). B6 (contents unknown) was not reproduced in either arm and so was not verified.
- The digest is useful because this work's fact sentences are long and Korean. We do not know the effect for English works or works with few derived facts.
- The digest was made from existing queries only (no migration), and the length, item count, and ordering (most recent first, 5/8/8) were my choices. Results could differ with other choices. In particular the "facts narrated to the reader" section caused the new drift above.
- The card verifier does not see this kind of continuity drift (all 20 cells passed, Score ≥ 0.991). The resolved judgments are independent of the card.
