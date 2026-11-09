package main

// Scorecard spike: run driver (new verifier twice / old verifier K=3).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// scCompleter is replaced by a fake in tests. The default is the subscription claude CLI (sonnet/low).
var scCompleter = func() render.Completer { return render.NewClaudeCLIWith(sonnet, "low") }

func countLines(p string) int {
	b, _ := os.ReadFile(p)
	return bytes.Count(b, []byte("\n"))
}

func pick(only string) []vdef {
	if only == "" {
		return scVariants[:len(scVariants)-1] // extras only run when given a name
	}
	var o []vdef
	for _, n := range strings.Split(only, ",") {
		for _, v := range scVariants {
			if v.Name == n {
				o = append(o, v)
			}
		}
	}
	return o
}

func newEnv(ctx context.Context, out, label string, used *int, budget int, lf *os.File) *scEnv {
	n := 0
	m := render.NewMeter(counter{scCompleter(), &n, used, budget}, lf).Label(label)
	return &scEnv{ctx: ctx, c: m, m: m, out: filepath.Join(out, "calls"), budget: budget}
}

func runSC(ctx context.Context, mode string, budget int, out string, only string) {
	lp := filepath.Join(out, "calls.jsonl")
	lf, _ := os.OpenFile(lp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer lf.Close()
	used := countLines(lp) // the hard cap applies to all calls in this directory
	fmt.Fprintln(os.Stderr, mode, "calls so far", used, "budget", budget)
	switch mode {
	case "sc-gen":
		if err := scGen(ctx, out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	case "sc-report":
		scReport(ctx, out)
		return
	}
	for _, vd := range pick(only) {
		prose, t := readVariant(out, vd.Name)
		s, b, _, err := scBase(ctx, vd.Base)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		v := buildSet(s, b.Lang, vd.Base, prose)
		if mode == "sc-old" {
			f := filepath.Join(out, "old", vd.Name+".json")
			if _, err := os.Stat(f); err == nil {
				continue
			}
			e := newEnv(ctx, out, "old-"+vd.Name, &used, budget, lf)
			vote, raws, err := render.ExtractK(ctx, e.c, s, b.Lang, prose, 3)
			rec := map[string]any{"Variant": vd.Name, "Raws": raws}
			if err != nil {
				rec["Err"] = err.Error()
			} else {
				rep := render.Diff(s, vote)
				rec["Report"], rec["Passed"] = rep, render.Passed(render.Render{Report: rep})
			}
			u := e.m.Total()
			rec["Usage"], rec["Weighted"] = u, weighted(u)
			j, _ := json.MarshalIndent(rec, "", "  ")
			_ = os.WriteFile(f, j, 0o644)
			fmt.Fprintln(os.Stderr, "old", vd.Name, "used", used, "err", err)
			if err != nil {
				return
			}
			continue
		}
		for run := 1; run <= 2; run++ {
			f := filepath.Join(out, "runs", fmt.Sprintf("%s_r%d.json", vd.Name, run))
			if _, err := os.Stat(f); err == nil {
				continue
			}
			e := newEnv(ctx, out, fmt.Sprintf("sc-%s-r%d", vd.Name, run), &used, budget, lf)
			r := runScorecard(e, fmt.Sprintf("%s_r%d", vd.Name, run), s, b, prose, v, t.Target)
			r.Variant, r.Run = vd.Name, run
			j, _ := json.MarshalIndent(r, "", "  ")
			_ = os.WriteFile(f, j, 0o644)
			fmt.Fprintf(os.Stderr, "%s r%d calls=%d used=%d g1=%v g2=%v score=%.3f err=%q\n", vd.Name, run, r.Calls, used, r.G1Fail, r.G2Fail, r.Score, r.Err)
			if r.Err != "" {
				return // on error, stop without burning more
			}
		}
	}
}
