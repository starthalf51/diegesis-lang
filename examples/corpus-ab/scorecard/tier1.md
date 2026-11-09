Tier2: absent-only

Rule: if tier 1 alone gives S1 detection >= 90% and false alarms <= 5%, tier 2 is off; otherwise absent-only. S1 detection 16/18 (89%), false alarms 0/168 (0%).

Raw tier-1 output re-graded over 24 runs (no tier-2 overwrite). Trial = variant x run

| Axis | Detected (variant items) | False alarms | Agreement across two runs | Flipped |
|---|---|---|---|---|
| S1 | 16/18 (89%) | 0/168 (0%) | 95/95 (100%) | 0.0% |
| S2 | 4/4 (100%) | 19/168 (11%) | 90/95 (95%) | 5.3% |
| G1 | 2/2 (100%) | 0/4 (0%) | 3/3 (100%) | 0.0% |

S2 detection (including deleted items): 18/18 (100%)
Check: 44 items overwritten by tier 2 (of which 4 had Present changed); tier-1 answer == final answer for items not overwritten: 174/174 (100%)

## Data source and limitations
- The tier-1 answers are re-parsed from the `raw` field (tier-1 raw output) of `calls/<variant>_r<n>_t1_a.json`. The tier-2 call is a separate file (`_t2_a.json`), and only `runs/*.json`, which holds the final answers, carries the values tier 2 overwrote (marked `T2:true`). The raw output is thus stored separately and was recomputed without estimation. There were no broken-JSON retries (`_b`) (24/24 parsed).
- Definitions match `sc_report.go`: S1 = fact_present, S2 = fact_present and as_stated, G1 = revealed, `Unsure` excluded, detection = the model answered "absent" for an item deleted in a D variant, false alarm = the model answered "absent" for an unmodified item whose answer is present (for S2, items seen as present with a wrong as_stated). Compared with the spike table (tier 1 + tier 2): S1 detection 18/18 -> 16/18, S1 false alarms 0/168 unchanged, S2 false alarms 5/168 (3%) -> 19/168 (11%), S2 detection, G1, agreement and flips unchanged.
- Tier 2 overwrote 44 items, of which 4 had Present changed. For the 174 items it did not overwrite, tier-1 and final answers are identical (recomputation consistency check).
- G2 was not recomputed: `coveredByIR`/`Diff` need the scene IR in PG, and the verdict rule uses only S1. G2 already failed in the spike, and under `absent-only` candidates are not asked again.
- The verdict is close to the boundary: S1 detection 16/18 = 88.9% falls one item short of the 90% criterion (17/18 = 94%). Both misses are E2 of d3-chibi (denial between letters, "calling me an envoy of Cao Cao's side") and are the same in r1 and r2: tier 1 answered "present" and tier 2 corrected it to "absent". So this item is a case where tier 2 was needed (the 18/18 in the spike table is thanks to tier 2).
