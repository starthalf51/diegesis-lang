package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

type stubC struct{ out string }

func (s stubC) Complete(ctx context.Context, system, user string) (render.Result, error) {
	return render.Result{Text: s.out}, nil
}

func TestSettleLines(t *testing.T) {
	got, _ := settleLines(context.Background(), stubC{"머리말\n- 철문은 쇠사슬로 잠겼다\n- 드론 빛은 흰색\n잡음"}, "원고")
	if !slices.Equal(got, []string{"철문은 쇠사슬로 잠겼다", "드론 빛은 흰색"}) {
		t.Errorf("settleLines = %q", got)
	}
	if got, err := settleLines(context.Background(), stubC{"목록 없음"}, "원고"); err != nil || len(got) != 0 {
		t.Errorf("no list lines → empty, no error: %q %v", got, err)
	}
	if got, _ := settleLines(context.Background(), stubC{strings.Repeat("- x\n", 40)}, "원고"); len(got) != 25 {
		t.Errorf("at most 25 lines, got %d", len(got))
	}
}

// Only settled files of scenes told before the episode reach the brief; a file not named after a scene is skipped.
func TestBriefSettled(t *testing.T) {
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
	sd := t.TempDir()
	os.WriteFile(filepath.Join(sd, "1.txt"), []byte("철문은 잠겼다\n"), 0o644)
	os.WriteFile(filepath.Join(sd, "3.txt"), []byte("미래\n"), 0o644)
	os.WriteFile(filepath.Join(sd, "notes.txt"), []byte("무시\n"), 0o644)
	got, err := briefText(context.Background(), pool, work, briefOpts{from: "2", to: "3", settled: sd})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## ① 앞 원고에서 정해진 것") || !strings.Contains(got, "- 철문은 잠겼다") {
		t.Errorf("settled detail missing:\n%s", got)
	}
	if strings.Contains(got, "미래") || strings.Contains(got, "무시") {
		t.Errorf("a later or unnamed file leaked:\n%s", got)
	}
}
