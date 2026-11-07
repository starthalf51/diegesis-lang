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

func TestNotUnderCorpus(t *testing.T) {
	root := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	for out, bad := range map[string]bool{"corpus": true, "corpus/x/y": true, "out/corpus": false, "corpusx": false, "out/s": false} {
		if err := notUnderCorpus(out); (err != nil) != bad {
			t.Errorf("%s: err=%v want bad=%t", out, err, bad)
		}
	}
}

func TestBadCodeEffortExits2(t *testing.T) {
	if c := cmdWrite([]string{"idea", "-o", t.TempDir(), "--code-effort", "max"}); c != 2 {
		t.Errorf("write exit %d", c)
	}
	if c := cmdContinue([]string{t.TempDir(), "--ep", "2", "--code-effort", "extreme"}); c != 2 {
		t.Errorf("continue exit %d", c)
	}
}

func TestWriteRejectsUnknownModel(t *testing.T) {
	if c := cmdWrite([]string{"idea", "-o", t.TempDir(), "--render-model", "nope"}); c != 2 {
		t.Fatalf("exit %d", c)
	}
}
