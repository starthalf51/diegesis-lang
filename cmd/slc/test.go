package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"story-lang/internal/accept"
	"story-lang/internal/pg"
	"story-lang/rules"
)

// cmdTest runs `slc test <corpus-dir> [--seed N] [--mutants N]`: compile, load, lint, the named
// queries and the mutation run, judged against <dir>/expect.yaml. The report goes to
// out/accept/<work>.md. Exit 0 only on PASS.
func cmdTest(args []string) int {
	var dir string
	seed, n := int64(1), 0
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case (a == "--seed" || a == "--mutants") && i+1 < len(args):
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return usage()
			}
			if a == "--seed" {
				seed = v
			} else {
				n = int(v)
			}
		case dir == "" && !strings.HasPrefix(a, "-"):
			dir = a
		default:
			return usage()
		}
	}
	if dir == "" {
		return usage()
	}
	r, err := runTest(dir, seed, n)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slc:", err)
		return 1
	}
	if !r.Pass {
		return 1
	}
	return 0
}

func runTest(dir string, seed int64, n int) (accept.Result, error) {
	b, err := os.ReadFile(filepath.Join(dir, "expect.yaml"))
	if err != nil {
		return accept.Result{}, err
	}
	e, err := accept.Parse(b)
	if err != nil {
		return accept.Result{}, err
	}
	w, cdiags, err := compile(dir)
	if err != nil {
		return accept.Result{}, err
	}
	in := accept.Input{}
	for _, d := range cdiags {
		if d.Severity == "error" {
			in.CompileErrors++
		}
		in.Diags = append(in.Diags, pg.Diagnostic{RunID: "compile", Rule: d.Rule, Severity: d.Severity, File: d.File,
			Line: d.Line, Col: d.Col, Message: d.Msg, Waived: d.Waived})
	}
	if in.CompileErrors == 0 {
		if w.Works[0].ID != e.Work {
			return accept.Result{}, fmt.Errorf("%s: work %q, expect.yaml says %q", dir, w.Works[0].ID, e.Work)
		}
		if err := migrate(io.Discard); err != nil {
			return accept.Result{}, err
		}
		ctx := context.Background()
		pool, err := pg.Connect(ctx)
		if err != nil {
			return accept.Result{}, err
		}
		defer pool.Close()
		if err := pg.Load(ctx, pool, w); err != nil {
			return accept.Result{}, err
		}
		ds, err := pg.Lint(ctx, pool, e.Work, rules.FS)
		if err != nil {
			return accept.Result{}, err
		}
		for i := range ds {
			if ds[i].File != "" {
				ds[i].File = filepath.ToSlash(filepath.Join(dir, ds[i].File))
			}
		}
		in.Diags = append(in.Diags, ds...)
		in.Rows, in.QueryErrors = accept.RunQueries(ctx, pool, e.Work, e.Queries)
		if in.Mutants, err = accept.RunMutants(ctx, pool, w, ds, seed, n, os.Stderr); err != nil {
			return accept.Result{}, err
		}
	}
	r := accept.Judge(e, in)
	out := filepath.Join("out", "accept", e.Work+".md")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return r, err
	}
	if err := os.WriteFile(out, []byte(accept.Report(e, in, r)), 0o644); err != nil {
		return r, err
	}
	for _, d := range in.Diags {
		fmt.Println(d)
	}
	for _, f := range r.Failures {
		fmt.Println("FAIL:", f)
	}
	verdict := "FAIL"
	if r.Pass {
		verdict = "PASS"
	}
	fmt.Printf("%s %s: compile errors %d, expected %d/%d, false positives %d, queries %d/%d, mutation %d/%d = %.2f (min %.2f) → %s\n",
		verdict, e.Work, in.CompileErrors, len(e.Expected)-len(r.Missing), len(e.Expected), len(r.FalsePositives),
		len(e.Queries)-len(r.BadQueries), len(e.Queries), r.Hit, r.Counted, r.Rate, e.MutationMinRate, out)
	return r, nil
}
