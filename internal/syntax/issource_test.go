package syntax

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSource(t *testing.T) {
	for p, want := range map[string]bool{"a.dieg.md": true, "d/a.dieg.md": true, "a.sil.md": false, "a.sl.md": false, "a.md": false, "a.dieg": false} {
		if got := IsSource(p); got != want {
			t.Errorf("IsSource(%q)=%v want %v", p, got, want)
		}
	}
}

func TestCorporaUseNewExtension(t *testing.T) {
	filepath.WalkDir("../../corpus", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".sl.md") || strings.HasSuffix(p, ".sil.md")) {
			t.Errorf("old extension: %s", p)
		}
		return nil
	})
}
