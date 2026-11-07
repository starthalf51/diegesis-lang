package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSerialDryRun(t *testing.T) {
	ctx := context.Background()
	out := t.TempDir()
	defer func() {
		if fmt.Sprintf("%p", runDieg) == fmt.Sprintf("%p", fakeDieg) {
			t.Error("-dry left runDieg swapped to fakeDieg")
		}
	}()
	idea := filepath.Join(out, "idea.txt")
	os.WriteFile(idea, []byte("가짜 아이디어."), 0o644)
	if err := run(ctx, []string{"-dry", "-idea", idea, "-eps", "3", "-rev-after", "2", "-out", out}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plan/plan.md", "plan/calls.jsonl", "a/outline.md", "a/summary.md", "a/eps/e03.md", "a/prompts/ep-2.txt", "a/prompts/rev1-pick.txt",
		"a/calls.jsonl", "b/calls.jsonl", "b/novel.md", "b-idea.txt", "judge/prompt.txt", "judge/mapping.txt", "judge/run1.txt",
		"judge/quotes.md", "judge/revision.md", "metrics.csv", "summary.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Error(err)
		}
	}
	rd := func(f string) string { b, _ := os.ReadFile(filepath.Join(out, f)); return string(b) }
	if p := rd("a/prompts/ep-2.txt"); !strings.Contains(p, "## 직전 화 (1화)") || !strings.Contains(p, "## 누적 요약") || !strings.Contains(p, "가짜 방향 2") {
		t.Errorf("ep-2 prompt lacks previous episode, summary or the shared brief:\n%s", p)
	}
	if !strings.Contains(rd("b-idea.txt"), "1화 방향: 가짜 방향 1") {
		t.Error("B idea lacks the shared brief")
	}
	jp := rd("judge/prompt.txt")
	if strings.Contains(jp, "첫 장면") || strings.Count(jp, "### 화 ") != 6 || !strings.Contains(jp, "수정 1 (2화 뒤에 받음)") {
		t.Errorf("judge prompt not blind, wrong episode count or no revision request")
	}
	if q := rd("judge/quotes.md"); !strings.Contains(q, "(있음) 종이 울렸다.") || !strings.Contains(q, "(없음) 지어낸 인용") {
		t.Errorf("quote check:\n%s", q)
	}

	// totals: fake call = in 2 + cache_read 100 + cache_write 50 = 152 real, 2+10+62.5 -> 74.5 weighted, out 10.
	// A: outline 1 + ep1 2 + ep2 2 + rev (pick 1 + 2 rewrites + sum 1) + ep3 2 = 11. B: write + continue x2 + revise = 4 x 3 = 12.
	var s struct {
		A, B, Judge    struct{ Calls, RealTokens, CacheWeighted, Output int64 }
		Plan_uncharged struct{ Calls int64 }
		Revisions      []struct {
			AfterEp   int `json:"after_ep"`
			A, B      struct{ Calls, Cache_weighted int64 }
			AResidual int
			BResidual int
		}
	}
	if err := json.Unmarshal([]byte(rd("summary.json")), &s); err != nil {
		t.Fatal(err)
	}
	if s.A.Calls != 11 || s.A.RealTokens != 11*152 || s.A.Output != 110 || s.A.CacheWeighted != 820 { // 11*74.5 = 819.5 -> 820
		t.Errorf("A %+v", s.A)
	}
	if s.B.Calls != 12 || s.B.RealTokens != 12*152 || s.B.CacheWeighted != 894 { // 12*74.5
		t.Errorf("B %+v", s.B)
	}
	if s.Plan_uncharged.Calls != 1 || s.Judge.Calls != 1 {
		t.Errorf("plan %+v judge %+v", s.Plan_uncharged, s.Judge)
	}
	if len(s.Revisions) != 1 || s.Revisions[0].AfterEp != 2 || s.Revisions[0].A.Calls != 4 || s.Revisions[0].B.Calls != 3 {
		t.Errorf("revisions %+v", s.Revisions)
	}
	// judge marks: X has 1 unapplied ep, Y has 2; mapping says which arm is which.
	x := strings.Contains(rd("judge/mapping.txt"), "X=A")
	wa, wb := 2, 1
	if x {
		wa, wb = 1, 2
	}
	if r := s.Revisions[0]; r.AResidual != wa || r.BResidual != wb {
		t.Errorf("residual A=%d B=%d, want %d %d", r.AResidual, r.BResidual, wa, wb)
	}

	// metrics.csv: cumulative, so the last row per arm equals the arm total.
	f, _ := os.Open(filepath.Join(out, "metrics.csv"))
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	last := map[string][]string{}
	for _, r := range rows[1:] {
		last[r[1]] = r
	}
	if rows[0][0] != "step" || len(rows) != 1+(1+3+1)+(3+1) || last["A"][2] != "11" || last["A"][4] != "820" || last["B"][2] != "12" || last["B"][3] != "1824" {
		t.Errorf("csv %v", rows)
	}

	// budget guard: stops instead of overspending.
	out2 := t.TempDir()
	if err := run(ctx, []string{"-dry", "-idea", idea, "-eps", "3", "-rev-after", "2", "-budget", "5", "-out", out2}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Errorf("budget not enforced: %v", err)
	}
}
