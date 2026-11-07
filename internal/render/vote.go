package render

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// DefaultK 는 렌더 하나를 다시 뽑는 횟수(R1). 홀수여야 다수가 정해진다.
const DefaultK = 3

// badExtract 는 재추출 답이 쓸 수 없을 때(멈춤·파싱 실패)의 오류다. 기록에 ExtractError 로 남는다.
// Completer 오류와 달리 다시 부를 까닭이 호출 쪽에 있지 않다.
type badExtract string

func (e badExtract) Error() string { return string(e) }

// IsBadExtract 는 err 가 재추출 답의 문제(기록에 남길 것)인지 본다.
func IsBadExtract(err error) bool {
	var b badExtract
	return errors.As(err, &b)
}

// ExtractK 는 같은 재추출 프롬프트로 k 번 따로 뽑고 Vote 로 합친다. 원본 k 개도 돌려준다(기록에 남겨 다시 판정한다).
// 하나라도 멈추거나 깨지면 badExtract 다(일부로 다수결을 하지 않는다).
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

// Vote 는 재추출 여럿의 다수결이다. 결정적이라 캐시에서 다시 판정할 때 그대로 다시 센다.
//   - 액트: 키(actor|동사류|정규 사실)마다, 과반의 재추출이 그 이상 뽑은 개수만 채택한다(멀티셋 중앙값).
//   - facts_stated: 과반이 말한 사실(정규 명제로 셈)만 채택한다.
//   - 숨긴 사실은 한 번이라도 나오면(액트든 facts_stated 든) facts_stated 에 넣는다 — 누출 판정은 fail-closed.
//   - new_events: 자유 문장이라 맞대지 못한다. IR 에 지은이가 쓴 사실 없는 행위를 다시 말한 것은 먼저 뺀다. 남은 것 중 과반이 그 이상 낸 개수만큼, 그만큼 낸 첫 재추출의 문장을 쓴다.
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
	factID := map[string]string{} // 정규 명제 → 처음 본 ID
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

// majority 는 과반(need 개)의 재추출이 다다른 가장 큰 개수: 재추출별 개수를 큰 순서로 놓은 need 번째 값.
func majority(k, need int, count func(int) int) int {
	cs := make([]int, k)
	for i := range cs {
		cs[i] = count(i)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(cs)))
	return cs[need-1]
}
