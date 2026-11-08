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

// R1-R3 of the plan-7 list must be word for word the earlier spike's.
func TestRetcon2ListKeepsR1toR3(t *testing.T) {
	got, err := loadRetcons("../../examples/value/retcon2/retcons.json")
	if err != nil || len(got) != 6 {
		t.Fatalf("%v, %d retcons", err, len(got))
	}
	for i := 0; i < 3; i++ {
		if got[i].Req != retcons[i].Req || got[i].Keys.String() != retcons[i].Keys.String() {
			t.Errorf("R%d differs from the spike", i+1)
		}
	}
}

// TestRetcon2Dry: the v2 path on fakes: stamped base, --skip for s32.., revise stats parsed, ep 12 never picked.
func TestRetcon2Dry(t *testing.T) {
	saved := retcons
	defer func() { retcons = saved }()
	out := t.TempDir()
	err := runRetcon(context.Background(), []string{"-rc-out", out, "-rc-src", "../../examples/serial/run3", "-rc-dry",
		"-rc-list", "../../examples/value/retcon2/retcons.json", "-rc-only", "4", "-rc-budget", "100"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "base", ".stamped")); err != nil {
		t.Error("base not stamped")
	}
	// the full story is in the copy (s32 coded), and the revise call carries --skip
	if _, err := os.Stat(filepath.Join(out, "r4", "b", "story", "ep12")); err != nil {
		t.Errorf("v2 copy should keep ep12 code: %v", err)
	}
	r, _ := os.ReadFile(filepath.Join(out, "results.json"))
	for _, w := range []string{`"StateStale": 1`, `"IRStale": 1`, `"Patched": 1`, `"s32"`} {
		if !strings.Contains(string(r), w) {
			t.Errorf("results.json lacks %s\n%s", w, r)
		}
	}
	if sk := skipIDs(filepath.Join(out, "r4", "b"), 32); len(sk) != 4 || sk[0] != "s32" || sk[3] != "s35" {
		t.Errorf("skipIDs = %v", sk)
	}
}

// -rc-b-only: no A call, one judge call; B still runs.
func TestRetcon2BOnlyDry(t *testing.T) {
	saved := retcons
	defer func() { retcons = saved }()
	out := t.TempDir()
	err := runRetcon(context.Background(), []string{"-rc-out", out, "-rc-src", "../../examples/serial/run3", "-rc-dry",
		"-rc-list", "../../examples/value/retcon2/retcons.json", "-rc-only", "2", "-rc-b-only", "-rc-budget", "100"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "calls.jsonl"))
	if n := strings.Count(string(b), "\n"); n != 4 { // 3 dieg + 1 judge
		t.Errorf("calls %d, want 4\n%s", n, b)
	}
	if strings.Contains(string(b), "-A-") {
		t.Error("A ran")
	}
}
