package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"story-lang/internal/ir"
	"story-lang/internal/sema"
	"story-lang/internal/syntax"
)

// compile parses every .sl / .sl.md under dir and builds the IR. Paths in
// the IR are relative to dir; diags carry dir-joined paths for humans.
func compile(dir string) (*ir.Work, []syntax.Diag, error) {
	var files []*syntax.File
	var diags []syntax.Diag
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(p, ".sl.md") || strings.HasSuffix(p, ".sl")) {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		f, ds := syntax.ParseFile(filepath.ToSlash(rel), src)
		files, diags = append(files, f), append(diags, ds...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	w, ds := sema.Build(files)
	diags = append(diags, ds...)
	for i := range diags {
		diags[i].File = filepath.ToSlash(filepath.Join(dir, diags[i].File))
	}
	return w, diags, nil
}

// report prints diags and says whether any is an error.
func report(out io.Writer, diags []syntax.Diag, asJSON bool) (bool, error) {
	failed := false
	for _, d := range diags {
		failed = failed || d.Severity == "error"
	}
	if asJSON {
		if diags == nil {
			diags = []syntax.Diag{}
		}
		b, err := json.MarshalIndent(diags, "", "  ")
		if err != nil {
			return failed, err
		}
		_, err = fmt.Fprintf(out, "%s\n", b)
		return failed, err
	}
	for _, d := range diags {
		fmt.Fprintln(out, d)
	}
	return failed, nil
}

// cmdCompile runs `slc check <dir> [--json]` (exit 1 on any error) and
// `slc build <dir> -o ir.json` (writes nothing when there are errors).
func cmdCompile(args []string) int {
	var dir, outPath string
	asJSON := false
	for i := 1; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--json":
			asJSON = true
		case a == "-o" && i+1 < len(args):
			i++
			outPath = args[i]
		case dir == "" && !strings.HasPrefix(a, "-"):
			dir = a
		default:
			return usage()
		}
	}
	if dir == "" || (args[0] == "build") != (outPath != "") {
		return usage()
	}
	w, diags, err := compile(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slc:", err)
		return 1
	}
	diagOut := io.Writer(os.Stdout)
	if args[0] == "build" {
		diagOut = os.Stderr
	}
	failed, err := report(diagOut, diags, asJSON)
	if err != nil || failed {
		return 1
	}
	if args[0] == "build" {
		b, err := json.MarshalIndent(w, "", "  ")
		if err == nil {
			err = os.WriteFile(outPath, append(b, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "slc:", err)
			return 1
		}
	}
	return 0
}

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: slc db migrate | check <dir> [--json] | build <dir> -o ir.json | load <ir.json|dir> | lint <work|dir|ir.json> [--json] | test <corpus-dir> [--seed N] [--mutants N] | render <work|dir> <scene> --backend <id> [-o file] | render-check <corpus...> [--force]")
	return 2
}
