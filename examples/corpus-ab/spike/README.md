# Spike — repair pass (H1) + extraction vocabulary (H2)

A throwaway experiment. The input is Phase 1's prose and verdicts (`../p1`); nothing was newly rendered. The code is `tools/corpus-ab/spike.go` (`-spike h1|h2`), reusing the call-cap guard (`counter`), `Meter`, and `ExtractK`/`Vote`/`Diff`/`Passed` as they are. The production extraction prompt was not touched (the variants live in spike.go). Per-item JSON and repaired prose are in this folder (`h1_*`, `h2_*`), and the call log is `calls.jsonl`.

## Calls
H2 24 + H1 40 = **64 / 75**. 0 errors. Only the subscription `claude` CLI was used.

## H2 — give the extractor the author's act vocabulary (24 calls)
Variant: the extraction prompt lists the scene IR's `does` acts (verb: actor -> target) and instructs "if a sentence restates this act, report it with exactly that verb". 8 prose × K=3 sonnet/low.

**Pre-registered criterion**: all 5 noise items disappear, all 7 real ones are kept, and no new hallucination (real or noise) appears.

| Pair | Hallucination before | Hallucination after | Note |
|---|---|---|---|
| chibi en-doyle | noise "hides it in his robe" | none | noise removed |
| chibi gojeon | noise "pockets…flees" + real extra-act `not jiang_gan_envoy` | new item `extra-act:jiang_gan\|perceive\|commanders_traitors` | noise removed. Real item lost. The new item is a kind mismatch for missing `jiang_gan\|read` (real) |
| chibi haera | noise | none | noise removed |
| chibi hapsho | noise | none | noise removed (→ pass) |
| odyssey gojeon | noise "diverts attention" | none | noise removed |
| bible en-doyle | real `not peter_knows_jesus` | same | kept |
| bible hapsho | real `not peter_knows_jesus` | none (→ pass) | **real item lost** |
| bible gojeon | real 3 (withdraws, 2 extra-acts) | real 1 (`extra-act:peter\|assert\|not peter_knows_jesus`) | 3 lost; this item is new (the IR has `lies … not peter_knows_jesus` once, so the excess is real) |

- Noise removed **5/5**.
- Real kept: by JSON there are 6 real items (en 1, hapsho 1, gojeon 3, chibi gojeon 1; one off from the README's "7": the Phase 1 table said 12 but the JSON hallucination total is 11), and **1/6 kept** (bible en-doyle). The 5 lost items are `extra-act` (fact form, duplication) and embellishment rather than synonym noise, so the vocabulary injection was not aimed at them. That is, rather than the vocabulary masking real hallucinations, the K=3 re-extraction itself varies a lot (the same group of items changed in both directions).
- 2 new items, both real (0 noise): bible gojeon `not peter_knows_jesus`, chibi gojeon `jiang_gan|perceive|commanders_traitors`.
- **Verdict: fail.** Noise removal succeeded at 5/5, but 5 real hallucinations disappeared and 2 new real items appeared. The vocabulary injection itself seems to work on noise, but because of re-extraction variation we could not confirm "real hallucinations are preserved" with this sample.

## H1 — repair pass for missing facts (40 calls, per pair 1 haiku/medium + sonnet/low K=3)
All 10 Phase 1 failed pairs with missing ≥ 1 (expected about 8 → actual 10; calls 4×10). Prompt: the system prompt is English instructions (the render prompt convention) + for Korean backends the dialogue/register rules tail is appended as is (style samples omitted); the user prompt has the existing prose / the missing acts in the render prompt's `## Acts` line format / a `Must not state or imply` list. Instruction: "add only these facts with minimal edits…".

**Pre-registered criterion**: at least half of the repaired renders pass, with no increase in real hallucinations or leaks in the repaired renders.

| Pair | missing before→after | Hallucination before→after | Leaks | Chars before→after | Edit (word-diff chars) | Pass |
|---|---|---|---|---|---|---|
| bible en-doyle | 1→0 | 1→0 | 0 | 954→985 | 31 | **✓** |
| bible gojeon | 2→1 | 3→4 | 0 | 376→376 | 0 (prose identical) | ✗ |
| chibi en-doyle | 3→2 | 1→1 | 0 | 2952→2966 | 72 | ✗ |
| chibi gojeon | 2→2 | 2→1 | 0 | 830→830 | 0 (identical) | ✗ |
| chibi haera | 2→1 | 1→1 | 0 | 922→943 | 21 | ✗ (hallucination noise) |
| chibi hapsho | 1→1 | 1→1 | 0 | 962→982 | 28 | ✗ (hallucination noise) |
| macbeth en-doyle | 3→4 | 0→0 | 0 | 5884→5871 | 41 | ✗ |
| odyssey en-doyle | 2→2 | 0→0 | 0 | 3301→3410 | 181 | ✗ |
| odyssey gojeon | 3→2 | 1→2 | 0 | 1315→1349 | 42 | ✗ |
| odyssey haera | 2→2 | 0→0 | 0 | 1309→1309 | 0 (identical) | ✗ |

- **Passed 1/10** (criterion of 5/10 not met). 0 leaks.
- Classification of new hallucinations (checked against the IR): (1) odyssey gojeon `extra-act:telemachus|perceive|beggar_is_odysseus` — **real**, created by the repair (wrote perceive instead of the missing assert). (2) bible gojeon `extra-act:mark_bystanders|assert|peter_with_jesus` — **real** (in the IR mark_bystanders does not speak; the maid tells them `peter_disciple`), but the prose is **character-for-character identical** to before the repair, so it is re-extraction variation, not something the repair created. (3) The hallucination sentences in the 3 chibi pairs are steal/flee synonym noise (same type as Phase 1), not new. macbeth got worse in missing, 3→4 (`thane_of_cawdor` added).
- Even with the repair prompt, in 3 pairs (bible gojeon, chibi gojeon, odyssey haera) haiku changed not a single character of the prose.
- **Verdict: fail.** Passed 1/10, and the repair created 1 real hallucination (odyssey gojeon).

## Deviations (itemized)
1. The number of H2 pairs is 8 (5 noise + 7 real merged by render unit: chibi 4, odyssey gojeon, bible 3). 24 calls instead of the pre-registered 36.
2. Real hallucinations were counted as 6 by JSON rather than the README's 7 (above).
3. H1 ran 10 pairs instead of 8 (including bible en-doyle's missing 1 and macbeth en-doyle); 40 calls.
4. The H1 system prompt is English instructions without style samples + the Korean register tail. We interpreted "a prompt written in the backend's language" as the render prompt convention (English instructions).
5. In the first H2 run, odyssey gojeon was skipped by the budget guard (`used+4>budget`), so it was run separately with 3 calls (a `-pairs` spike flag was added). This does not affect the total calls.
6. Edit size is "the total characters of added and deleted words" counted by word LCS (a rough value).

## Limitations
- n: H2 8 renders (3 scenes), H1 10 renders (4 scenes). Backends of the same scene are not independent samples.
- Single run. Even with K=3, re-extraction varies a lot (the same prose went from 3 to 4 hallucinations), so the disappearance or appearance of individual items may be sampling variation. The verdicts are directional evidence only, not repeated measurement.
- H2 had a positive signal of 5/5 noise removed, but did not meet the criterion (real kept, no new items), so we wrote "fail". How the vocabulary variant acts on `extra-act` needs separate repeated measurement.
