package render

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// DefaultK is the number of times one render is re-extracted (R1). It must be odd so a majority is defined.
const DefaultK = 3

// badExtract is the error when a re-extraction answer is unusable (stopped, parse failure). It is left in the record as ExtractError.
// Unlike a Completer error, the reason to call again is not on the caller's side.
type badExtract string

func (e badExtract) Error() string { return string(e) }

// IsBadExtract checks whether err is a problem with the re-extraction answer (to be recorded).
func IsBadExtract(err error) bool {
	var b badExtract
	return errors.As(err, &b)
}

// ExtractK extracts k times separately with the same re-extraction prompt and merges them with Vote. It also returns the original k (recorded so the verdict can be redone).
// If even one stops or breaks it is badExtract (no majority is taken over the partial ones).
func ExtractK(ctx context.Context, c Completer, s Scene, lang, prose string, k int) (Extracted, []Extracted, error) {
	sys, user := BuildExtractPrompt(s, lang, prose)
	var xs []Extracted
	for i := range k {
		res, err := c.Complete(ctx, sys, user)
		if err != nil {
			return Extracted{}, xs, fmt.Errorf("extract %d/%d: %w", i+1, k, err)
		}
		if res.StopReason != "end_turn" {
			return Extracted{}, xs, badExtract(stopped("extract", res.StopReason))
		}
		e, err := ParseExtracted(res.Text)
		if err != nil {
			return Extracted{}, xs, badExtract(err.Error())
		}
		xs = append(xs, e)
	}
	return Vote(s, xs), xs, nil
}

// Vote is the majority vote over several re-extractions. It is deterministic, so it is simply recounted when re-judging from cache.
//   - acts: for each key (actor|verb class|canonical fact), adopt only the count that a majority of re-extractions extracted at least (multiset median).
//   - facts_stated: adopt only facts a majority stated (counted as canonical propositions).
//   - a hidden fact that appears even once (as an act or in facts_stated) goes into facts_stated — the leak verdict is fail-closed.
//   - new_events: free sentences, so they cannot be matched. First drop those that re-tell an author act with no fact in the IR. Of the rest, use the count that a majority emitted at least, with the sentence of the first re-extraction that emitted that many.
func Vote(s Scene, xs []Extracted) Extracted {
	out := Extracted{Acts: []ExtractedAct{}, FactsStated: []string{}, NewEvents: []NewEvent{}}
	if len(xs) == 0 {
		return out
	}
	need := len(xs)/2 + 1
	withheld := map[string]bool{}
	for _, id := range s.Withheld {
		withheld[s.Canon(id)] = true
	}
	var order []string
	examples := map[string][]ExtractedAct{}
	actCounts := make([]map[string]int, len(xs))
	factRuns := map[string]int{}
	factID := map[string]string{} // canonical proposition → the ID first seen
	var factOrder []string
	leaked := map[string]bool{}
	for i, x := range xs {
		actCounts[i] = map[string]int{}
		seen := map[string]bool{}
		note := func(id string, stated bool) {
			if id == "" {
				return
			}
			c := s.Canon(id)
			if _, ok := factID[c]; !ok {
				factID[c] = id
				factOrder = append(factOrder, c)
			}
			if withheld[c] {
				leaked[c] = true
			}
			if stated && !seen[c] {
				seen[c] = true
				factRuns[c]++
			}
		}
		for _, a := range x.Acts {
			k := actKey(a.Actor, a.Verb, s.Canon(a.Fact))
			if _, ok := examples[k]; !ok {
				order = append(order, k)
			}
			actCounts[i][k]++
			if actCounts[i][k] > len(examples[k]) {
				examples[k] = append(examples[k], a)
			}
			note(a.Fact, false)
		}
		for _, id := range x.FactsStated {
			note(id, true)
		}
	}
	for _, k := range order {
		n := majority(len(xs), need, func(i int) int { return actCounts[i][k] })
		out.Acts = append(out.Acts, examples[k][:n]...)
	}
	for _, c := range factOrder {
		if factRuns[c] >= need || leaked[c] {
			out.FactsStated = append(out.FactsStated, factID[c])
		}
	}
	sort.Strings(out.FactsStated)
	// authored restatements are dropped first, so they cannot hide a majority invention
	kept := make([][]NewEvent, len(xs))
	for i, x := range xs {
		for _, ev := range x.NewEvents {
			if !s.authored(ev) {
				kept[i] = append(kept[i], ev)
			}
		}
	}
	n := majority(len(xs), need, func(i int) int { return len(kept[i]) })
	for _, k := range kept {
		if len(k) >= n {
			out.NewEvents = append(out.NewEvents, k[:n]...)
			break
		}
	}
	return out
}

// majority is the largest count that a majority (need) of re-extractions reached: the need-th value of the per-re-extraction counts in descending order.
func majority(k, need int, count func(int) int) int {
	cs := make([]int, k)
	for i := range cs {
		cs[i] = count(i)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(cs)))
	return cs[need-1]
}
