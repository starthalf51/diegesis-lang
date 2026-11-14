package main

import (
	"context"
	"fmt"
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

func TestDealLook(t *testing.T) {
	got := dealLook("a · b · c · d · e", []string{"e1.3", "e1.1"})
	if want := "e1.1에 쓸 재료: a · b / e1.3에 쓸 재료: c · d / 쓰지 않아도 되는 것: e"; got != want {
		t.Errorf("dealLook = %q, want %q", got, want)
	}
	if got := dealLook("한 문장짜리 감각", []string{"e1"}); got != "한 문장짜리 감각" {
		t.Errorf("sentence look changed: %q", got)
	}
}

// The frame closing rule is work data (frame_policy); only on_change drops the closing line when nothing moved.
func TestCloseLinesPolicy(t *testing.T) {
	tpl := []string{"[w] 남은 {d}일", "[t] 누적 {n}/7"}
	same := map[string]span{"d": {start: "25", end: "25"}, "n": {start: "3", end: "3"}}
	if got := closeLines(tpl, same, "every"); !strings.Contains(got, "`[w] 남은 25일`") || strings.Contains(got, "줄 없이") {
		t.Errorf("every with no change must still close with the first template:\n%s", got)
	}
	if got := closeLines(tpl, same, ""); !strings.Contains(got, "`[w] 남은 25일`") {
		t.Errorf("empty policy = every:\n%s", got)
	}
	if got := closeLines(tpl, same, "bogus"); !strings.Contains(got, "`[w] 남은 25일`") {
		t.Errorf("unknown policy = every:\n%s", got)
	}
	if got := closeLines(tpl, same, "on_change"); !strings.Contains(got, "바뀌는 값이 없다") {
		t.Errorf("on_change with no change keeps the no-line close:\n%s", got)
	}
	moved := map[string]span{"d": {start: "25", end: "25"}, "n": {start: "3", end: "4"}}
	if got := closeLines(tpl, moved, "every"); !strings.Contains(got, "`[t] 누적 4/7`") {
		t.Errorf("a changed value is offered:\n%s", got)
	}
	maybe := map[string]span{"d": {start: "25", end: "25", maybe: "24"}, "n": {start: "3", end: "3"}}
	if got := closeLines(tpl, maybe, "every"); !strings.Contains(got, "`[w] 남은 24일`") || !strings.Contains(got, "`[w] 남은 25일`") {
		t.Errorf("open day under every: both the midnight line and the same-day line:\n%s", got)
	}
}

// A character who speaks into the episode without being on stage still gets a card, once.
func TestBriefOffstageSpeakerHasCard(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	dir := t.TempDir()
	code := "work RSO \"T\"\ncharacter a \"Ann\"\ncharacter b \"Bob\"\nfact f1 \"문이 잠겼다\" true\nattr b.role = \"통신 너머의 사람\"\nattr a.role = \"문지기\"\n" +
		"scene 1\n  enter a\n  #! allow offstage-speaker: \"voice\"\n  b tells a: f1\n  a tells a: f1\nscene 2\n  enter a\n"
	os.WriteFile(filepath.Join(dir, "w.dieg.md"), []byte("---\nmodule: rso.s\nmode: authored\n---\n\n```sl\n"+code+"```\n"), 0o644)
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
	got, err := briefText(context.Background(), pool, work, briefOpts{from: "1", to: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "역할: 통신 너머의 사람") {
		t.Errorf("an offstage speaker must get a card:\n%s", got)
	}
	if strings.Count(got, "역할: 문지기") != 1 {
		t.Errorf("a character both present and speaking appears once:\n%s", got)
	}
}

// One closing rule per brief: with a frame closing line, ④ points at it; without one, ④ keeps the object close.
// A work with no clocks, counters or frames gets no empty ⓪ clock section.
func TestBriefOneClosingRule(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	pool, err := pg.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	gen := func(extra string) string {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "w.dieg.md"), []byte("---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n"+rsWork+extra+"```\n"), 0o644)
		work, err := load(dir, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(context.Background(), `DELETE FROM work WHERE id=$1`, work)
		got, err := briefText(context.Background(), pool, work, briefOpts{from: "2", to: "3", since: "1"})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	framed := gen("attr jokbo.frame_open = \"[기록] 열림\"\nattr jokbo.frame_close = \"[기록] 닫힘\"\n")
	if !strings.Contains(framed, "마지막 줄은 ⓪의 닫는 줄이다") || strings.Contains(framed, "마지막 단락은 이 화가 남긴") {
		t.Errorf("framed work: ④ must point at the ⓪ closing line only:\n%s", framed)
	}
	plain := gen("")
	if strings.Contains(plain, "마지막 줄은 ⓪의 닫는 줄이다") || !strings.Contains(plain, "마지막 단락은 이 화가 남긴") {
		t.Errorf("plain work keeps the object close:\n%s", plain)
	}
	if strings.Contains(plain, "## ⓪ 시계와 카운터") {
		t.Errorf("no empty clock section:\n%s", plain)
	}
}

// Review fix: under every with an open day, the template whose value may move gives both branches, wherever it is.
func TestCloseLinesOpenDayNotFirst(t *testing.T) {
	tpl := []string{"[t] 누적 {n}/7", "[w] 남은 {d}일"}
	maybe := map[string]span{"d": {start: "25", end: "25", maybe: "24"}, "n": {start: "3", end: "3"}}
	got := closeLines(tpl, maybe, "every")
	if !strings.Contains(got, "`[w] 남은 24일`") || !strings.Contains(got, "`[w] 남은 25일`") {
		t.Errorf("the moving template's two branches:\n%s", got)
	}
}

// Review fix: "first template" is the first declared, also across attr9/attr10 ids.
func TestBriefFirstFrameCloseByDeclaration(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	pool, err := pg.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	dir := t.TempDir()
	extra := ""
	for i := 1; i <= 7; i++ {
		extra += fmt.Sprintf("attr c.x%d = \"v\"\n", i)
	}
	extra += "attr jokbo.frame_open = \"[기록] 열림\"\nattr jokbo.frame_close = \"[기록] 첫째\"\nattr jokbo.frame_close = \"[기록] 둘째\"\n"
	os.WriteFile(filepath.Join(dir, "w.dieg.md"), []byte("---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n"+rsWork+extra+"```\n"), 0o644)
	work, err := load(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM work WHERE id=$1`, work)
	got, err := briefText(context.Background(), pool, work, briefOpts{from: "2", to: "3", since: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "`[기록] 첫째`") || strings.Contains(got, "`[기록] 둘째`") {
		t.Errorf("first declared frame_close closes:\n%s", got)
	}
}
