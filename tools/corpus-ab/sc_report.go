package main

// 스코어카드 스파이크: 정답(truth)과 실행 결과를 맞대 축별 지표를 낸다. 호출 없음.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type ctr struct{ hit, n int }

func (c *ctr) add(ok bool) {
	c.n++
	if ok {
		c.hit++
	}
}
func (c ctr) String() string {
	if c.n == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", c.hit, c.n, 100*float64(c.hit)/float64(c.n))
}

func norm(s string) []rune {
	return []rune(strings.Map(func(r rune) rune {
		if strings.ContainsRune(" \n\t“”\"'‘’.,!?", r) {
			return -1
		}
		return r
	}, s))
}

// overlap: 두 문자열의 최장 공통 부분문자열 길이(룬).
func overlap(a, b string) int {
	x, y := norm(a), norm(b)
	best := 0
	prev := make([]int, len(y)+1)
	for i := 1; i <= len(x); i++ {
		cur := make([]int, len(y)+1)
		for j := 1; j <= len(y); j++ {
			if x[i-1] == y[j-1] {
				cur[j] = prev[j-1] + 1
				best = max(best, cur[j])
			}
		}
		prev = cur
	}
	return best
}

func loadRun(out, name string, n int) (scRun, bool) {
	var r scRun
	b, err := os.ReadFile(filepath.Join(out, "runs", fmt.Sprintf("%s_r%d.json", name, n)))
	if err != nil || json.Unmarshal(b, &r) != nil {
		return r, false
	}
	return r, true
}

