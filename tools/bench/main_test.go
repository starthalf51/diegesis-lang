package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchDryRun(t *testing.T) {
	ctx := context.Background()
	out := t.TempDir()
	defer func() {
		if fmt.Sprintf("%p", runB) == fmt.Sprintf("%p", fakeB) {
			t.Error("-dry left runB swapped to fakeB")
		}
	}()
	idea := filepath.Join(out, "idea.txt")
	os.WriteFile(idea, []byte("가짜 아이디어. 장면 3개."), 0o644)
	for _, c := range []string{"a", "b"} {
		if err := run(ctx, c, []string{"-dry", "-idea", idea, "-n", "3", "-out", out}, nil); err != nil {
			t.Fatal(c, err)
		}
	}
	// A resumes: a second run must not call again.
	if err := run(ctx, "a", []string{"-dry", "-idea", idea, "-n", "3", "-out", out}, nil); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, "judge", []string{"-dry", "-n", "3", "-out", out}, nil); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a/outline.md", "a/scenes/s3.md", "a/prompts/scene-3.txt", "a/calls.jsonl", "b/calls.jsonl",
		"judge/prompt.txt", "judge/run1.txt", "judge/mapping.txt", "judge/quotes.md", "judge/calls.jsonl"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Error(err)
		}
	}
	p, _ := os.ReadFile(filepath.Join(out, "a/prompts/scene-2.txt"))
	if !strings.Contains(string(p), "## 직전 장면 (장면 1)") || !strings.Contains(string(p), "장면 3개") {
		t.Errorf("scene prompt lacks previous scene or count:\n%s", p)
	}
	jp, _ := os.ReadFile(filepath.Join(out, "judge/prompt.txt"))
	if strings.Contains(string(jp), "첫 장면") || strings.Count(string(jp), "### 장면 ") != 5 {
		t.Errorf("judge prompt not blind or wrong scene count")
	}
	q, _ := os.ReadFile(filepath.Join(out, "judge/quotes.md"))
	if !strings.Contains(string(q), "(있음) 종이 울렸다.") || !strings.Contains(string(q), "(없음) 지어낸 인용") {
		t.Errorf("quote check:\n%s", q)
	}
	a, _ := ReadCalls(filepath.Join(out, "a/calls.jsonl"))
	if a.Calls != 4 || a.Output != 40 || In(a.Usage) != 4*152 {
		t.Errorf("A totals %+v", a)
	}
	var w bytes.Buffer
	if err := run(ctx, "sum", []string{out}, &w); err != nil {
		t.Fatal(err)
	}
	s := w.String()
	for _, want := range []string{"| A | 4 | 0 | 8 | 400 | 200 | 608 | 40 |", "| B | 5 | 0 | 10 | 500 | 250 | 760 | 50 |",
		"verify-failed=1 [s2]", "code-r1 in=152 out=10", "| s1 | 152 | 10 | 152 | 152 | 20 |", "| s3 | 152 | 10 | 0 | 0 | 0 |"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
}
