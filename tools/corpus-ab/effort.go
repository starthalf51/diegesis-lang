package main

// -spike effort: render effort A/B (Plan 6 Task 5). 6 scenes × {medium, low}, haiku render + card verification (RenderVerified2, Retries 2).
// Run medium → low interleaved per scene so it stays fair even if the budget is cut off. The budget guard comes before each call (counter).
//   go run ./tools/corpus-ab -spike effort -budget 60 -out examples/effort-ab      (real calls)
//   go run ./tools/corpus-ab -spike effort-dry                                      (fake completer, 0 calls)
//   go run ./tools/corpus-ab -spike effort-report -out examples/effort-ab           (no calls, table and verdict only)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

// effortCompleter is replaced by a fake in the dry run.
var effortCompleter = func(model, effort string) render.Completer { return render.NewClaudeCLIWith(model, effort) }

type effScene struct {
	Name, Work string
	S          render.Scene
}

type effRun struct {
	Scene, Effort          string
	Done                   bool // finished without being cut off by budget or error
	Pass                   bool
	Attempts               int
	Score, S1, S2, S3      float64
	RenderCalls, CardCalls int
	RenderOut              int64
	RenderOutPerCall       float64
	Weighted               float64
	Missing, Leaked        []string
	ExtractError, Err      string `json:",omitempty"`
}

var effCorpus = [][2]string{{"bible", "mark_denial"}, {"chibi", "45.88"}, {"macbeth", "1.3"}, {"odyssey", "19.1"}}

