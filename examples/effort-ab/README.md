# Render effort A/B (Plan 6, Task 5)

We ran haiku (`claude-haiku-5-5`) renders at effort `medium` and `low` and compared them by card verifier score. The result decides the default `--render-effort` (applied in Task 7).

- Harness: `go run ./tools/corpus-ab -spike effort -budget 60 -out examples/effort-ab` (dry run `-spike effort-dry`, regenerate only the table with `-spike effort-report`).
- One `render.RenderVerified2` run per scene (`K:1, Retries:2, Verifier:"card", Card:{}`). Grading and re-grading use sonnet/low. All backends are `ko-haera`.
- Per scene, medium then low were run back to back. A budget guard (60, hard cap) precedes every call.
- TargetChars is 0, so S4 (length) is omitted and Score is `(0.6·S1+0.2·S2+0.1·S3)/0.9`. S1 = fact present, S2 = fact form correct, S3 = register.
- Weighted tokens are `3·(input+cache)+15·output` (the tools/corpus-ab weights), summed over render and grading calls.
- Prose and per-scene JSON are in `runs/`, the call log in `calls.jsonl`, the table in `table.md`.

## Scenes (6)

| Scene | Source |
|---|---|
| bible/mark_denial | corpus render-check |
| chibi/45.88 | corpus render-check |
| macbeth/1.3 | corpus render-check |
| odyssey/19.1 | corpus render-check |
| haejin/s3 | serial haejin (top 2 by act count, FreeDialogue) |
| haejin/s19 | serial haejin (same) |

## Results by scene and arm

| Scene | effort | Pass | Attempts | Score | S1 | S2 | S3 | Render output tokens/call | Render calls | Grading calls | Weighted tokens |
|---|---|---|---|---|---|---|---|---|---|---|---|
| bible/mark_denial | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3043 | 1 | 2 | 86889 |
| bible/mark_denial | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3830 | 1 | 1 | 81618 |
| chibi/45.88 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 5599 | 1 | 1 | 115512 |
| chibi/45.88 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 6118 | 1 | 1 | 124650 |
| macbeth/1.3 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 8646 | 1 | 1 | 171756 |
| macbeth/1.3 | low | ✓ | 1 | 0.998 | 1.00 | 1.00 | 0.98 | 5122 | 1 | 1 | 117714 |
| odyssey/19.1 | medium | ✓ | 2 | 0.968 | 1.00 | 0.86 | 1.00 | 5288 | 2 | 4 | 256386 |
| odyssey/19.1 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 5412 | 1 | 2 | 134853 |
| haejin/s3 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 4480 | 1 | 1 | 101706 |
| haejin/s3 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 4104 | 1 | 1 | 92244 |
| haejin/s19 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3522 | 1 | 1 | 83238 |
| haejin/s19 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3273 | 1 | 1 | 78900 |

## Averages (both arms finished all 6 scenes)

| | medium | low |
|---|---|---|
| Mean Score | 0.9947 | 0.9997 |
| Scenes passed | 6/6 | 6/6 |
| Render output tokens/call (summed) | 5123.9 | 4643.2 |
| Weighted tokens per scene | 135,915 | 104,997 |

## Verdict

Pre-registered rule: choose low if it meets all of: mean Score drop ≤ 0.02, no decrease in passed scenes, and a decrease in render output tokens/call.

- Mean Score drop (medium−low) = −0.0049 (low is actually slightly higher) → met
- Passed 6 vs 6 → met
- Render output tokens/call 5123.9 → 4643.2 (−9.4%) → met

Default effort: low

## Actual call count

**30 calls** (budget 60). 0 errored calls, 0 wasted. The budget guard never cut a run off.

## Limitations

- n=6 scenes, one run per scene and arm. No standard deviation can be computed.
- The difference is small. The Score gap is almost entirely one medium retry in odyssey (S2 0.86) and a 0.02 register score in macbeth. Another run could flip it.
- Per-scene render output tokens/call (medium → low) differ in sign. Scenes where low used more: bible 3043 → 3830, chibi 5599 → 6118, odyssey 5288 → 5412 (medium is the mean over 2 retries). Scenes where low used fewer: macbeth 8646 → 5122, haejin/s19 3522 → 3273, haejin/s3 4480 → 4104. Three against three.
- Without macbeth, render output tokens/call are 4536.8 for medium and 4547.4 for low, so low is actually slightly higher (recomputed from calls.jsonl). The token criterion passed thanks to the single macbeth scene. The verdict follows the pre-registered rule as written, and the token gain is not robust at n=6.
- Low's higher mean Score is because only medium needed one retry in odyssey (it is not evidence of a quality advantage).
- The pass criterion is ADR-028 (card verifier: 0 leaks and missing+wrong-kind ≤ 1, G2 is a warning only), so many scores sit on the ceiling (1.0) and discrimination is low.
- The card verifier is a single sonnet/low judgment. Grading noise was not measured separately.
