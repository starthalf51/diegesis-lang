// costcurve: zero-call analysis of examples/serial call logs. Reads files only; writes CSVs to -out.
// Cost = model factor x (input + 0.1*cache_read + 1.25*cache_write + 5*output); opus 1, sonnet 0.6, haiku 0.2.
// Model comes from the label exactly as tools/serial modelOf does (render- haiku, extract- sonnet, else opus).
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type call struct {
	Label           string
	In, Out, CR, CW float64
}

type step struct {
	Run, Arm, Name, Kind string
	Ep                   int
	Calls                int
	In, CR, CW, Out      float64
	Cost                 float64
	MaxPrompt            float64 // A: prompt of the ep-N call; B: largest code-call prompt
}

func factor(label string) float64 {
	switch {
	case strings.HasPrefix(label, "render-"):
		return 0.2
	case strings.HasPrefix(label, "extract-"):
		return 0.6
	}
	return 1
}

func readCalls(p string) []call {
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var cs []call
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]any
		json.Unmarshal(sc.Bytes(), &m)
		g := func(k string) float64 { v, _ := m[k].(float64); return v }
		l, _ := m["label"].(string)
		cs = append(cs, call{l, g("input"), g("output"), g("cache_read"), g("cache_write")})
	}
	return cs
}

// steps splits calls by the cumulative call counts in metrics.csv (the harness's own step boundaries).
// Calls after the last step (run1 A: 2 cut calls) are ignored.
func steps(root, run string) (out []step) {
	mf, _ := os.Open(filepath.Join(root, run, "metrics.csv"))
	rows, _ := csv.NewReader(mf).ReadAll()
	mf.Close()
	cs := map[string][]call{"A": readCalls(filepath.Join(root, run, "a", "calls.jsonl")), "B": readCalls(filepath.Join(root, run, "b", "calls.jsonl"))}
	prev := map[string]int{}
	for _, r := range rows[1:] {
		arm, name := r[1], r[0]
		n, _ := strconv.Atoi(r[2])
		s := step{Run: run, Arm: arm, Name: name, Kind: "revision"}
		if strings.HasPrefix(name, "ep") {
			s.Kind = "episode"
			s.Ep, _ = strconv.Atoi(name[2:])
		} else if name == "outline" {
			s.Kind = "outline"
		}
		for _, c := range cs[arm][prev[arm]:n] {
			s.Calls++
			s.In += c.In
			s.CR += c.CR
			s.CW += c.CW
			s.Out += c.Out
			s.Cost += factor(c.Label) * (c.In + 0.1*c.CR + 1.25*c.CW + 5*c.Out)
			p := c.In + c.CR + c.CW
			if (arm == "A" && strings.HasPrefix(c.Label, "ep-")) || (arm == "B" && strings.Contains(c.Label, "code") && p > s.MaxPrompt) {
				s.MaxPrompt = p
			}
		}
		prev[arm] = n
		out = append(out, s)
	}
	return
}

type fit struct {
	N               int
	B0, B1, R2, SE float64
}

func ols(x, y []float64) fit {
	n := float64(len(x))
	var mx, my float64
	for i := range x {
		mx += x[i] / n
		my += y[i] / n
	}
	var sxx, sxy, syy float64
	for i := range x {
		sxx += (x[i] - mx) * (x[i] - mx)
		sxy += (x[i] - mx) * (y[i] - my)
		syy += (y[i] - my) * (y[i] - my)
	}
	b1 := sxy / sxx
	b0 := my - b1*mx
	var sse float64
	for i := range x {
		e := y[i] - b0 - b1*x[i]
		sse += e * e
	}
	r2 := 0.0
	if syy > 0 {
		r2 = 1 - sse/syy
	}
	return fit{len(x), b0, b1, r2, math.Sqrt(sse / (n - 2) / sxx)}
}

// cum is the sum over e=1..N of max(b0+b1*e, 0).
func (f fit) cum(N int) (s float64) {
	for e := 1; e <= N; e++ {
		s += math.Max(f.B0+f.B1*float64(e), 0)
	}
	return
}

