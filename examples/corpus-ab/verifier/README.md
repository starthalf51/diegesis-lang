# Spike — verifier reproducibility (open extraction S1 vs closed checklist S2)

A throwaway experiment. We re-verified the Phase 1 prose (`../p1`) as is, without re-rendering. The code is `tools/corpus-ab/verifier.go` (`-spike dump|s1|s2|report`). Production prompts were not touched (the S2 prompt lives in verifier.go). Calls used only the subscription `claude` CLI (sonnet/low), with the `counter` budget guard as it is.

## Why we did it
In the earlier spike (`../spike`), re-extracting the **same prose** changed the verdict (only 1 of 6 real hallucinations was rediscovered, with 2 new items). Before fixing the renderer, we measure the verifier's own reproducibility and see whether a closed question (a checklist) is an alternative.

## Sample (including deviations from the plan)
The plan was 1 per scene (4), but **the actual comparison has 3**: bible `ko-gojeon`, chibi `ko-hapsho`, macbeth `ko-haera`. For odyssey `ko-haera` we made only the truth labels and could not make the calls — because of a budget accident (see "Calls" below). Pass/fail is mixed (Phase 1: bible failed, chibi failed, macbeth passed). We picked renders that have missing facts.
- **These 4 scenes have 0 withheld facts.** The leak truth and measurement are empty. This experiment cannot tell us the reproducibility of leak detection.

## Method
- Expected items = the `(actor|verb class|canonical fact)` key that `Diff` uses (duplicates merged into one). `expected_*.json` has the items and plain-language statements.
- **Truth (truth.json)**: I read the prose myself and, per item, recorded present/absent + a supporting quotation + an uncertainty mark. One labeller (me). 5 uncertain: bible E1, E5, chibi E3, E6, macbeth E14 (+odyssey E6).
- **S1**: the production extraction prompt (`BuildExtractPrompt`) called alone 6 times per render. Single-call item = "present" if not in `Diff`'s missing. Vote A = calls 1–3, B = 4–6, `Vote`+`Diff`+`Passed` (including hallucinations). 6-call majority = `Vote` over 6 (4 votes).
- **S2**: the same prose + numbered expected facts (plain language) → per-item yes/no + shortest quotation JSON. 6 calls per render. Vote = yes in at least 2 of 3 calls, pass = missing ≤ 1 and leak 0, hallucinations ignored. 6-call majority = at least 4 yes.
- Metrics: (a) item agreement = mean over items of max(p, 1−p) (p = the share of "present" among 6 calls), (b) agreement of the two votes on pass/fail, missing, hallucination, and leak sets, (c) item accuracy against the truth (6-call majority, and the two K=3 votes combined).

## Results (3 renders, 28 items) — detailed tables in `report_3.txt`
| | Item agreement (a) | Accuracy 6-majority (c) | Accuracy K3 votes (c) | Pass/fail agreement between votes (b) | Missing set agreement | Hallucination set agreement |
|---|---|---|---|---|---|---|
| S1 open | 94.0% | 78.6% | 82.1% | 2/3 | 1/3 | 2/3 |
| S2 closed | 94.0% (see note below) | 96.4% | 96.4% | 3/3 | 3/3 | 3/3 (hallucinations ignored, always an empty set) |

**Key S1-only result: voting K=3 twice on the same prose flips the pass/fail verdict on 1 of 3 (33%)** (macbeth: A passes / B fails, the Glamis claim missing only in B). The missing sets differed in 2 of 3 (only bible among bible, chibi, macbeth was the same), and the hallucination sets differed in 1 of 3 (bible). The 94% item agreement looks high mostly because most items are 6/6, and the wobbling items (macbeth E5 3/6, chibi E1 3/6, bible E3 4/6) change the verdict. S1 also has items that are **consistently wrong**: macbeth E14 0/6 and bible E1 0/6 (truth is present, marked uncertain). So there is a bias problem as well as reproducibility.

S2 note: the 6th bible call returned broken JSON (a quote-escaping error), and everything was treated as "no". This one case dragged S2 item agreement down to 94.0%. Excluding the broken call, S2 is **98.8%**. Per the pre-registered criterion, including the broken call, it ties with S1.

## Verdict: inconclusive
Of the 3 pre-registered criteria:
1. Item agreement S2 ≥ 90% and higher than S1 — including it, 94.0 vs 94.0 is a tie, **not met** (strictly); excluding the broken call, 98.8 > 94.0 met.
2. Accuracy S2 ≥ S1 — 96.4 ≥ 78.6/82.1 **met**.
3. Pass/fail agreement between votes S2 ≥ S1 — 3/3 ≥ 2/3 **met**.

Criterion 1 is a tie, so strictly it is not an "S2 win", and the reason the criterion wobbles is one format error in S2 (S2 can produce broken JSON — it needs a retry or parser hardening). The direction favors S2 (accuracy +14–18 pp, 0 verdict flips), but with n=3 we do not confirm it. S1's verdict flip (1/3) is a confirmed result either way.

**S2's limitation (mandatory)**: a closed checklist **only confirms facts that are in the prose**. By principle it cannot catch invented events or facts (hallucinations). Hallucination detection still needs an open extraction pass. The S2 hallucination column in this table is "not measured". Also S2 almost always answers yes for items whose truth is "present" (leniency), so it risks missing facts that are actually missing; in this sample the only true omission (truth = absent) is chibi E6 (uncertain), so **S2's omission detection is unverified**: on that item S2 gave 5/6 yes (wrong), and S1 also 5/6 (wrong).

## Calls
**54 recorded** (within the cap of 60; up to 56 if a call was in flight at the forced stop). `accident_calls.jsonl` 13 + `calls.jsonl` 41.
- Accident 1: we ran `dump` mode before wiring it into main, and the existing `runSpike` wasted 13 calls on the stale H2 path (result files deleted). That left too little budget, so we dropped odyssey's S1/S2.
- Accident 2: overlapping background jobs ran bible S1 in two processes (5 calls wasted, only 6 files remain). Terminated immediately.
- Data used: S1 18 calls (bible 6, chibi 6, macbeth 6; 5 of the 23 recorded were duplicates and wasted) + S2 18 calls.
- `go vet ./...`, `go test -count=1 ./...` passed.

## Limitations
- n=3 renders (28 items), one labeller (me) — the 5 uncertain labels could shift the results.
- We tested only one extractor, sonnet/low. Other models or efforts may differ.
- The truth is almost all "present", so omission-detection accuracy was effectively not measured. With 0 withheld facts, leaks were not measured either. Hallucinations were not labelled.
- The S2 item statements were made mechanically from the IR (results may vary with the plain-language phrasing).
- This folder: `s1_*`/`s2_*` = per-call JSON (including raw responses), `truth.json` = truth, `expected_*.json` = item tables, `report_3.txt` = tables.
