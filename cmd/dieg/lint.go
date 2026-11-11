package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

// cmdLint runs `dieg lint <work|dir|ir.json> [--json] [--rules <dir>]`. A dir or ir.json is
// built and loaded first; a bare name is a work already loaded. Exit 1 when
// any unwaived warn remains.
func cmdLint(args []string) int {
	var target string
	var rulesDir string
	asJSON := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			asJSON = true
		case a == "--rules" && i+1 < len(args):
			i++
			rulesDir = args[i]
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" {
		return usage()
	}
	var extra []fs.FS
	if rulesDir != "" {
		// validated before the database is touched: headers, at least one rule, no ID collisions with the built-ins
		if st, err := os.Stat(rulesDir); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "dieg: --rules %s: not a directory\n", rulesDir)
			return 1
		}
		x := os.DirFS(rulesDir)
		bundled, err := pg.Rules(rules.FS)
		if err == nil {
			var more []pg.Rule
			if more, err = pg.Rules(x); err == nil {
				if len(more) == 0 {
					err = fmt.Errorf("--rules %s: no rules (*.sql) found", rulesDir)
				} else {
					_, err = pg.MergeRules(bundled, more)
				}
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		extra = append(extra, x)
	}
	work, base := target, ""
	if st, err := os.Stat(target); err == nil {
		if work, err = load(target, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		if st.IsDir() {
			base = target // file paths in the IR are relative to the directory
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer pool.Close()
	ds, err := pg.Lint(ctx, pool, work, rules.FS, extra...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	failed := false
	for i := range ds {
		if base != "" && ds[i].File != "" {
			ds[i].File = filepath.ToSlash(filepath.Join(base, ds[i].File))
		}
		failed = failed || ds[i].Severity == "warn" && !ds[i].Waived
	}
	if asJSON {
		if ds == nil {
			ds = []pg.Diagnostic{}
		}
		b, _ := json.MarshalIndent(ds, "", "  ")
		fmt.Printf("%s\n", b)
	} else {
		for _, d := range ds {
			fmt.Println(d)
		}
	}
	if failed {
		return 1
	}
	return 0
}
