package write

import (
	"os"
	"strings"
	"testing"
)

// The keyword/lint-rule block goes into the coding prompts verbatim, so any change to it
// (including a rules/*.sql `-- summary:` header) is a prompt change. Regenerate with UPDATE_GOLDEN=1.
func TestKeywordsGolden(t *testing.T) {
	r, err := LoadRefs()
	if err != nil {
		t.Fatal(err)
	}
	const p = "testdata/keywords.golden"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(p, []byte(r.Keywords), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Compare without CR: on Windows checkouts (core.autocrlf) rule files and the golden file get CRLF.
	if lf(string(want)) != lf(r.Keywords) {
		t.Fatalf("keywords block differs from %s; this changes the coding prompt", p)
	}
}

func lf(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
