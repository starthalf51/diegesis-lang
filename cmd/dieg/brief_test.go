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
	got, err := briefText(context.Background(), pool, work, "2", "3", "CODE", "")
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
	if s := sec("## ②"); !strings.Contains(s, "배가 떠났다") || strings.Contains(s, "문이 잠겼다") {
		t.Errorf("handover section wrong:\n%s", s)
	}
	if s := sec("## ③"); strings.Contains(s, "배가 떠났다") || strings.Contains(s, "문이 잠겼다") {
		t.Errorf("hidden section leaks known facts:\n%s", s)
	}
	if !strings.HasSuffix(got, "CODE") {
		t.Errorf("code not appended")
	}
}
