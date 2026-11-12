package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Ruling T1-R2: a waived error still fails `dieg lint` and still reaches the write/convert loop; a waived warn passes as before.
func TestLintErrorNotWaivable(t *testing.T) {
	story := func(id, body string) string {
		dir := t.TempDir()
		src := "module " + id + "\nwork " + id + " \"T\"\ncharacter a \"A\"\ncharacter b \"B\"\n" + body
		if err := os.WriteFile(filepath.Join(dir, "s.dieg.md"), []byte("```sl\n"+src+"```\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	term := story("t1r2_term", "term chip \"칩\" = \"신분 칩\"\nscene 1\n  enter a, b\n  #! allow reader-unknown-term: \"style\"\n  a says \"칩을 봐\"\n")
	warn := story("t1r2_warn", "fact p \"P\" true\nscene 1\n  enter a, b\n  #! allow info-flow: \"style\"\n  a tells b: p\n")
	var code int
	captureStderr(t, func() { code = cmdLint([]string{term}) })
	if code != 1 {
		t.Errorf("waived error: exit %d, want 1", code)
	}
	ds, err := writeChecker(nil)(context.Background(), term)
	if err != nil || len(ds) != 1 || ds[0].Rule != "reader-unknown-term" || !ds[0].Blocking() {
		t.Errorf("write checker dropped the waived error: %v %v", ds, err)
	}
	captureStderr(t, func() { code = cmdLint([]string{warn}) })
	if code != 0 {
		t.Errorf("waived warn: exit %d, want 0", code)
	}
}
