package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// The brief splits facts into held-before, handed-over-in-range and still-hidden; a fact never lands in two sections.
func TestBriefSections(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	dir := t.TempDir()
	src := "---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n" + rsWork + "attr jokbo.rule = \"한 줄에 한 사람\"\nattr a.role = \"문지기\"\n```\n"
	os.WriteFile(filepath.Join(dir, "w.dieg.md"), []byte(src), 0o644)
	work, err := load(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pg.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	defer pool.Exec(context.Background(), `DELETE FROM work WHERE id=$1`, work)
	got, err := briefText(context.Background(), pool, work, briefOpts{from: "2", to: "3", since: "1", code: "CODE"})
	if err != nil {
		t.Fatal(err)
	}
	sec := func(mark string) string {
		i := strings.Index(got, mark)
		if i < 0 {
			t.Fatalf("no section %q in\n%s", mark, got)
		}
		s := got[i:]
		if j := strings.Index(s[len(mark):], "\n## "); j >= 0 {
			s = s[:len(mark)+j]
		}
		return s
	}
	if s := sec("## ①"); !strings.Contains(s, "문이 잠겼다") || strings.Contains(s, "배가 떠났다") {
		t.Errorf("held section wrong:\n%s", s)
	}
	if s := sec("## ② 이 화가 세울 세계"); !strings.Contains(s, "풀이(참고): 손목 신분 칩") || !strings.Contains(s, "⚠ 보여 줄 사건") || strings.Contains(s, "족보") {
		t.Errorf("new-terms section wrong (chip is first explained in scene 2, jokbo in scene 1):\n%s", s)
	}
	if s := sec("## ② 이 화가 독자에게"); !strings.Contains(s, "배가 떠났다") || strings.Contains(s, "문이 잠겼다") {
		t.Errorf("handover section wrong:\n%s", s)
	}
	if s := sec("## ③"); strings.Contains(s, "배가 떠났다") || strings.Contains(s, "문이 잠겼다") {
		t.Errorf("hidden section leaks known facts:\n%s", s)
	}
	if !strings.Contains(got, "- 족보: 한 줄에 한 사람") {
		t.Errorf("rule missing:\n%s", got)
	}
	if !strings.Contains(got, "역할: 문지기") || !strings.Contains(got, "앞에서 독자가 들은 말") {
		t.Errorf("card or heard lines missing:\n%s", got)
	}
	if !strings.HasSuffix(got, "CODE") {
		t.Errorf("code not appended")
	}
}

func TestBriefCardAndComments(t *testing.T) {
	if got := stripCodeComments("# 8화\n```sl\nscene 1\n  # 작가 주석\n  enter a\n```\n"); strings.Contains(got, "작가 주석") || !strings.Contains(got, "# 8화") {
		t.Errorf("stripCodeComments = %q", got)
	}
	if !ruleSays("- 삭제: 에이전트는 판정 없이 해칠 수 없다. 구금도", "에이전트는 판정 없이 해칠 수 없다 — 구금도 위해다") {
		t.Error("ruleSays missed a rule-stated fact")
	}
}

// Unmet characters lose both name and identifier; a short id is replaced only as a whole word.
func TestBriefMasksUnmet(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "w.dieg.md"), []byte("---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n"+rsWork+"```\n"), 0o644)
	work, err := load(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pg.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	defer pool.Exec(context.Background(), `DELETE FROM work WHERE id=$1`, work)
	got, err := maskNames(context.Background(), pool, work, "2", toldOrd(context.Background(), pool, work, "3"), "", "enter a, c\nCyd came; cache stays")
	if err != nil {
		t.Fatal(err)
	}
	if want := "enter a, (이름을 모르는 인물 1)\n(이름을 모르는 인물 1) came; cache stays"; got != want {
		t.Errorf("maskNames = %q, want %q", got, want)
	}
}

func TestSubj(t *testing.T) {
	for in, want := range map[string]string{"단속반원": "단속반원이", "축지": "축지가", "0419": "0419가", "춘섬": "춘섬이"} {
		if got := subj(in); got != want {
			t.Errorf("subj(%q) = %q, want %q", in, got, want)
		}
	}
}