func main() {
	root := flag.String("root", "examples/serial", "")
	out := flag.String("out", "examples/value/cost-curve", "")
	flag.Parse()
	runs := []string{"run1", "run2", "run3"}
	var all []step
	for _, r := range runs {
		all = append(all, steps(*root, r)...)
	}
	g := func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) }
	csvOut := func(name string, rows [][]string) {
		f, _ := os.Create(filepath.Join(*out, name))
		defer f.Close()
		w := csv.NewWriter(f)
		w.WriteAll(rows)
	}

	rows := [][]string{{"run", "arm", "step", "kind", "calls", "input_uncached", "cache_read", "cache_write", "output", "cost_weighted", "max_prompt_tokens"}}
	for _, s := range all {
		rows = append(rows, []string{s.Run, s.Arm, s.Name, s.Kind, strconv.Itoa(s.Calls), g(s.In), g(s.CR), g(s.CW), g(s.Out), g(s.Cost), g(s.MaxPrompt)})
	}
	csvOut("steps.csv", rows)

	fits := map[string]fit{}
	fitOf := func(key string, sel func(step) bool, val func(step) float64) {
		var x, y []float64
		for _, s := range all {
			if s.Kind == "episode" && sel(s) {
				x = append(x, float64(s.Ep))
				y = append(y, val(s))
			}
		}
		fits[key] = ols(x, y)
	}
	var keys []string
	for _, arm := range []string{"A", "B"} {
		for _, r := range append([]string{"all"}, runs...) {
			a, rr := arm, r
			fitOf(a+"/"+rr, func(s step) bool { return s.Arm == a && (rr == "all" || s.Run == rr) }, func(s step) float64 { return s.Cost })
			keys = append(keys, a+"/"+rr)
		}
		a := arm
		fitOf(a+"/prompt", func(s step) bool { return s.Arm == a }, func(s step) float64 { return s.MaxPrompt })
		keys = append(keys, a+"/prompt")
	}
	rows = [][]string{{"series", "n", "intercept", "slope_per_episode", "slope_se", "r2"}}
	for _, k := range keys {
		x := fits[k]
		rows = append(rows, []string{k, strconv.Itoa(x.N), g(x.B0), fmt.Sprintf("%.1f", x.B1), fmt.Sprintf("%.1f", x.SE), fmt.Sprintf("%.3f", x.R2)})
	}
	csvOut("fits.csv", rows)

	rows = [][]string{{"fit", "N", "cumA", "cumB", "B_over_A"}}
	for _, r := range append([]string{"all"}, runs...) {
		for _, N := range []int{12, 24, 50, 100} {
			a, b := fits["A/"+r].cum(N), fits["B/"+r].cum(N)
			rows = append(rows, []string{r, strconv.Itoa(N), g(a), g(b), fmt.Sprintf("%.3f", b/a)})
		}
	}
	// Hypothetical A-full: every episode prompt carries all earlier episode text (o tokens each) instead of
	// summary + previous episode. Extra input per episode e = w*o*(e-1); w=0.1 (all cache read) or 1.25 (all cache write).
	var osum, on, p1sum, p1n float64
	for _, r := range runs {
		for _, c := range readCalls(filepath.Join(*root, r, "a", "calls.jsonl")) {
			if strings.HasPrefix(c.Label, "ep-") {
				osum += c.Out
				on++
			}
			if c.Label == "ep-1" {
				p1sum += c.In + c.CR + c.CW
				p1n++
			}
		}
	}
	o, p1 := osum/on, p1sum/p1n
	fmt.Printf("mean ep output %.0f tokens, ep-1 prompt %.0f, A-full crosses 200k at episode %.1f\n", o, p1, 1+(200000-p1)/o)
	fmt.Printf("current policy crosses 200k: A at ep %.0f, B at ep %.0f\n", (200000-fits["A/prompt"].B0)/fits["A/prompt"].B1, (200000-fits["B/prompt"].B0)/fits["B/prompt"].B1)
	for _, w := range []float64{0.1, 1.25} {
		for _, N := range []int{12, 24, 50, 100} {
			extra := 0.0
			for e := 1; e <= N; e++ {
				extra += w * o * float64(e-1)
			}
			a, b := fits["A/all"].cum(N)+extra, fits["B/all"].cum(N)
			rows = append(rows, []string{fmt.Sprintf("A-full(w=%.2f)", w), strconv.Itoa(N), g(a), g(b), fmt.Sprintf("%.3f", b/a)})
		}
	}
	csvOut("projection.csv", rows)

	rows = [][]string{{"run", "ep", "digest_bytes"}}
	for _, r := range runs {
		for e := 2; e <= 12; e++ {
			if st, err := os.Stat(filepath.Join(*root, r, "b", fmt.Sprintf("digest-ep%02d.txt", e))); err == nil {
				rows = append(rows, []string{r, strconv.Itoa(e), strconv.FormatInt(st.Size(), 10)})
			}
		}
	}
	csvOut("digest.csv", rows)

	// A ep-call own output tokens and prompt size, per episode (for the full-text hypothetical)
	rows = [][]string{{"run", "label", "prompt_tokens", "output_tokens"}}
	for _, r := range runs {
		for _, c := range readCalls(filepath.Join(*root, r, "a", "calls.jsonl")) {
			if strings.HasPrefix(c.Label, "ep-") {
				rows = append(rows, []string{r, c.Label, g(c.In + c.CR + c.CW), g(c.Out)})
			}
		}
	}
	csvOut("a_ep_calls.csv", rows)

	for _, k := range keys {
		x := fits[k]
		fmt.Printf("%-10s n=%d b0=%.0f b1=%.1f se=%.1f r2=%.3f\n", k, x.N, x.B0, x.B1, x.SE, x.R2)
	}
	for _, r := range append([]string{"all"}, runs...) {
		fmt.Print(r, " ")
		for _, N := range []int{12, 24, 50, 100} {
			fmt.Printf("N=%d %.3f  ", N, fits["B/"+r].cum(N)/fits["A/"+r].cum(N))
		}
		fmt.Println()
	}
}
