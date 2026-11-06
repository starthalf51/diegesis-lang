package syntax

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSource(t *testing.T) {
	for p, want := range map[string]bool{"a.sil.md": true, "d/a.sil.md": true, "a.sl.md": false, "a.md": false, "a.sil": false} {
		if got := IsSource(p); got != want {
			t.Errorf("IsSource(%q)=%v want %v", p, got, want)
		}
	}
}

func TestCorporaUseNewExtension(t *testing.T) {
	filepath.WalkDir("../../corpus", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".sl.md") {
			t.Errorf("old extension: %s", p)
		}
		return nil
	})
}
