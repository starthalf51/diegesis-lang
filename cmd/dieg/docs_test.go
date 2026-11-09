package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDocsExamplesCompile compiles every ```sl block in docs/ that is marked with
// <!-- check --> on the line before the fence. A block that starts with "module" is a whole
// file body; any other block gets a "module: docs.exN" front matter. No PG, no model.
func TestDocsExamplesCompile(t *testing.T) {
	files, _ := filepath.Glob("../../docs/*.md")
	re := regexp.MustCompile("(?s)<!-- check -->\n```sl\n(.*?)\n```")
	n := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range re.FindAllStringSubmatch(strings.ReplaceAll(string(src), "\r\n", "\n"), -1) {
			n++
			name := fmt.Sprintf("%s#%d", filepath.Base(f), i+1)
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				body := "---\nmodule: docs.ex\n---\n\n```sl\n" + m[1] + "\n```\n"
				if strings.HasPrefix(m[1], "module ") {
					body = "```sl\n" + m[1] + "\n```\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "ex.dieg.md"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
				_, diags, err := compile(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range diags {
					if d.Severity == "error" {
						t.Errorf("%s", d)
					}
				}
			})
		}
	}
	if n < 5 {
		t.Fatalf("only %d checked examples, want >= 5", n)
	}
}
