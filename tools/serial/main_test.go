package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMain doubles as the fake dieg binary for TestRealDiegExit: with SERIAL_HELPER_EXIT set it only exits.
func TestMain(m *testing.M) {
	if v := os.Getenv("SERIAL_HELPER_EXIT"); v != "" {
		n, _ := strconv.Atoi(v)
		os.Exit(n)
	}
	os.Exit(m.Run())
}

type armSum struct {
	Calls, RealTokens, CacheWeighted, Output int64
	ByModel                                  map[string]struct{ Calls int64 }
	PriceWeighted                            *float64
}

type summary struct {
	Cut_at_episode, Episodes int
	Stopped_by, Judge_status string
	Rev_after                []int
	Chars_per_episode        map[string][]int
	B_missing_scenes         map[string]int
	Plan_uncharged, A, B     armSum
	Judge                    armSum
	Revisions                []struct {
		AfterEp              int `json:"after_ep"`
		A, B                 struct{ Calls int64 }
		AResidual, BResidual any
	}
}

// dry runs the harness with fakes in a fresh dir and returns the dir, the parsed summary.json and a file reader.
func dry(t *testing.T, args ...string) (string, summary, func(string) string) {
	t.Helper()
	out := t.TempDir()
	idea := filepath.Join(out, "idea.txt")
	os.WriteFile(idea, []byte("가짜 아이디어."), 0o644)
	all := append([]string{"-dry", "-idea", idea, "-out", out}, args...)
	if err := run(context.Background(), all); err != nil {
		t.Fatal(err)
	}
	rd := func(f string) string { b, _ := os.ReadFile(filepath.Join(out, f)); return string(b) }
	var s summary
	if err := json.Unmarshal([]byte(rd("summary.json")), &s); err != nil {
		t.Fatal(err)
	}
	return out, s, rd
}

