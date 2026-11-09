package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
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

// TestDocsLinks checks that every relative link in docs/ and the root README resolves, including
// #anchors (GitHub heading slugs).
func TestDocsLinks(t *testing.T) {
	files, _ := filepath.Glob("../../docs/*.md")
	files = append(files, "../../README.md")
	link := regexp.MustCompile(`\]\(([^)\s]+)\)`)
	slugs := map[string]map[string]bool{}
	slugsOf := func(p string) map[string]bool {
		if s, ok := slugs[p]; ok {
			return s
		}
		s := map[string]bool{}
		slugs[p] = s
		b, err := os.ReadFile(p)
		if err != nil {
			return s
		}
		fence := false
		for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if strings.HasPrefix(l, "```") {
				fence = !fence
			}
			if fence || !strings.HasPrefix(l, "#") {
				continue
			}
			h := strings.ToLower(strings.TrimSpace(strings.TrimLeft(l, "#")))
			var sb strings.Builder
			for _, r := range h {
				switch {
				case r == ' ':
					sb.WriteByte('-')
				case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
					sb.WriteRune(r)
				}
			}
			s[sb.String()] = true
		}
		return s
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range link.FindAllStringSubmatch(string(b), -1) {
			u := m[1]
			if strings.Contains(u, "://") || strings.HasPrefix(u, "mailto:") {
				continue
			}
			path, anchor, _ := strings.Cut(u, "#")
			target := f
			if path != "" {
				target = filepath.Join(filepath.Dir(f), path)
			}
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: broken link %s", filepath.Base(f), u)
				continue
			}
			if anchor != "" && strings.HasSuffix(target, ".md") && !slugsOf(target)[anchor] {
				t.Errorf("%s: missing anchor %s", filepath.Base(f), u)
			}
		}
	}
}
