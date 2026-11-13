package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

const rsWork = `
work RSW "T"
character a "Ann"
character b "Bob"
character c "Cyd"
term jokbo "족보" = "시민 기록부"
term chip "칩" = "손목 신분 칩"
fact f1 "문이 잠겼다" true
fact f2 "배가 떠났다" true
fact f3 "금고가 비었다" false
scene 1
  enter a
  a tells b: f1
  a lies b: f3
  render
    explain jokbo
scene 2
  enter a, b
  a tells b: f2
  render
    explain chip
scene 3
  enter a, c
`

func TestReaderStateUntil(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	dir := t.TempDir()
	src := "---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n" + rsWork + "```\n"
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
	out := func(until string) string {
		s, err := readerState(context.Background(), pool, work, until)
		if err != nil {
			t.Fatal(err)
		}
		return s.Text()
	}
	s3 := out("3")
	for _, w := range []string{"족보: 시민 기록부", "칩: 손목 신분 칩", "Ann", "Bob", "문이 잠겼다", "배가 떠났다"} {
		if !strings.Contains(s3, w) {
			t.Errorf("until 3 missing %q:\n%s", w, s3)
		}
	}
	if strings.Contains(s3, "Cyd") {
		t.Errorf("until 3 leaks scene 3 cast:\n%s", s3)
	}
	s2 := out("2")
	if !strings.Contains(s2, "족보") || !strings.Contains(s2, "문이 잠겼다") || !strings.Contains(s2, "Ann") {
		t.Errorf("until 2 lacks scene 1:\n%s", s2)
	}
	for _, w := range []string{"칩", "배가 떠났다", "Bob"} {
		if strings.Contains(s2, w) {
			t.Errorf("until 2 leaks %q:\n%s", w, s2)
		}
	}
	if s1 := out("1"); strings.TrimSpace(s1) != "" {
		t.Errorf("until 1 should be empty:\n%s", s1)
	}
}

func TestReaderStateMarksFalseFacts(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Skip("SL_PG_DSN not set")
	}
	dir := t.TempDir()
	src := "---\nmodule: rs.s\nmode: authored\n---\n\n```sl\n" + rsWork + "```\n"
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
	s, err := readerState(context.Background(), pool, work, "2")
	if err != nil {
		t.Fatal(err)
	}
	var lie, truth *rsFact
	for i := range s.Facts {
		switch {
		case strings.Contains(s.Facts[i].Text, "금고가 비었다"):
			lie = &s.Facts[i]
		case strings.Contains(s.Facts[i].Text, "문이 잠겼다"):
			truth = &s.Facts[i]
		}
	}
	if lie == nil || truth == nil || lie.Value || !truth.Value {
		t.Fatalf("facts %+v", s.Facts)
	}
	txt := s.Text()
	if !strings.Contains(txt, "알려진 사실: (사실 아님) ") || strings.Contains(txt, "알려진 사실: (사실 아님) 문이 잠겼다") {
		t.Errorf("prefix wrong:\n%s", txt)
	}
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), `"value":false`) {
		t.Errorf("json lacks value:false: %s", b)
	}
	if s3, _ := readerState(context.Background(), pool, work, "1"); len(s3.Facts) != 0 {
		t.Errorf("scene 1 facts leak into until 1: %+v", s3.Facts)
	}
}