func TestSerialDryRun(t *testing.T) {
	defer func() {
		if fmt.Sprintf("%p", runDieg) == fmt.Sprintf("%p", fakeDieg) {
			t.Error("-dry left runDieg swapped to fakeDieg")
		}
	}()
	out, s, rd := dry(t, "-eps", "3", "-rev-after", "2", "-price", "claude-opus-5-5=1:2,claude-sonnet-5-5=1:1")
	for _, f := range []string{"plan/plan.md", "plan/calls.jsonl", "a/outline.md", "a/summary.md", "a/eps/e03.md", "a/prompts/ep-2.txt", "a/prompts/rev1-pick.txt",
		"a/calls.jsonl", "b/calls.jsonl", "b/novel/e1a.ko-haera.md", "b-idea.txt", "judge/prompt1.txt", "judge/mapping.txt", "judge/run1.txt",
		"judge/quotes.md", "judge/revision.md", "judge/calls.jsonl", "metrics.csv", "summary.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Error(err)
		}
	}
	if p := rd("a/prompts/ep-2.txt"); !strings.Contains(p, "## 직전 화 (1화)") || !strings.Contains(p, "## 누적 요약") || !strings.Contains(p, "가짜 방향 2") || !strings.Contains(p, "약 5000자") {
		t.Errorf("ep-2 prompt lacks previous episode, summary, shared brief or length:\n%s", p)
	}
	if i := rd("b-idea.txt"); !strings.Contains(i, "1화 방향: 가짜 방향 1") || !strings.Contains(i, "약 5000자") {
		t.Error("B idea lacks the shared brief or the length line")
	}
	jp := rd("judge/prompt1.txt")
	if strings.Contains(jp, "첫 장면") || strings.Count(jp, "### 화 ") != 6 || !strings.Contains(jp, "수정 1 (2화 뒤에 받음)") {
		t.Errorf("judge prompt not blind, wrong episode count or no revision request")
	}
	if q := rd("judge/quotes.md"); !strings.Contains(q, "(있음) 종이 울렸다.") || !strings.Contains(q, "(없음) 지어낸 인용") {
		t.Errorf("quote check:\n%s", q)
	}

	// fake call = in 2 + cache_read 100 + cache_write 50 = 152 real, 74.5 weighted, out 10.
	// A: outline 1 + ep1 2 + ep2 2 + rev (pick 1 + 2 rewrites + sum 1) + ep3 2 = 11. B: write + continue x2 + revise = 4 x 3 = 12.
	if s.A.Calls != 11 || s.A.RealTokens != 11*152 || s.A.Output != 110 || s.A.CacheWeighted != 820 { // 819.5 rounds up
		t.Errorf("A %+v", s.A)
	}
	if s.B.Calls != 12 || s.B.RealTokens != 12*152 || s.B.CacheWeighted != 894 {
		t.Errorf("B %+v", s.B)
	}
	if s.Plan_uncharged.Calls != 1 || s.Judge.Calls != 1 || s.Cut_at_episode != 3 || s.Judge_status != "ok" {
		t.Errorf("plan %+v judge %+v cut %d status %q", s.Plan_uncharged, s.Judge, s.Cut_at_episode, s.Judge_status)
	}
	// per model: B has 4 code calls on opus and 8 render/extract calls on sonnet; price-weighted = weighted*in + output*out.
	if b := s.B.ByModel; b["claude-opus-5-5"].Calls != 4 || b["claude-sonnet-5-5"].Calls != 8 || len(s.A.ByModel) != 1 {
		t.Errorf("per-model %+v / %+v", s.B.ByModel, s.A.ByModel)
	}
	if s.A.PriceWeighted == nil || *s.A.PriceWeighted != 820+2*110 || s.B.PriceWeighted == nil || *s.B.PriceWeighted != 298+2*40+596+80 {
		t.Errorf("price-weighted A %v B %v", s.A.PriceWeighted, s.B.PriceWeighted)
	}
	if len(s.Revisions) != 1 || s.Revisions[0].AfterEp != 2 || s.Revisions[0].A.Calls != 4 || s.Revisions[0].B.Calls != 3 {
		t.Errorf("revisions %+v", s.Revisions)
	}
	// judge marks: X has 1 unapplied ep, Y has 2; mapping says which arm is which.
	wa, wb := 2.0, 1.0
	if strings.Contains(rd("judge/mapping.txt"), "X=A") {
		wa, wb = 1, 2
	}
	if r := s.Revisions[0]; r.AResidual != wa || r.BResidual != wb {
		t.Errorf("residual A=%v B=%v, want %v %v", r.AResidual, r.BResidual, wa, wb)
	}

	// metrics.csv: interleaved, cumulative, so the last row per arm equals the arm total; errors column present.
	f, _ := os.Open(filepath.Join(out, "metrics.csv"))
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	last := map[string][]string{}
	for _, r := range rows[1:] {
		order = append(order, r[0]+r[1])
		last[r[1]] = r
	}
	if rows[0][3] != "errors" || len(rows) != 10 || last["A"][2] != "11" || last["A"][5] != "820" || last["B"][2] != "12" || last["B"][4] != "1824" {
		t.Errorf("csv %v", rows)
	}
	if got := strings.Join(order, " "); got != "outlineA ep1A ep1B ep2A ep2B rev1A rev1B ep3A ep3B" {
		t.Errorf("not interleaved: %s", got)
	}
}

func TestSerialBudgetCut(t *testing.T) {
	// limit for non-judge steps = 11-2 = 9 calls: plan 1, outline 1, ep1 A 2 + B 3 = 7, ep2 A 2 = 9, B ep2 trips.
	_, s, rd := dry(t, "-eps", "3", "-rev-after", "2", "-budget", "11")
	if s.Cut_at_episode != 1 || !strings.Contains(s.Stopped_by, "budget") || s.Judge_status != "ok" || len(s.Revisions) != 0 || len(s.Rev_after) != 0 {
		t.Errorf("%+v", s)
	}
	if jp := rd("judge/prompt1.txt"); strings.Count(jp, "### 화 ") != 2 || strings.Contains(jp, "수정 1 (") {
		t.Errorf("judge saw more than episode 1 of each arm or a revision that was not applied:\n%s", jp)
	}
	if len(s.Chars_per_episode["A"]) != 1 {
		t.Errorf("chars %+v", s.Chars_per_episode)
	}
}

