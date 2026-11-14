// dieg is the Diegesis compiler CLI. PG makes the judgments; this only calls and prints.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/starthalf51/diegesis-lang/db"
	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// version is SemVer. Bump it together with the git tag (v0.2.0) when releasing. Until 1.0 the grammar may change.
var version = "0.16.0"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Println("dieg", version)
		return
	}
	if len(args) >= 2 && args[0] == "db" && args[1] == "migrate" {
		if err := migrate(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 0 {
		os.Exit(usage())
	}
	if args[0] == "check" || args[0] == "build" {
		os.Exit(cmdCompile(args))
	}
	if args[0] == "test" {
		os.Exit(cmdTest(args[1:]))
	}
	if args[0] == "render" {
		os.Exit(cmdRender(args[1:]))
	}
	if args[0] == "render-check" {
		os.Exit(cmdRenderCheck(args[1:]))
	}
	if args[0] == "write" {
		os.Exit(cmdWrite(args[1:]))
	}
	if args[0] == "convert" {
		os.Exit(cmdConvert(args[1:]))
	}
	if args[0] == "convert-book" {
		os.Exit(cmdConvertBook(args[1:]))
	}
	if args[0] == "continue" {
		os.Exit(cmdContinue(args[1:]))
	}
	if args[0] == "revise" {
		os.Exit(cmdRevise(args[1:]))
	}
	if args[0] == "stamp" {
		os.Exit(cmdStamp(args[1:]))
	}
	if args[0] == "demo" {
		os.Exit(cmdDemo(args[1:], os.Stdout))
	}
	if args[0] == "lint" {
		os.Exit(cmdLint(args[1:]))
	}
	if args[0] == "patterns" {
		os.Exit(cmdPatterns(args[1:]))
	}
	if args[0] == "curve" {
		os.Exit(cmdCurve(args[1:]))
	}
	if args[0] == "reader-state" {
		os.Exit(cmdReaderState(args[1:]))
	}
	if args[0] == "stats" {
		os.Exit(cmdStats(args[1:]))
	}
	if args[0] == "load" && len(args) == 2 {
		if _, err := load(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "dieg %s: not implemented\n", args[0])
	os.Exit(2)
}

// load reads ir.json, or for a directory compiles it as build does, loads it, and returns the work ID.
// It checks migrations every time (does nothing if already applied). Progress messages go to out.
func load(path string, out io.Writer) (string, error) {
	var w ir.Work
	if st, err := os.Stat(path); err != nil {
		return "", err
	} else if st.IsDir() {
		got, diags, err := compile(path)
		if err != nil {
			return "", err
		}
		if failed, _ := report(os.Stderr, diags, false); failed {
			return "", fmt.Errorf("%s: compile errors, nothing loaded", path)
		}
		w = *got
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := migrate(out); err != nil {
		return "", err
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer pool.Close()
	if err := pg.Load(ctx, pool, &w); err != nil {
		return "", err
	}
	fmt.Fprintln(out, "loaded", w.Works[0].ID)
	return w.Works[0].ID, nil
}

func migrate(out io.Writer) error {
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	dir, err := fs.Sub(db.FS, "migrations")
	if err != nil {
		return err
	}
	applied, err := pg.Migrate(ctx, pool, dir)
	for _, n := range applied {
		fmt.Fprintln(out, "applied", n)
	}
	if err == nil && len(applied) == 0 {
		fmt.Fprintln(out, "up to date")
	}
	return err
}
