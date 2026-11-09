package main

// 일회용 스파이크: 검증기 재현성(열린 추출 S1 vs 닫힌 체크리스트 S2). 프로덕션 프롬프트는 건드리지 않는다.
//   -spike dump|s1|s2|report  (-out examples/corpus-ab/verifier, -budget N)

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

var vPairs = []string{"bible/mark_denial.ko-gojeon", "chibi/45.88.ko-hapsho", "macbeth/1.3.ko-haera", "odyssey/19.1.ko-haera"}

const reps = 6

type vItem struct{ ID, Key, Text string }

type vSet struct {
	Pair, Prose string
	Expect      []vItem // E1.. : Diff 가 기대하는 (행위자|동사류|사실) 키, 중복은 하나로
	Withheld    []vItem // W1..
	Want        map[string]int
}

func vName(s render.Scene, lang, id string) string {
	if e, ok := s.Entity(id); ok && len(e.Names[lang]) > 0 {
		return e.Names[lang][0]
	}
	return id
}

func buildSet(sc render.Scene, lang, pair, prose string) vSet {
	v := vSet{Pair: pair, Prose: prose, Want: map[string]int{}}
	vp := map[string]string{"assert": "says, conveys or thinks", "perceive": "sees, hears or notices", "read": "reads or writes", "act": "does"}
	for _, b := range sc.Beats {
		for _, a := range b.Acts {
			if a.Fact == "" || render.VerbClass(a.Verb) == "presence" {
				continue
			}
			k := a.Actor + "|" + render.VerbClass(a.Verb) + "|" + sc.Canon(a.Fact)
			if v.Want[k]++; v.Want[k] > 1 {
				continue
			}
			f, _ := sc.FactByID(a.Fact)
			var ts []string
			for _, t := range a.Targets {
				ts = append(ts, vName(sc, lang, t))
			}
			t := vName(sc, lang, a.Actor) + " " + vp[render.VerbClass(a.Verb)]
			if len(ts) > 0 {
				t += " (toward " + strings.Join(ts, ", ") + ")"
			}
			t += ": " + f.Text
			if !f.True {
				t += " [a claim or belief; it need not be true]"
			}
			v.Expect = append(v.Expect, vItem{fmt.Sprintf("E%d", len(v.Expect)+1), k, t})
		}
	}
	seen := map[string]bool{}
	for _, id := range sc.Withheld {
		if c := sc.Canon(id); !seen[c] {
			seen[c] = true
			f, _ := sc.FactByID(id)
			v.Withheld = append(v.Withheld, vItem{fmt.Sprintf("W%d", len(v.Withheld)+1), c, f.Text})
		}
	}
	return v
}

func s2Prompt(v vSet) (string, string) {
	sys := `You check a piece of prose against a checklist. Judge only from what the prose says or implies to a reader who does not know the source story; ignore your knowledge of any original work.
For each item: answer "yes" if the prose states or clearly implies it (including through how a character speaks, addresses someone, admits something, or thinks), otherwise "no". For "yes" give the shortest supporting quote copied from the prose; for "no" leave quote empty.
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"items":[{"id":"E1","answer":"yes","quote":"..."}]} with one entry for every id below.`
	var b strings.Builder
	b.WriteString("## Prose\n" + v.Prose + "\n\n## Expected facts: does the prose state or clearly imply each?\n")
	for _, i := range v.Expect {
		b.WriteString(i.ID + ". " + i.Text + "\n")
	}
	b.WriteString("\n## Withheld facts: does the prose state or clearly imply each?\n")
	for _, i := range v.Withheld {
		b.WriteString(i.ID + ". " + i.Text + "\n")
	}
	return sys, b.String()
}

type s2Out struct {
	Items []struct {
		ID     string `json:"id"`
		Answer string `json:"answer"`
		Quote  string `json:"quote"`
	} `json:"items"`
}

func parseS2(text string) (map[string]bool, error) {
	body := text
	if i := strings.Index(text, "```json"); i >= 0 {
		rest := text[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 {
			body = rest[:j]
		}
	}
	var o s2Out
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &o); err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for _, it := range o.Items {
		m[it.ID] = strings.EqualFold(it.Answer, "yes")
	}
	return m, nil
}

type callRec struct {
	Pair, Raw string
	Err       string `json:",omitempty"`
}

