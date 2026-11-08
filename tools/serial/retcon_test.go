package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditDist(t *testing.T) {
	for _, c := range []struct {
		a, b string
		d    int
	}{{"", "", 0}, {"abc", "abc", 0}, {"abc", "", 3}, {"kitten", "sitting", 3}, {"가나다라", "가나라", 1}} {
		if g := editDist([]rune(c.a), []rune(c.b)); g != c.d {
			t.Errorf("%q %q: %d want %d", c.a, c.b, g, c.d)
		}
	}
}

// TestRetconDry runs the whole spike on fakes against the real run3 files: no model call, fake dieg.
func TestRetconDry(t *testing.T) {
	out := t.TempDir()
	if err := runRetcon(context.Background(), []string{"-rc-out", out, "-rc-src", "../../examples/serial/run3", "-rc-dry", "-rc-only", "2"}); err != nil {
		t.Fatal(err)
	}
	// 1 pick + 2 rewrites + 1 summary (A) + 3 (B) + 2 judge = 9 logged calls
	b, _ := os.ReadFile(filepath.Join(out, "calls.jsonl"))
	if n := strings.Count(string(b), "\n"); n != 9 {
		t.Errorf("calls logged %d, want 9\n%s", n, b)
	}
	if !strings.Contains(string(b), "r2-B-render-s3-a1") {
		t.Errorf("B calls not relabelled:\n%s", b)
	}
	// B copy must not hold the never-rendered scenes
	if _, err := os.Stat(filepath.Join(out, "r2", "b", "story", "ep12")); err == nil {
		t.Error("ep12 copied")
	}
	r, _ := os.ReadFile(filepath.Join(out, "results.json"))
	if !strings.Contains(string(r), `"UnitsChanged": 1`) {
		t.Errorf("B should change exactly s3:\n%s", r)
	}
}

func TestRetconBudgetGuard(t *testing.T) {
	out := t.TempDir()
	err := runRetcon(context.Background(), []string{"-rc-out", out, "-rc-src", "../../examples/serial/run3", "-rc-dry", "-rc-budget", "5"})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "calls.jsonl")); len(b) != 0 {
		t.Errorf("calls made despite budget 5:\n%s", b)
	}
}