func TestSerialMissingScene(t *testing.T) {
	fakeSkip = "e2b"
	defer func() { fakeSkip = "" }()
	_, s, _ := dry(t, "-eps", "3", "-rev-after", "2")
	b := s.Chars_per_episode["B"]
	if s.B_missing_scenes["2"] != 1 || len(s.B_missing_scenes) != 1 || len(b) != 3 || b[1] >= b[2] {
		t.Errorf("missing %v chars %v: episode 3 must keep its own scenes", s.B_missing_scenes, b)
	}
}

func TestSerialJudgeFailures(t *testing.T) {
	defer func() { fakeJudge = "" }()
	for _, mode := range []string{"garbage", "noY"} {
		fakeJudge = mode
		out, s, _ := dry(t, "-eps", "3", "-rev-after", "2")
		if !strings.HasPrefix(s.Judge_status, "unknown") || s.A.Calls != 11 || len(s.Revisions) != 1 ||
			s.Revisions[0].AResidual != "unknown" || s.Revisions[0].BResidual != "unknown" {
			t.Errorf("%s: %+v", mode, s)
		}
		if _, err := os.Stat(filepath.Join(out, "judge/run1.txt")); err != nil {
			t.Errorf("%s: raw judge output lost", mode)
		}
	}
	// a revision the judge omits for one arm is unknown for that arm, never 0
	fakeJudge = "norev"
	out, s, _ := dry(t, "-eps", "3", "-rev-after", "2")
	mp, _ := os.ReadFile(filepath.Join(out, "judge/mapping.txt"))
	xRes, yRes := s.Revisions[0].BResidual, s.Revisions[0].AResidual // X=B
	if strings.Contains(string(mp), "X=A") {
		xRes, yRes = s.Revisions[0].AResidual, s.Revisions[0].BResidual
	}
	if s.Judge_status != "ok" || xRes != 1.0 || yRes != "unknown" {
		t.Errorf("X residual %v, Y residual %v", xRes, yRes)
	}
}

func TestSerialJudgeHalves(t *testing.T) {
	judgeMaxChars = 100
	defer func() { judgeMaxChars = 150000 }()
	out, s, rd := dry(t, "-eps", "3", "-rev-after", "2")
	if s.Judge.Calls != 2 || s.Judge_status != "ok" || strings.Count(rd("judge/prompt1.txt"), "### 화 ") != 2 || strings.Count(rd("judge/prompt2.txt"), "### 화 ") != 4 {
		t.Errorf("halves: %+v", s.Judge)
	}
	if _, err := os.Stat(filepath.Join(out, "judge/run2.txt")); err != nil {
		t.Error(err)
	}
}

// TestRealDiegExit: exit 3 (a scene failed verify) is tolerated and logged, 0 passes, any other code aborts.
func TestRealDiegExit(t *testing.T) {
	diegBin = os.Args[0]
	for code, wantErr := range map[string]bool{"0": false, "3": false, "1": true} {
		out := t.TempDir()
		t.Setenv("SERIAL_HELPER_EXIT", code)
		err := realDieg(context.Background(), out, []string{"write"})
		if (err != nil) != wantErr {
			t.Errorf("exit %s: err %v", code, err)
		}
		if lg, _ := os.ReadFile(filepath.Join(out, "b.log")); code == "3" && !strings.Contains(string(lg), "exit 3") {
			t.Errorf("exit 3 not logged: %q", lg)
		}
	}
}