func callFile(out, mode, pair string, i int) string {
	return filepath.Join(out, fmt.Sprintf("%s_%s_%d.json", mode, strings.ReplaceAll(pair, "/", "_"), i))
}

func runVerifier(ctx context.Context, h string, budget int, out string, scs []scene, bs []render.Backend, only string) {
	vPairs := vPairs
	if only != "" {
		vPairs = strings.Split(only, ",")
	}
	_ = os.MkdirAll(out, 0o755)
	lf, _ := os.OpenFile(filepath.Join(out, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer lf.Close()
	used, n := 0, 0
	for _, p := range vPairs {
		sc, b := findPair(scs, bs, p)
		prose, _ := readP1(p)
		v := buildSet(sc, b.Lang, p, prose)
		if h == "dump" {
			j, _ := json.MarshalIndent(v, "", "  ")
			_ = os.WriteFile(filepath.Join(out, "expected_"+strings.ReplaceAll(p, "/", "_")+".json"), j, 0o644)
			continue
		}
		xm := render.NewMeter(counter{render.NewClaudeCLIWith(sonnet, "low"), &n, &used, budget}, lf).Label(h + "-" + p)
		sys, user := render.BuildExtractPrompt(sc, b.Lang, prose)
		if h == "s2" {
			sys, user = s2Prompt(v)
		}
		for i := 1; i <= reps; i++ {
			f := callFile(out, h, p, i)
			if _, err := os.Stat(f); err == nil {
				continue
			}
			res, err := xm.Complete(ctx, sys, user)
			r := callRec{Pair: p, Raw: res.Text}
			if err != nil {
				fmt.Fprintln(os.Stderr, p, i, "ERR", err)
				if err == errBudget {
					return
				}
				r.Err = err.Error()
			}
			j, _ := json.MarshalIndent(r, "", "  ")
			_ = os.WriteFile(f, j, 0o644)
			fmt.Fprintln(os.Stderr, h, p, i, "used", used)
		}
	}
	fmt.Fprintln(os.Stderr, h, "calls", used)
}

func findPair(scs []scene, bs []render.Backend, p string) (render.Scene, render.Backend) {
	for _, sc := range scs {
		for _, b := range bs {
			if p == sc.work+"/"+sc.s.ID+"."+b.ID {
				return sc.s, b
			}
		}
	}
	panic("pair not found: " + p)
}

// ---- report

type truth map[string]map[string]struct {
	Present bool
	Leaked  bool
	Quote   string
	Unsure  bool
}

// verdict: 한 판정(투표 또는 6회 다수)의 항목별 답(E*: 있음, W*: 누설)과 보고 집합
type verdict struct {
	Ans                 map[string]bool
	Missing, Hall, Leak []string
	Pass                bool
}

func s1Verdict(sc render.Scene, v vSet, xs []render.Extracted) verdict {
	rep := render.Diff(sc, render.Vote(sc, xs))
	d := verdict{Ans: map[string]bool{}, Missing: rep.Missing, Hall: rep.Hallucinated, Leak: rep.Leaked}
	d.Pass = render.Passed(render.Render{Report: rep})
	miss := map[string]bool{}
	for _, m := range rep.Missing {
		miss[m] = true
	}
	for _, i := range v.Expect {
		d.Ans[i.ID] = !miss[i.Key]
	}
	lk := map[string]bool{}
	for _, l := range rep.Leaked {
		lk[sc.Canon(l)] = true
	}
	for _, i := range v.Withheld {
		d.Ans[i.ID] = lk[i.Key]
	}
	return d
}

func s2Verdict(v vSet, ms []map[string]bool) verdict {
	d := verdict{Ans: map[string]bool{}, Missing: []string{}, Hall: []string{}, Leak: []string{}}
	need := len(ms)/2 + 1 // 6개면 4, 3개면 2
	for _, i := range append(append([]vItem{}, v.Expect...), v.Withheld...) {
		y := 0
		for _, m := range ms {
			if m[i.ID] {
				y++
			}
		}
		d.Ans[i.ID] = y >= need
		if i.ID[0] == 'E' && y < need {
			d.Missing = append(d.Missing, i.ID)
		}
		if i.ID[0] == 'W' && y >= need {
			d.Leak = append(d.Leak, i.ID)
		}
	}
	d.Pass = len(d.Missing) <= render.MaxMissing && len(d.Leak) == 0
	return d
}

func eq(a, b []string) bool {
	x, y := append([]string{}, a...), append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00")
}

func runReport(out string, scs []scene, bs []render.Backend, only string) {
	vPairs := vPairs
	if only != "" {
		vPairs = strings.Split(only, ",")
	}
	var tr truth
	tb, err := os.ReadFile(filepath.Join(out, "truth.json"))
	if err != nil || json.Unmarshal(tb, &tr) != nil {
		fmt.Println("truth.json missing/invalid")
		return
	}
	for _, mode := range []string{"s1", "s2"} {
		var agreeSum, accMaj, accVote, nItems, nVotes float64
		var passSame, missSame, hallSame, leakSame, nPairs int
		fmt.Printf("\n### %s\n", mode)
		for _, p := range vPairs {
			sc, b := findPair(scs, bs, p)
			prose, _ := readP1(p)
			v := buildSet(sc, b.Lang, p, prose)
			var xs []render.Extracted
			var ms []map[string]bool
			var single []verdict
			for i := 1; i <= reps; i++ {
				raw, _ := os.ReadFile(callFile(out, mode, p, i))
				var r callRec
				_ = json.Unmarshal(raw, &r)
				if mode == "s1" {
					e, err := render.ParseExtracted(r.Raw)
					if err != nil {
						fmt.Println("parse fail", p, i, err)
						e = render.Extracted{Acts: []render.ExtractedAct{}, FactsStated: []string{}}
					}
					xs = append(xs, e)
					single = append(single, s1Verdict(sc, v, []render.Extracted{e}))
				} else {
					m, err := parseS2(r.Raw)
					if err != nil {
						fmt.Println("parse fail", p, i, err)
						m = map[string]bool{}
					}
					ms = append(ms, m)
					single = append(single, s2Verdict(v, []map[string]bool{m}))
				}
			}
			vd := func(lo, hi int) verdict {
				if mode == "s1" {
					return s1Verdict(sc, v, xs[lo:hi])
				}
				return s2Verdict(v, ms[lo:hi])
			}
			A, B, M := vd(0, 3), vd(3, 6), vd(0, 6)
			fmt.Printf("\n%s\n| id | 항목 | 정답 | x/6 | 다수6 | 투표A | 투표B |\n|---|---|---|---|---|---|---|\n", p)
			for _, it := range append(append([]vItem{}, v.Expect...), v.Withheld...) {
				t := tr[p][it.ID]
				gt := t.Present
				if it.ID[0] == 'W' {
					gt = t.Leaked
				}
				x := 0
				for _, s := range single {
					if s.Ans[it.ID] {
						x++
					}
				}
				f := float64(x) / reps
				agreeSum += max(f, 1-f)
				nItems++
				if M.Ans[it.ID] == gt {
					accMaj++
				}
				for _, w := range []verdict{A, B} {
					nVotes++
					if w.Ans[it.ID] == gt {
						accVote++
					}
				}
				mark := ""
				if t.Unsure {
					mark = "?"
				}
				fmt.Printf("| %s | %.50s | %v%s | %d/6 | %v | %v | %v |\n", it.ID, it.Text, gt, mark, x, M.Ans[it.ID], A.Ans[it.ID], B.Ans[it.ID])
			}
			nPairs++
			if A.Pass == B.Pass {
				passSame++
			}
			if eq(A.Missing, B.Missing) {
				missSame++
			}
			if eq(A.Hall, B.Hall) {
				hallSame++
			}
			if eq(A.Leak, B.Leak) {
				leakSame++
			}
			fmt.Printf("투표A pass=%v miss=%v hall=%v leak=%v\n투표B pass=%v miss=%v hall=%v leak=%v\n", A.Pass, A.Missing, A.Hall, A.Leak, B.Pass, B.Missing, B.Hall, B.Leak)
		}
		fmt.Printf("\n%s 요약: 항목일치 %.1f%% / 정확도(6다수) %.1f%% / 정확도(K3 투표) %.1f%% / pass 일치 %d/%d / missing 일치 %d/%d / hall 일치 %d/%d / leak 일치 %d/%d\n",
			mode, 100*agreeSum/nItems, 100*accMaj/nItems, 100*accVote/nVotes, passSame, nPairs, missSame, nPairs, hallSame, nPairs, leakSame, nPairs)
	}
}
