# Spike — multi-axis scorecard verifier (tier-1) tested on variants

A throwaway experiment. The code is `tools/corpus-ab/sc*.go` (`-spike sc-gen|sc-run|sc-old|sc-report|sc-dry`). Production prompts were not touched. Calls used only the subscription `claude` CLI (sonnet/low), with the `counter` budget guard and the `Meter` call log as they are. Runs were one at a time, foreground, with no work reloading (already in PG).

## Background
In the verifier reproducibility spike (`../verifier`), the open-extraction (S1) verdict flipped on 1/3 of the same manuscripts. The design note `rendering.md`'s "multi-axis scorecard (proposal)" is the alternative: one call returns both the checklist (G1, S1, S2) and an open list (G2), register and length are measured deterministically, and only borderline cases are re-asked in a second tier. We measure whether this proposal meets the criteria (per-axis detection ≥90%, false alarms ≤5%, flips ≤5%, cost per render ≤ the old 3 calls, score ordering) using variants whose correct answer is mechanically known.

## Design (what was implemented)
- **Tier 1 (1 call per render)**: scene catalog + checklist (the `Diff` key `(actor|verb class|fact)` in plain language, including subject and verb class) + facts to hide + prose. It answers in one JSON.
  - For each item: `fact_present` (is the content in the prose, whoever did it), `as_stated` (did that subject do it with that verb class), and the shortest quotation. S1 = fact_present, S2 = fact_present and as_stated.
  - For each fact to hide: `revealed` + quotation (G1).
  - `other`: events or facts not on the checklist + quotation (G2). Each event goes through `render.Diff` (the `authored()` rule) and is dropped if it is explained by an authored act. In addition, **it is dropped if subject, verb class and object overlap a fact-less authored act (orders, whispers, sees, etc.)** (`coveredByIR`, added in this spike — `authored()` only honors does and presence). The prompt also includes the authored `does` act vocabulary (the same prescription as the earlier spike H2).
  - If the JSON is broken or an id is missing, one re-request.
- **Tier 2 (once, only when needed)**: items with fact_present=no or as_stated=no in tier 1, plus G2 candidates, are re-asked in a single call. Items are overwritten with the tier-2 answer, and candidates get a verdict of `extra`/`covered`/`trivial`, keeping only `extra` (if tier 2 is broken, candidates are conservatively kept). G1 is not re-asked in tier 2.
- **Deterministic**: S3 = the backend's target register ratio from `ClassifyTone`, S4 = min(chars, target)/max(chars, target). Target chars = the character count of the pre-variation base prose (a stand-in value, since there is no real target setting).
- **Score** = 0.6·S1 + 0.2·S2 + 0.1·S3 + 0.1·S4 (S1 and S2 denominator = expected item count), efficiency = score / (weighted tokens/10k), weighted tokens = 3·(in+cache)+15·out (sonnet, the same assumption as renderer-ab). Gates (G1, G2) are reported separately from the score.

## Variant ground truth
Built from base prose (the ko renders of Phase 1 `../p1` + serial haejin s3) by deleting or inserting exactly one sentence at a time; the truth is in `truth/<variant>.json` and the prose in `variants/<variant>.md`. The edits are in `sc_variants.go`, and generation fails unless there is "exactly 1 match".

