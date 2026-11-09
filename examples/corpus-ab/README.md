# Corpus scene A/B — single+haiku vs layered+haiku (after the verifier fix, v0.5.0)

Question: after the verifier noise fix (v0.5.0), does the current default (single verified renderer, haiku/medium render, sonnet/low re-extraction, 2 retries) pass on the render-check corpus scenes? If it fails, does layered+haiku fix it?

- The harness is `tools/corpus-ab` (`go run ./tools/corpus-ab -mode single|layered -budget N -out DIR [-pairs ...]`). renderer-ab has its 4 scenes hard-coded, so we left it alone and built a sibling tool. The call cap is enforced by `counter`.
- "16 scenes" means **4 scenes × 4 backends** (ko-haera, ko-hapsho, ko-gojeon, en-doyle). Only 4 corpora have a render_scene in expect.yaml: bible, chibi, macbeth, odyssey. Below, one pair (scene × backend) is called one run. Backends of the same scene are not independent samples.
- The pass criterion is `render.Passed`: 0 hallucinations, 0 leaks, missing ≤ 1, no extraction errors. (render-check's Jaccard and register conditions are not used.)
- Artifacts: `p1/` (single, 16 pairs, JSON, prose, calls.jsonl), `p2/` (layered), `p1.log`, `p2.log`. Prompts and the verifier were not changed.

## Call counts

| Phase | Cap | Used |
|---|---|---|
| Phase 1 single+haiku, 16 pairs | 100 | **84** |
| Phase 2 layered+haiku | 60 | **58** |

0 call errors. Model calls were made only through the subscription `claude` CLI.

## Scenes

| Scene | Acts | Beats |
|---|---|---|
| bible/mark_denial | 10 | 4 |
| chibi/45.88 | 24 | 10 |
| macbeth/1.3 | 26 | 11 |
| odyssey/19.1 | 29 | 10 |

## Phase 1 — single+haiku

**Passed 5/16 (31%)**. 11 failures.

| Scene (beats) | ko-haera | ko-hapsho | ko-gojeon | en-doyle | Scene passes |
|---|---|---|---|---|---|
| bible/mark_denial (4) | ✓ (4 calls) | ✗ | ✗ | ✗ | 1/4 |
| chibi/45.88 (10) | ✗ | ✗ | ✗ | ✗ | 0/4 |
| macbeth/1.3 (11) | ✓ (2) | ✓ (4) | ✓ (2) | ✗ | 3/4 |
| odyssey/19.1 (10) | ✗ | ✓ (6) | ✗ | ✗ | 1/4 |

Failure reasons (missing / hallucinated / leaked):

| Pair | Calls | Chars | missing | hallucinated |
|---|---|---|---|---|
| bible en-doyle | 6 | 954 | 1 (`peter\|assert\|not peter_with_jesus`) | extra-act `peter\|assert\|not peter_knows_jesus` |
| bible gojeon | 6 | 376 | 2 (perceive missing, assert missing) | "베드로가 불에서 문간으로 물러난다" ("Peter withdraws from the fire to the doorway") + 2 extra-acts (fact-form kind mismatch) |
| bible hapsho | 6 | 455 | 0 | extra-act `peter\|assert\|not peter_knows_jesus` |
| chibi en-doyle | 6 | 2952 | 3 (`jiang_gan\|read`, `wu_officers\|assert`, `zhou_yu\|assert`) | "장간이 서신을 로브에 숨긴다" ("Jiang Gan hides the letter in his robe") |
| chibi gojeon | 6 | 830 | 2 | "서신을 챙기고 나중에 도망간다" ("takes the letter and flees later") + extra-act `jiang_gan\|assert\|not jiang_gan_envoy` |
| chibi haera | 6 | 922 | 2 | "장간이 편지를 몰래 소매 속에 감추었다" ("Jiang Gan secretly tucked the letter into his sleeve") |
| chibi hapsho | 6 | 962 | 1 (`jiang_gan\|read`) | "장간이 서신을 챙긴다" ("Jiang Gan takes the letter") |
| macbeth en-doyle | 6 | 5884 | 3 (`holds(king,macbeth)` ×2, `holds(thane_of_glamis)`) | – |
| odyssey en-doyle | 6 | 3301 | 2 | – |
| odyssey gojeon | 6 | 1315 | 3 | "아테나가 페넬로페의 주의를 돌린다" ("Athena diverts Penelope's attention") |
| odyssey haera | 6 | 1309 | 2 | – |

Leaks are 0 for all 16 pairs. The 3 macbeth pairs that passed also lacked one `holds(king,macbeth)` (the allowed limit is 1).

### Hallucination classification (checked against the IR)

We matched the 12 hallucination items one by one against the corpus IR (`corpus/*/…dieg.md`).

| Item | IR | Verdict |
|---|---|---|
| chibi en-doyle "hides it in his robe" | `jiang_gan does steal forged_letter` | **noise (new type)** |
| chibi gojeon "takes the letter and flees later" | `does steal` + `does flee` | **noise (new type)** |
| chibi haera "tucked it into his sleeve" | `does steal` | **noise (new type)** |
| chibi hapsho "takes the letter" | `does steal` | **noise (new type)** |
| odyssey gojeon "Athena diverts attention" | `athena does divert penelope` (19.30) | **noise (new type)** |
| bible en-doyle, hapsho extra-act `not peter_knows_jesus` | mark_denial has `lies … not peter_knows_jesus` only once (Mark 14:71). The prose turned the first denial (`not peter_with_jesus`) into "I don't know him" or said it once more | real (fact substitution / duplication) |
| bible gojeon "withdraws" | never used as an act ("went out on the porch" appears only in a `sees` quotation) | real (embellishment) |
| bible gojeon extra-act ×2 | kind changed perceive→assert, assert→act. Paired with the 2 missing | real (fact-form kind) |
| chibi gojeon extra-act `not jiang_gan_envoy` | the IR's lie is a single fact `not_envoy` | real or duplicate statement (not confirmed) |

- **5 noise, 7 real.** All the noise is the prose restating a fact-less `does` with a different verb. `authored()` compares `new_event.Action` and the IR act's verb by **exact string match** (steal≠hide/pocket, flee≠도망, divert≠돌리다), so the v0.5.0 fix cannot filter these. Unlike the earlier noise (entrances/exits, reach, hide), this is a new type: **synonym verb mismatch**. We did not fix the verifier.
- Counterfactual with only the noise removed: chibi hapsho has 0 hallucinations and missing 1, so it passes. The other 4 (chibi 3, odyssey gojeon) have missing ≥2 and still fail. Counterfactual pass rate **6/16**. Real failures **10/16**.
- The dominant form of real failure is **missing facts in long scenes** (missing 2–3 in chibi, odyssey, macbeth en-doyle). The short bible has fact-form substitution and kind mismatch, not omissions.

### Pass rate by beat count

| Beats | Runs | Passed | Note |
|---|---|---|---|
| 4 | 4 | 1 (25%) | 1 scene, bible |
| 10 | 8 | 1 (13%) | chibi, odyssey |
| 11 | 4 | 3 (75%) | 1 scene, macbeth |

Beat count and pass rate are not monotonic. The figures are split by 3–4 scenes, so scene-specific properties (e.g. macbeth's one miss falls within the limit) matter more than any beat-count effect.

## Phase 2 — layered+haiku (`narrate.RenderLayered`, beat retries default)

Because of budget we ran only 2 pairs (long scenes first). chibi haera and odyssey haera **could not be run**, since the estimated 28 calls exceeded the remaining budget (27) (chibi by one call).

| Pair | Single | Layered | Calls (single→layered) | Attempts per beat | Adopted |
|---|---|---|---|---|---|
| macbeth/1.3 en-doyle (11 beats) | ✗ missing 3 | **✗** missing 2 (`holds(king)`, `holds(thane_of_glamis)`), hallucinated 0 | 6 → 33 (5.5×) | 1,1,1,2,1,1,1,1,1,3,1 (beat 10 failed) | unstitched |
| bible/mark_denial ko-hapsho (4 beats) | ✗ hallucinated 1 | **✗** missing 1, hallucinated 1 (extra-act `not peter_knows_jesus`) | 6 → 25 (4.2×) | 3,3,3,1 (beat 1 failed) | stitched |

Layered reduced missing from 3 to 2 on macbeth but did not reach the pass limit (≤1). On bible the hallucination remains.

## Calls and tokens

| Phase | Calls | haiku (render) in/out | sonnet (re-extraction) in/out |
|---|---|---|---|
| Phase 1 (16 pairs) | 84 (5.3 per pair) | 149,712 / 250,600 | 125,994 / 34,712 |
| Phase 2 (2 pairs) | 58 (29 per pair) | 46,511 / 60,342 | 71,392 / 8,035 |

in is input+cache_read+cache_write. Phase 2 is about 4.8× the single calls (12) for the same two pairs. Per-pair tokens are in each JSON.

## Verdict

Pre-registered rule: if layered turns at least half of Phase 1 failures on long scenes (≥8 beats) into passes and total calls ≤ 4× single, "keep layered for long scenes only"; otherwise "layered is a deletion candidate".

- Phase 1 had 8 failures on long scenes (chibi 4, odyssey 3, macbeth 1). Half would be 4 conversions.
- One long scene was run layered: **0 passes**, calls 5.5× (>4×).
- **Verdict: layered is a deletion candidate** (rule applied as written). The evidence is weak, though. With n=1 long scene we can only say "no evidence of an effect and the cost is 4–5×", not "layered has no effect". Layered turned 0/2 into passes.
- Side result: the current default (single+haiku) passes 5/16 (6/16 with noise removed). Real failures are mainly missing facts in long scenes, and a new synonym-verb-mismatch noise remains on the verifier side.

## Limitations

- The 16 pairs are backend variants of 4 scenes and not independent. At scene level, n=4.
- Phase 2 covers 2 pairs (1 long scene). Because of budget, chibi and odyssey could not be run layered. The other 9 are unmeasured.
- One run per scene, so run-to-run variance is unknown. The macbeth en-doyle result is also a single layered run.
- In the classification, `not jiang_gan_envoy` was not confirmed because we did not check the IR's fact normalization.
