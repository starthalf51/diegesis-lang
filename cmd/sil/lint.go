package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"silmari/internal/pg"
	"silmari/rules"
)

// cmdLint runs `sil lint <work|dir|ir.json> [--json]`. A dir or ir.json is
// built and loaded first; a bare name is a work already loaded. Exit 1 when
// any unwaived warn remains.
func cmdLint(args []string) int {
	var target string
	asJSON := false
	for _, a := range args {
		switch {
		case a == "--json":
			asJSON = true
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" {
		return usage()
	}
	work, base := target, ""
	if st, err := os.Stat(target); err == nil {
		if work, err = load(target, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "sil:", err)
			return 1
		}
		if st.IsDir() {
			base = target // IR 의 파일 경로는 디렉터리 기준이다
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sil:", err)
		return 1
	}
	defer pool.Close()
	ds, err := pg.Lint(ctx, pool, work, rules.FS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sil:", err)
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
