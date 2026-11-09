# Cost curve zero-point analysis — does B (Diegesis) get cheaper than A (direct) in a long serial?

Hypothesis ①: A's input cost grows as the serial gets longer (since each episode must carry the earlier content), while B's per-episode cost is flat, because PG holds the state and only a digest is sent. So the longer it gets, the cheaper B becomes.

**0 model calls.** We only read the record files of `examples/serial/run1~run3`. The program is `tools/costcurve/main.go` (`go run ./tools/costcurve`), and the outputs are the CSVs in this folder.

## Conclusion

**No cost evidence.** (Pre-registered rule: B slope ≤ half of A's, and cumulative B/A at 50 episodes ≤ 0.7. In the pooled fit, B's slope is 0.85× A's and cumulative B/A at 50 episodes is 1.09, so both fall short. The "weak" condition that the direction be right, which I read as also requiring B/A < 1, is not met either — the rule has no definition of "direction", so I state this interpretation.)

The main reason is that the hypothesis's premise does not match the code. **A does not carry the full text of earlier episodes.** So A's input barely grows as the serial gets longer (below). Most of the growth in A's per-episode cost is from output (later episodes get longer), and B's input actually grew faster than A's.

## 1. The context A and B receive each episode (from the code)

- **A** (`tools/serial/direct.go` `EpisodePrompt`): harness rules + `confirmed revisions` + idea + outline (written once, fixed) + **cumulative summary** (updated by the `sum-N` call after each episode, "around 1500 characters") + **the full text of the previous episode only**. There is no full text of all earlier episodes or of the last N. So A is already, by design, the "summary" approach of (b). Two calls per episode (`ep-N`, `sum-N`).
- **B** (`tools/serial/b.go` → `dieg continue`; `internal/write/prompt.go` `ContinuePrompt`): rules + the existing **bundle of cast declaration files** + **PG state digest** + brief. Past scene code is not carried. Per episode, 1–2 `code` rounds + render (haiku) and extract (sonnet) for each scene.

Measured prompt size (= input + cache read + cache write, tokens):

| Target | Call | Ep 1 | Ep 11–12 | Growth per episode (fit, n) | Reaches 200k |
|---|---|--:|--:|--:|--:|
| A | `ep-N` | about 4.3k | about 11–12k | +341 (SE 88, R² 0.31, n=35) | episode 562 (extrapolated) |
| B | largest code call | about 15k | about 25–28k | +950 (SE 49, R² 0.93, n=33) | episode 195 (extrapolated) |

- A's input is flat at 10–12k from episode 2 on (the summary has a length cap and only the previous episode is carried).
- B's input grows — confirming (c): **B's input is not bounded.** The digest size also grows linearly (`digest.csv`: run3 2.3KB (ep 2) → 6.7KB (ep 12), about +0.45KB per episode). The growth of the code prompt (+950 tokens/episode) is not explained by the digest alone, and appears to be added to by the growing bundle of cast declarations, but we did not break it down. Up to episode 100 it is about 110k tokens, inside the 200k window (extrapolated).

## 2. Per-episode table (cost = model factor × (input + 0.1×cache read + 1.25×cache write + 5×output), opus 1 / sonnet 0.6 / haiku 0.2)

Stage boundaries use the cumulative call counts of each run's `metrics.csv`. The model is decided by label (`render-` haiku, `extract-` sonnet, the rest opus; `tools/serial` `modelOf`). Revisions (`rev1`, `rev2`) are separate rows. All data is in `steps.csv`.
Run1 A wrote up to episode 12 (outside the verdict) and B was cut off at episode 11. The episode 12 row is also in A's fit.

### run1
| Stage | A calls | A uncached input | A cache read | A cache write | A output | A cost | B calls | B uncached input | B cache read | B cache write | B output | B cost |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| outline | 1 | 2 | 0 | 3222 | 3588 | 21970 | - | - | - | - | - | - |
| ep1 | 2 | 4 | 0 | 9666 | 9071 | 57442 | 7 | 14 | 6162 | 25736 | 13762 | 69773 |
| ep2 | 2 | 4 | 0 | 17828 | 7870 | 61639 | 7 | 14 | 6162 | 28680 | 11631 | 61572 |
| ep3 | 2 | 4 | 0 | 17651 | 8378 | 63958 | 6 | 12 | 4108 | 43644 | 10971 | 92184 |
| ep4 | 2 | 4 | 0 | 18978 | 8758 | 67516 | 13 | 26 | 12324 | 38638 | 16834 | 74112 |
| rev1 | 3 | 6 | 0 | 25929 | 12245 | 93642 | 39 | 78 | 39026 | 89466 | 43538 | 140913 |
| ep5 | 2 | 4 | 0 | 20032 | 9053 | 70309 | 7 | 14 | 6162 | 29834 | 10234 | 58478 |
| ep6 | 2 | 4 | 0 | 17916 | 8320 | 63999 | 10 | 20 | 8216 | 55962 | 19054 | 125777 |
| ep7 | 2 | 4 | 0 | 17598 | 9421 | 69106 | 14 | 28 | 12324 | 65989 | 23753 | 140145 |
| ep8 | 2 | 4 | 0 | 19353 | 11040 | 79395 | 7 | 14 | 6162 | 34310 | 11990 | 69575 |
| rev2 | 7 | 14 | 0 | 98757 | 52347 | 385195 | 31 | 62 | 32344 | 81747 | 36245 | 138971 |
| ep9 | 2 | 4 | 0 | 20425 | 20789 | 129480 | 7 | 14 | 6162 | 34564 | 11984 | 69394 |
| ep10 | 2 | 4 | 0 | 21513 | 11722 | 85505 | 9 | 18 | 8216 | 38131 | 12256 | 69180 |
| ep11 | 2 | 4 | 0 | 20514 | 12681 | 89052 | 11 | 22 | 10270 | 42412 | 17189 | 91201 |
| ep12 | 2 | 4 | 0 | 20043 | 13839 | 94253 | - | - | - | - | - | - |
### run2
| Stage | A calls | A uncached input | A cache read | A cache write | A output | A cost | B calls | B uncached input | B cache read | B cache write | B output | B cost |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| outline | 1 | 2 | 0 | 3150 | 3139 | 19634 | - | - | - | - | - | - |
| ep1 | 2 | 4 | 0 | 9295 | 7844 | 50843 | 13 | 26 | 13985 | 34469 | 34056 | 94945 |
| ep2 | 2 | 4 | 0 | 17528 | 8044 | 62134 | 5 | 10 | 4294 | 25374 | 12658 | 55354 |
| ep3 | 2 | 4 | 0 | 16821 | 8576 | 63910 | 5 | 10 | 4294 | 24402 | 10964 | 50279 |
| ep4 | 2 | 4 | 0 | 17581 | 8385 | 63905 | 9 | 18 | 8588 | 30268 | 16380 | 63931 |
| rev1 | 6 | 12 | 0 | 72242 | 31006 | 245344 | 32 | 64 | 32205 | 91463 | 64119 | 177905 |
| ep5 | 2 | 4 | 0 | 19440 | 9458 | 71594 | 7 | 14 | 4294 | 68643 | 19873 | 148024 |
| ep6 | 2 | 4 | 0 | 18141 | 8773 | 66545 | 9 | 18 | 6441 | 74962 | 21295 | 144622 |
| ep7 | 2 | 4 | 0 | 18667 | 10569 | 76183 | 9 | 18 | 8588 | 35547 | 21187 | 83623 |
| ep8 | 2 | 4 | 0 | 19701 | 10666 | 77960 | 13 | 26 | 12882 | 44082 | 37172 | 103576 |
| rev2 | 4 | 8 | 0 | 45599 | 25562 | 184817 | 35 | 70 | 36499 | 90353 | 79090 | 180704 |
| ep9 | 2 | 4 | 0 | 20391 | 10584 | 78413 | 11 | 22 | 10735 | 42319 | 26421 | 89599 |
| ep10 | 2 | 4 | 0 | 17612 | 12524 | 84639 | 7 | 14 | 6441 | 35009 | 20082 | 91475 |
| ep11 | 2 | 4 | 0 | 18500 | 9943 | 72844 | - | - | - | - | - | - |
### run3
| Stage | A calls | A uncached input | A cache read | A cache write | A output | A cost | B calls | B uncached input | B cache read | B cache write | B output | B cost |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| outline | 1 | 2 | 0 | 3189 | 3138 | 19678 | - | - | - | - | - | - |
| ep1 | 2 | 4 | 0 | 9638 | 8832 | 56212 | 5 | 10 | 4048 | 22634 | 16886 | 70877 |
| ep2 | 2 | 4 | 0 | 17843 | 7939 | 62003 | 5 | 10 | 4048 | 25245 | 13049 | 61937 |
| ep3 | 2 | 4 | 0 | 18978 | 9677 | 72112 | 6 | 12 | 4048 | 46799 | 15267 | 105928 |
| ep4 | 2 | 4 | 0 | 18913 | 9885 | 73070 | 6 | 12 | 4048 | 49888 | 14113 | 100218 |
| rev1 | 6 | 12 | 0 | 80365 | 29481 | 247873 | 21 | 42 | 18216 | 80371 | 37397 | 152664 |
| ep5 | 2 | 4 | 0 | 19539 | 9112 | 69988 | 8 | 16 | 4048 | 77056 | 20371 | 162414 |
| ep6 | 2 | 4 | 0 | 18650 | 9307 | 69852 | 6 | 12 | 4048 | 34266 | 14407 | 80585 |
| ep7 | 2 | 4 | 0 | 17544 | 8660 | 65234 | 5 | 10 | 4048 | 30869 | 13830 | 77048 |
| ep8 | 2 | 4 | 0 | 18172 | 10134 | 73389 | 7 | 14 | 4048 | 37974 | 19669 | 99469 |
| rev2 | 10 | 20 | 0 | 151550 | 75192 | 565418 | 10 | 20 | 8096 | 50579 | 27245 | 113451 |
| ep9 | 2 | 4 | 0 | 20071 | 11494 | 82563 | 7 | 14 | 4048 | 63755 | 14018 | 119746 |
| ep10 | 2 | 4 | 0 | 19491 | 14090 | 94818 | 5 | 10 | 4048 | 36271 | 16273 | 89985 |
| ep11 | 2 | 4 | 0 | 18127 | 11231 | 78818 | 5 | 10 | 4048 | 35524 | 24383 | 128759 |
| ep12 | 2 | 4 | 0 | 18753 | 9971 | 73300 | 5 | 10 | 4048 | 36472 | 18839 | 104942 |

## 3. Per-episode cost slope (revisions and outline excluded, simple linear in episode number)

`fits.csv`. The cost unit is the weighted tokens above.

| Series | n | Intercept | Slope/episode | SE | R² |
|---|--:|--:|--:|--:|--:|
| A overall (3 runs) | 35 | 55,250 | **+2,875** | 515 | 0.49 |
| B overall (3 runs) | 33 | 77,568 | **+2,457** | 1,540 | 0.08 |
| A run1 / run2 / run3 | 12 / 11 / 12 | | +4,002 / +2,511 / +1,928 | | 0.52 / 0.76 / 0.49 |
| B run1 / run2 / run3 | 11 / 10 / 12 | | +1,289 / +3,216 / +2,692 | | 0.03 / 0.09 / 0.12 |

- **n is small.** Each series has 10–12 points, 33–35 when pooled, and those are the same single item run three times, so they are not independent samples.
- A's slope is positive and about 5× its SE, so the increase itself is credible. B's SE is 60% of its slope and R² is 0.08, so neither "flat" nor "grows like A" can be distinguished. B's per-episode cost is driven by retries and scene verification counts (10–16 calls).
- Of A's slope of about 2,900, the context share is small: a prompt growth of 341 tokens × cache write 1.25 ≈ 430, about 15%. The rest is output (×5) — later episodes (especially 9–11) are long (the ep call output goes from about 5k to 8–13k tokens). This looks like the effect of larger payoff volume in the late story rather than context accumulation, and there is weak ground to extrapolate it linearly to episode 50 or 100.

## 4. Cumulative cost projection (B/A, revisions excluded, per-episode cost = fitted line, negatives clipped to 0)

`projection.csv`.

| Assumption | 12 ep | 24 ep | 50 ep | 100 ep |
|---|--:|--:|--:|--:|
| (a) A's current policy + pooled fit | 1.27 | 1.19 | **1.09** | 1.01 |
| (a) run1 fit only | 1.09 | 0.91 | 0.71 | 0.56 |
| (a) run2 fit only | 1.35 | 1.33 | 1.32 | 1.31 |
| (a) run3 fit only | 1.38 | 1.38 | 1.39 | 1.39 |
| (b') hypothetical policy where A carries all earlier full texts, all as cache reads (0.1) | 1.21 | 1.10 | 0.97 | 0.87 |
| (b') same hypothetical policy, all as cache writes (1.25) | 0.79 | 0.59 | **0.43** | 0.34 |

Assumptions:

- **(a) A keeps its current policy.** As in section 1, A's context does not reach the 200k window (about episode 562 by extrapolating the prompt growth). So the "input grows in proportion to length" that hypothesis ① speaks of does not occur under A's current policy. The projection is a linear extrapolation of the whole cost curve (see the caution in section 3: output growth is the main cause), and splits into 0.56–1.39 depending on the run. The sign flips because B's SE is large.
- **(b) Past the window, A must summarize.** A already uses a summary. Under the hypothetical policy of carrying all earlier full texts (`A-full`: average output of 6,498 tokens per episode accumulates, episode 1 prompt 4,320), it exceeds 200k at **about episode 31**. After that it would have to summarize, and a summary loses detail — this is the point where B's advantage (state is kept exactly in PG) has meaning. The (b') rows above add only the cost of `A-full`, and are an unrealistic upper bound that assumes full texts keep being carried after episode 31. The result swings wildly with the cache hit-rate assumption (50 episodes 0.97 ↔ 0.43). **Quality (loss of detail, continuity errors) cannot be measured here.** The cost curve alone cannot capture that effect.
- **(c) Assume B's digest is bounded.** Measurement says it is not (section 1). The B in the projection uses the measured slope as it is, so this is already reflected. If the digest and cast were assumed bounded, B's slope might fall below 2,457, but this data has no evidence for that.
- Revision cost is excluded. The cost of one revision varies widely, about 94k–565k for A and about 113k–153k for B (by run1 and run3), so we did not mix it into the per-episode slope.

## 5. Verdict

| Condition | Value | Met |
|---|---|---|
| A per-episode cost slope > 0 and B slope ≤ half of A's | A +2,875, B +2,457 (0.85×) | No |
| Cumulative B/A at 50 episodes ≤ 0.7 | 1.09 (only run1 at 0.71) | No |
| Direction is right (B slope < A, B/A at 50 episodes < 1) | slope weakly right, B/A above 1 | No |

**No cost evidence.** However, because of the limitations, we cannot say "we proved that B does not get cheaper as it gets longer". B's slope in this data is too uncertain to distinguish.

## 6. Limitations

- Few points (n=10–12 per run), and they are repetitions of the same item, the same plan, and the same revision requests, so they are not independent. There is no data past 12 episodes, so 24, 50, and 100 are all extrapolations.
- The linear fit does not decompose the cause of the cost growth (output growth). The extrapolation to a per-episode cost of about 200k for A at 50 episodes (pooled fit) and about 340k at 100 may be unrealistic.
- Prices are assumptions (s=0.6, h=0.2, output = input × 5), not an actual price list. It is a token weighting, not subscription usage.
- The label→model mapping is an assumption in `modelOf` (the records have no model).
- **Quality was not examined.** The longer it gets, the more A may lose detail through summaries and gain continuity errors, while B may not, since PG verifies. Hypothesis ① is a cost claim and this analysis looks only at cost. If B has a chance of winning, it is on continuity error rate, not cost, and this analysis cannot tell us that.
- A and B have different episode counts: run1 A 12 / B 11, run2 A 11 / B 10, run3 both 12. That is why the pooled fit n is 35 vs 33.

## Files

- `steps.csv` — per run, arm, and stage: calls, uncached input, cache read, cache write, output, weighted cost, largest prompt
- `fits.csv` — linear fits (n, intercept, slope, SE, R²)
- `projection.csv` — cumulative cost at 12/24/50/100 episodes and B/A
- `digest.csv` — B digest size
- `a_ep_calls.csv` — prompt and output tokens of A's `ep-N` calls
