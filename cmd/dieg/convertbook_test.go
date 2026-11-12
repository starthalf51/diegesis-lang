package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/write"
)

func TestConvertBookEmptySelection(t *testing.T) {
	dir := t.TempDir()
	text := filepath.Join(dir, "book.txt")
	os.WriteFile(text, []byte("CHAPTER 1. A\n\nx\n"), 0o644)
	o := write.DefaultOpts()
	o.Out = filepath.Join(dir, "out")
	code, err := runConvertBook(text, `^CHAPTER \d+`, "200-300", o)
	if code != 1 || err == nil || !strings.Contains(err.Error(), `"200-300"`) {
		t.Fatalf("code %d err %v", code, err)
	}
}