| Base | Variants | Edit | Truth |
|---|---|---|---|
| chibi hapsho | orig / D1 / D3 / H | D1: delete E5 (Zhou Yu's "Cao Cao's head"). D3: delete E1 (hidden meaning, playing the envoy), E2 (Jiang Gan's denial), and E5. H: insert the sentence "군량 창고에 불이 났다" ("the grain storehouse caught fire") | D: that item absent. H: 1 invented event |
| macbeth haera | orig / D1 / D3 / K | D1: delete E12 (Cawdor's treason). D3: delete E12, E7 (prosperous gentleman), banquo_line (E4, E8) = 3 facts, 4 items. K: swap the speaker from Ross to Angus | K: E10 present but as_stated=false |
| bible gojeon | K | In "heard the cock crow" swap the one who hears, Peter→the maid | E4 present but as_stated=false |
| haejin s3 haera | orig / H / L | **Synthetic withheld**: this scene has no withheld fact, so only inside the spike `minjae_leaves` (Min-jae leaves) was marked withheld, and the base (orig) is the prose with that sentence deleted. H: insert "이웃집 노인이 돈 봉투를 쥐여 주었다" ("the old man next door pressed a money envelope into her hand"). L: insert a sentence where Min-jae says he "is going to the mainland on this afternoon's boat and is not coming back" | L: W1 revealed. H: 1 invented event |

- The truth for base items comes from the earlier spike's `../verifier/truth.json` (labelled by me; items marked uncertain are excluded from the metrics); haejin was read and labelled all present.
- We built 16 variants and ran 12 (24 runs). `orig-odyssey`, `orig-bible`, `d1-bible`, and `h-macbeth` were only generated and not run, due to budget. So there are **4 bases** (chibi, macbeth, bible, haejin), odyssey from the plan's 5 is missing, and there are 12 variants instead of the plan's ~18.

## Method
- New verifier: 2 runs per variant (`runs/<variant>_r1|r2.json`), raw call text in `calls/`, call log `calls.jsonl` (with tokens, recorded right after each call).
- Old verifier: on 5 variants (D1 `d1-chibi`, D3 `d3-macbeth`, H `h-chibi`, K `k-macbeth`, L `l-haejin`), `ExtractK` K=3 → `Vote` → `Diff` → `Passed` (`old/`).
- Dry run beforehand: with a fake completer (`-spike sc-dry`, 0 calls) we passed generation, execution, broken-JSON retry, tier 2, the old verifier, and the report end to end, then started the real calls.
- **Fixed once midway**: in the first 4 calls (orig-chibi ×2), G2 raised a false alarm by writing the original's `steal` (authored act `does steal forged_letter`) as "Jiang Gan puts the letter in his sleeve". The old verifier has the same false alarm. We put the authored act vocabulary in the prompt and re-ran (the first result is kept in `v1-prompt/`). Those 4 calls are wasted. Then in orig/d1-chibi, fact-less authored acts (`orders`, `whispers`) surfaced as candidates, so we added `coveredByIR`. This matcher was added after the 4 chibi runs, so **the G2 of the 4 chibi runs is a value re-filtered with the same matcher at report time** (the tier-2 calls of those 4 runs might have been unnecessary had the matcher existed, so cost is slightly overstated).
- Item agreement (flips) = the share of items whose answer is the same across the two runs of the same variant. Gate agreement = the share where the G1 and G2 results are the same.

## Per-axis table (trial = variant × run)
| Axis | Detection (variant items) | False alarms | Agreement across two runs | Flipped |
|---|---|---|---|---|
| S1 fact coverage | 18/18 (100%) — deleted items of D variants | 0/168 (0%) | 95/95 | 0% |
| S2 fact form | 4/4 (100%) — K 2 variants×2 runs (18/18 including deleted items) | 5/168 (3%) — wrong subject/verb among items seen as present | 90/95 | 5.3% |
| G1 leak | 2/2 (100%) — L 1 variant×2 runs | 0/4 | 3/3 | 0% |
| G2 invention | 2/4 (50%) — H: haejin 2/2, chibi 0/2 | 8/24 (33%) on originals and non-H variants | 10/12 (gate) | 17% |

- Both false alarms and detection exclude `Unsure`-labelled items. Items whose truth differs within a variant entered the false-alarm denominator as "unmodified items".
- **Breakdown of the 8 G2 false alarms** (see the quotations in `report.txt`): 6 are macbeth-family (orig ×2, d1 r2, d3 r2, k ×2), all restatements of facts already on the checklist ("반역죄가 자백되고…" ("treason is confessed and…") = E12, "무거운 심판 아래…" ("under a heavy judgment…") = E11 — present in the original IR quotations) that tier 2 judged `extra` = true false alarms. 2 (k-bible ×2: "베드로가 자리를 피하여 문간으로 나가니" ("Peter avoided the seat and went out to the doorway")) are **hallucinations that really exist in the original prose** (the old verifier also caught it in Phase 1 as "Peter moves away from the fire to the gateway") = true detections. Without these 2 it is 6/22 (27%).
- **G2 misses**: h-chibi's "군량 창고에 불" ("fire in the grain storehouse") did not appear in the `other` list in either run (the model treated it as background description, and the list was filled only with restatements of authored acts). The call ended after one, so there was no candidate to re-ask in tier 2.
- All item flips occurred in macbeth D1 and D3 (±1–2 items in S2's "who" judgment).

## Score ordering
| Base | run1 (orig / D1 / D3) | run2 |
|---|---|---|
| chibi | 1.000 / 0.861 / 0.581 | 1.000 / 0.861 / 0.581 |
| macbeth | 1.000 / 0.938 / 0.730 | 1.000 / 0.896 / 0.758 |
| bible | (orig, D1, D3 not run) | |

- orig > D1 > D3 holds on both bases in both runs (4/4). The score difference between two runs of the same variant averages 0.006, at most 0.043 (macbeth d1).
- Gates: L → G1 fails 2/2. H → G2 fails haejin 2/2, chibi 0/2.
- The original score is 1.000: the model answered all expected items as "present" (chibi E6 was excluded from the metrics as uncertain but counts toward the score). The score can be generous toward "present" (limitation).

## Cost
| | Calls per render | Weighted tokens per render | Sample |
|---|---|---|---|
| New verifier | **1.75** (tier 1: 1.0 + tier 2: 0.75, non-retry) | **33,855** | 24 runs |
| Old verifier K=3 | 3 | 66,184 | 5 variants (different variants, so a rough comparison) |

- Tier 2 fired in 18 of 24 runs (75%). That is much higher than the design note's expectation (+0.3 calls on average): D and K variants by definition always have absent items, and originals almost always raised G2 candidates too.
- Broken-JSON retries: 0 of 24 runs (path confirmed only in the dry run).
- Total real calls **61** (cap 65). 4 of them wasted (orig-chibi run with the first prompt, `v1-prompt/`).

## Verdict
| Axis | Verdict | Basis |
|---|---|---|
| S1 coverage | **Pass** | detection 100%, false alarms 0%, flips 0%, score ordering holds, cost ≤ old |
| S2 fact form | **Inconclusive** | detection 4/4 but only 2 distinct mutations (K, 2), flips 5.3% marginally exceed the 5% criterion, false alarms 3% pass |
| G1 leak | **Inconclusive** | 2/2, 0/4, 3/3 but 1 variant and a synthetic withheld (the work has no real withheld scene) |
| G2 invention | **Fail** | detection 50% (<90), false alarms 27–33% (>5), gate flips 17% (>5) |
| S3, S4 | Not applicable (deterministic, not a variant target) | |
| Cost | Pass | 1.75 calls < 3, half the weighted tokens |
| Score ordering | Pass | |
| **Overall** | **Inconclusive (partial adoption)** | Only S1 is confirmed. G2 fails with this design — the open list cannot tell "description" from "new event" (misses) and emits checklist restatements as `extra` (false alarms). Without G2, the 1-call checklist (S1, S2, G1) looks usable, but S2 and G1 need more samples. |

## Limitations
- The truth is set by the variants. Labels for base items are my labels from the earlier spike (uncertain ones excluded), and the haejin base was read and labelled all present. One labeller.
- Samples per mutation type are small: 8 distinct S1 deletions, 2 S2 (K), 1 G1, 2 G2 (H). Two runs of the same variant give nearly the same answers (sonnet/low is fairly deterministic), so "2-run agreement" does not give confidence like independent trials. The detection-rate confidence intervals are wide.
- S1 and S2 have a generous bias: the original (including uncertain chibi E6) scores 1.000. Against a base with truly missing items, we measured only with D variants.
- G1 is one synthetic withheld fact, and a directly mentioned sentence (an expression similar to the source text), so it is an easy case. Implied leaks were not measured.
- The G2 matcher `coveredByIR` was added in this spike, and the 4 chibi runs got it after the fact. An invented event with the same subject, verb class and object could also be masked (we eased this by limiting it to fact-less authored acts).
- The old verifier comparison covers 5 variants of differing kinds, so it is not an accuracy comparison. Old (summary): D1 detection ✓, D3 3 of 4 ✓, K ✓, L (leak) ✓, H ✗; however all 5 have `Passed=false` (false alarms such as chibi's `steal` hallucination and missing/extra-act on uncertain items in K and L) — the new verifier missed H-chibi among the same 5 variants and caught the rest, and its gate false alarms on originals concentrated in G2 (macbeth).
- S4's target chars were replaced by the base prose length (variants change only the length, so S4 weakly reflects the variant effect). S3 covers only Korean ko-* backends.
- The weighted-token ratio (3:15) is an assumption, and the subscription CLI's cache tokens are mixed into in.