func scReport(ctx context.Context, out string) {
	var det = map[string]*ctr{"S1": {}, "S2": {}, "S2(D)": {}, "G1": {}, "G2": {}}
	var fa = map[string]*ctr{"S1": {}, "S2": {}, "G1": {}, "G2": {}}
	var agree = map[string]*ctr{"S1": {}, "S2": {}, "G1": {}, "G2": {}, "gate": {}}
	var dscore, calls, wt, nr float64
	var maxd float64
	scores := map[string][2]float64{}
	t2 := 0
	fmt.Println("### 변형별 (run1 / run2)")
	fmt.Println("| 변형 | 호출 | S1 | S2 | S3 | S4 | 점수 | G1 실패 | G2 실패 | 후보(확정) |\n|---|---|---|---|---|---|---|---|---|---|")
	for _, vd := range scVariants {
		t0 := vd
		_, t := readVariant(out, t0.Name)
		var rs [2]scRun
		ok := [2]bool{}
		for i := range rs {
			rs[i], ok[i] = loadRun(out, t0.Name, i+1)
			if ok[i] {
				sc, _, _, _ := scBase(ctx, t0.Base)
				rs[i].G2Fail = false
				for k := range rs[i].Cands {
					c := &rs[i].Cands[k]
					c.Covered = coveredByIR(sc, c.Ev)
					rs[i].G2Fail = rs[i].G2Fail || (c.Confirmed && !c.Covered)
				}
			}
		}
		if !ok[0] && !ok[1] {
			continue
		}
		mut := map[string]bool{}
		for _, m := range t.Mutated {
			mut[m] = true
		}
		line := []string{t0.Name}
		for i, r := range rs {
			if !ok[i] {
				continue
			}
			nr++
			calls += float64(r.Calls)
			wt += r.Weighted
			t2 += r.Tier2
			sc := scores[t0.Name]
			sc[i] = r.Score
			scores[t0.Name] = sc
			cs := []string{}
			for _, c := range r.Cands {
				cs = append(cs, fmt.Sprintf("%s[%s%s]", c.Quote, c.Verdict, map[bool]string{true: ",IR", false: ""}[c.Covered]))
			}
			fmt.Printf("| %s r%d | %d | %.2f | %.2f | %.2f | %.2f | %.3f | %v | %v | %s |\n", t0.Name, i+1, r.Calls, r.S1, r.S2, r.S3, r.S4, r.Score, r.G1Fail, r.G2Fail, strings.Join(cs, "; "))
			// 항목별 검출·오경보
			for id, tr := range t.Expect {
				if tr.Unsure {
					continue
				}
				a := r.Items[id]
				switch {
				case mut[id] && !tr.Present:
					det["S1"].add(!a.Present)
					det["S2(D)"].add(!a.AsStated)
				case mut[id] && tr.Present && !tr.AsStated:
					det["S2"].add(!a.AsStated)
				case !mut[id] && tr.Present:
					fa["S1"].add(!a.Present)
					if a.Present {
						fa["S2"].add(!a.AsStated)
					}
				}
			}
			for id, rv := range t.Withheld {
				if rv {
					det["G1"].add(r.Leak[id])
				} else {
					fa["G1"].add(r.Leak[id])
				}
			}
			if len(t.Invented) > 0 {
				hit := false
				for _, c := range r.Cands {
					hit = hit || (c.Confirmed && !c.Covered && overlap(c.Quote+c.Ev.Label(), t.Invented[0]) >= 6)
				}
				det["G2"].add(hit)
				extra := 0
				for _, c := range r.Cands {
					if c.Confirmed && !c.Covered && overlap(c.Quote+c.Ev.Label(), t.Invented[0]) < 6 {
						extra++
					}
				}
				fa["G2"].add(extra > 0)
			} else {
				fa["G2"].add(r.G2Fail)
			}
		}
		_ = line
		if ok[0] && ok[1] {
			for id, tr := range t.Expect {
				if tr.Unsure {
					continue
				}
				agree["S1"].add(rs[0].Items[id].Present == rs[1].Items[id].Present)
				agree["S2"].add(rs[0].Items[id].AsStated == rs[1].Items[id].AsStated)
			}
			for id := range t.Withheld {
				agree["G1"].add(rs[0].Leak[id] == rs[1].Leak[id])
			}
			agree["G2"].add(rs[0].G2Fail == rs[1].G2Fail)
			agree["gate"].add(rs[0].G1Fail == rs[1].G1Fail && rs[0].G2Fail == rs[1].G2Fail)
			d := rs[0].Score - rs[1].Score
			if d < 0 {
				d = -d
			}
			dscore += d
			maxd = max(maxd, d)
		}
	}
	fmt.Println("\n### 축별 지표 (시행 = 변형 × 실행)")
	fmt.Println("| 축 | 검출 | 오경보 | 두 실행 일치(항목/변형) |\n|---|---|---|---|")
	for _, k := range []string{"S1", "S2", "G1", "G2"} {
		fmt.Printf("| %s | %s | %s | %s |\n", k, det[k], fa[k], agree[k])
	}
	fmt.Printf("\nS2 검출(삭제 항목도 포함): %s / 게이트 일치: %s / 점수 |Δ| 평균 %.4f 최대 %.4f\n", det["S2(D)"], agree["gate"], dscore/float64(max(1, agree["gate"].n)), maxd)
	fmt.Printf("실행 %d회: 렌더당 평균 호출 %.2f, 가중 토큰 %.0f, 2단 발동 %d/%d\n", int(nr), calls/max(nr, 1), wt/max(nr, 1), t2, int(nr))
	fmt.Println("\n### 점수 순서 (원본 > D1 > D3)")
	var bs []string
	for _, b := range []string{"chibi", "macbeth", "bible"} {
		bs = append(bs, b)
	}
	sort.Strings(bs)
	for _, b := range bs {
		o, d1, d3 := scores["orig-"+b], scores["d1-"+b], scores["d3-"+b]
		for i := 0; i < 2; i++ {
			fmt.Printf("%s r%d: orig %.3f  D1 %.3f  D3 %.3f  원본>D1 %v  D1>D3 %v\n", b, i+1, o[i], d1[i], d3[i], o[i] > d1[i], d1[i] > d3[i] || d3[i] == 0)
		}
	}
	// 옛 검증기
	fmt.Println("\n### 옛 검증기 K=3")
	files, _ := filepath.Glob(filepath.Join(out, "old", "*.json"))
	var ow, oc float64
	for _, f := range files {
		var rec struct {
			Variant  string
			Report   struct{ Missing, Hallucinated, Leaked []string }
			Passed   bool
			Weighted float64
			Err      string
		}
		b, _ := os.ReadFile(f)
		_ = json.Unmarshal(b, &rec)
		ow += rec.Weighted
		oc += 3
		fmt.Printf("%s: passed=%v missing=%v hall=%v leak=%v weighted=%.0f err=%s\n", rec.Variant, rec.Passed, rec.Report.Missing, rec.Report.Hallucinated, rec.Report.Leaked, rec.Weighted, rec.Err)
	}
	if len(files) > 0 {
		fmt.Printf("옛: 렌더당 호출 3, 가중 토큰 %.0f (%d개)\n", ow/float64(len(files)), len(files))
	}
	_ = utf8.RuneLen
}