// effLoad: an already loaded work is not loaded again (avoids work_pkey duplicates and needless writes). The dry run does not load.
func effLoad(ctx context.Context, load bool) ([]effScene, error) {
	pool, err := pg.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	have := func(w string) bool {
		var n int
		return pool.QueryRow(ctx, `SELECT count(*) FROM work WHERE id=$1`, w).Scan(&n) == nil && n > 0
	}
	for _, wd := range [][2]string{{"bible", "corpus/bible"}, {"chibi", "corpus/chibi"}, {"macbeth", "corpus/macbeth"}, {"odyssey", "corpus/odyssey"}, {"haejin", "examples/serial/run1/b/story"}} {
		if have(wd[0]) {
			continue
		}
		if !load {
			return nil, fmt.Errorf("work %s not in PG (dry-run does not load)", wd[0])
		}
		fmt.Fprintln(os.Stderr, "loading", wd[0])
		cmd := exec.CommandContext(ctx, "go", "run", "./cmd/dieg", "load", wd[1])
		cmd.Stderr = os.Stderr
		if _, err := cmd.Output(); err != nil {
			return nil, fmt.Errorf("load %s: %w", wd[1], err)
		}
	}
	get := func(w, id string) (render.Scene, error) {
		b, err := pg.RenderScene(ctx, pool, w, id)
		if err != nil {
			return render.Scene{}, err
		}
		return render.ParseScene(b)
	}
	var out []effScene
	for _, ws := range effCorpus {
		s, err := get(ws[0], ws[1])
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", ws[0], ws[1], err)
		}
		out = append(out, effScene{ws[0] + "/" + ws[1], ws[0], s})
	}
	rows, err := pool.Query(ctx, `SELECT id FROM scene WHERE work_id = 'haejin' AND shown ORDER BY discourse_ord, id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	var ser []effScene
	for _, id := range ids {
		s, err := get("haejin", id)
		if err != nil {
			return nil, err
		}
		s.FreeDialogue = true // a serial story uses skeleton mode
		ser = append(ser, effScene{"haejin/" + filepath.Base(id), "haejin", s})
	}
	sort.SliceStable(ser, func(i, j int) bool { return acts(ser[i].S) > acts(ser[j].S) })
	if len(ser) < 2 {
		return nil, fmt.Errorf("haejin: %d shown scenes", len(ser))
	}
	return append(out, ser[:2]...), nil
}

func effArm(ctx context.Context, sc effScene, b render.Backend, effort string, used *int, budget int, lf *os.File, out string) (effRun, bool) {
	r := effRun{Scene: sc.Name, Effort: effort}
	var rn, xn int
	rm := render.NewMeter(counter{effortCompleter(haiku, effort), &rn, used, budget}, lf).Label("render-" + sc.Name + "-" + effort)
	xm := render.NewMeter(counter{effortCompleter(sonnet, "low"), &xn, used, budget}, lf).Label("card-" + sc.Name + "-" + effort)
	rr, n, err := render.RenderVerified2(ctx, render.Completers{Render: rm, Extract: xm}, sc.Work, sc.S, b, render.VerifyOpts{K: 1, Retries: 2, Verifier: "card", Card: render.CardOpts{}})
	r.Attempts, r.RenderCalls, r.CardCalls = n, rn, xn
	ru, xu := rm.Total(), xm.Total()
	r.RenderOut = ru.Output
	if rn > 0 {
		r.RenderOutPerCall = float64(ru.Output) / float64(rn)
	}
	r.Weighted = weighted(ru) + weighted(xu)
	r.Pass = err == nil && render.Passed(rr)
	rep := rr.Report
	r.Missing, r.Leaked, r.ExtractError = rep.Missing, rep.Leaked, rep.ExtractError
	if rep.Scores != nil {
		r.Score, r.S1, r.S2, r.S3 = rep.Scores.Score, rep.Scores.Coverage, rep.Scores.Kind, rep.Scores.Tone
	}
	r.Done = err == nil
	if err != nil {
		r.Err = err.Error()
	}
	j, _ := json.MarshalIndent(r, "", "  ")
	base := filepath.Join(out, "runs", strings.ReplaceAll(sc.Name, "/", "_")+"."+effort)
	_ = os.WriteFile(base+".json", append(j, '\n'), 0o644)
	if p := strings.TrimSpace(rr.Prose); p != "" {
		_ = os.WriteFile(base+".md", []byte(p+"\n"), 0o644)
	}
	return r, errors.Is(err, errBudget)
}

func runEffort(ctx context.Context, dry bool, budget int, out string) {
	if dry {
		out = "examples/effort-ab-dry"
		_ = os.RemoveAll(out)
		defer os.RemoveAll(out)
		f := &fakeC{}
		effortCompleter = func(model, _ string) render.Completer { return fakeEff{f, model == haiku} }
	}
	scs, err := effLoad(ctx, !dry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bs, err := render.LoadBackends("styles")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var b render.Backend
	for _, x := range bs {
		if x.ID == "ko-haera" {
			b = x
		}
	}
	_ = os.MkdirAll(filepath.Join(out, "runs"), 0o755)
	lp := filepath.Join(out, "calls.jsonl")
	lf, _ := os.OpenFile(lp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer lf.Close()
	used := countLines(lp) // the hard cap applies to all calls in this directory
	fmt.Fprintln(os.Stderr, "effort: calls so far", used, "budget", budget)
loop:
	for _, sc := range scs {
		for _, e := range []string{"medium", "low"} {
			if used >= budget {
				fmt.Fprintln(os.Stderr, "budget reached before", sc.Name, e)
				break loop
			}
			r, cut := effArm(ctx, sc, b, e, &used, budget, lf, out)
			fmt.Fprintf(os.Stderr, "%-26s %-6s done=%v pass=%v att=%d score=%.3f render=%d card=%d used=%d %s\n", sc.Name, e, r.Done, r.Pass, r.Attempts, r.Score, r.RenderCalls, r.CardCalls, used, r.Err)
			if !r.Done || cut { // error or budget: stop without burning more
				break loop
			}
		}
	}
	effReport(out, used)
	if dry {
		fmt.Println("dry-run ok; fake calls:", used)
	}
}

// fakeEff: the render model returns fixed prose, everything else is the existing fake scorer.
type fakeEff struct {
	f      *fakeC
	render bool
}

func (c fakeEff) Complete(ctx context.Context, sys, user string) (render.Result, error) {
	if c.render {
		return render.Result{Text: "그는 말했다.", StopReason: "end_turn", Usage: render.Usage{Input: 100, Output: 40}}, nil
	}
	return c.f.Complete(ctx, sys, user)
}

func effReport(out string, calls int) {
	fs, _ := filepath.Glob(filepath.Join(out, "runs", "*.json"))
	by := map[string]map[string]effRun{}
	var order []string
	for _, f := range fs {
		var r effRun
		raw, _ := os.ReadFile(f)
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		if by[r.Scene] == nil {
			by[r.Scene] = map[string]effRun{}
			order = append(order, r.Scene)
		}
		by[r.Scene][r.Effort] = r
	}
	rank := map[string]int{"bible/mark_denial": 0, "chibi/45.88": 1, "macbeth/1.3": 2, "odyssey/19.1": 3}
	sort.SliceStable(order, func(i, j int) bool {
		a, ok1 := rank[order[i]]
		b, ok2 := rank[order[j]]
		if !ok1 {
			a = 9
		}
		if !ok2 {
			b = 9
		}
		return a < b || (a == b && order[i] < order[j])
	})
	var sb strings.Builder
	sb.WriteString("| scene | effort | pass | attempts | Score | S1 | S2 | S3 | render output tokens/call | render calls | scoring calls | weighted tokens |\n|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	var n int
	var dScore, pm, pl float64
	var om, ol, cm, cl int64
	for _, sc := range order {
		for _, e := range []string{"medium", "low"} {
			r, ok := by[sc][e]
			if !ok {
				fmt.Fprintf(&sb, "| %s | %s | (not run) | | | | | | | | | |\n", sc, e)
				continue
			}
			note := ""
			if !r.Done {
				note = " (cut off)"
			}
			fmt.Fprintf(&sb, "| %s | %s%s | %s | %d | %.3f | %.2f | %.2f | %.2f | %.0f | %d | %d | %.0f |\n", sc, e, note, map[bool]string{true: "✓", false: "✗"}[r.Pass], r.Attempts, r.Score, r.S1, r.S2, r.S3, r.RenderOutPerCall, r.RenderCalls, r.CardCalls, r.Weighted)
		}
		m, l := by[sc]["medium"], by[sc]["low"]
		if m.Done && l.Done {
			n++
			dScore += m.Score - l.Score
			if m.Pass {
				pm++
			}
			if l.Pass {
				pl++
			}
			om, ol, cm, cl = om+m.RenderOut, ol+l.RenderOut, cm+int64(m.RenderCalls), cl+int64(l.RenderCalls)
		}
	}
	verdict := "medium"
	if n > 0 {
		dScore /= float64(n)
		perM, perL := float64(om)/float64(max(cm, 1)), float64(ol)/float64(max(cl, 1))
		if dScore <= 0.02 && pl >= pm && perL < perM {
			verdict = "low"
		}
		fmt.Fprintf(&sb, "\ncompared scenes n=%d (only scenes finished in both arms)\n- mean Score drop (medium−low): %.4f (criterion ≤ 0.02)\n- passing scenes: medium %.0f, low %.0f (criterion: low ≥ medium)\n- render output tokens/call (total): medium %.1f, low %.1f (criterion: low < medium)\n", n, dScore, pm, pl, perM, perL)
	} else {
		sb.WriteString("\nno comparable scenes\n")
	}
	fmt.Fprintf(&sb, "\ndefault effort: %s\nactual calls: %d\n", verdict, calls)
	_ = os.WriteFile(filepath.Join(out, "table.md"), []byte(sb.String()), 0o644)
	fmt.Print(sb.String())
}
