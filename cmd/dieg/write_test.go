package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCheckerRejectsCorpusID(t *testing.T) {
	ds, err := writeChecker([]string{"macbeth"})(context.Background(), "../../corpus/macbeth")
	if err != nil || len(ds) == 0 || ds[len(ds)-1].Rule != "bundle" {
		t.Fatalf("%v %v", ds, err)
	}
}

func TestWriteCheckerRelativePaths(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "scenes"), 0o755)
	os.WriteFile(filepath.Join(dir, "scenes", "s1.dieg.md"), []byte("---\nmodule: x.scenes\n---\n\n```sl\nnonsense here\n```\n"), 0o644)
	ds, err := writeChecker(nil)(context.Background(), dir)
	if err != nil || len(ds) == 0 {
		t.Fatalf("%v %v", ds, err)
	}
	if ds[0].File != "scenes/s1.dieg.md" {
		t.Errorf("file %q, want bundle-relative", ds[0].File)
	}
}
