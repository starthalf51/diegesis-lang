package write

import (
	"os"
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
	if string(want) != r.Keywords {
		t.Fatalf("keywords block differs from %s; this changes the coding prompt", p)
	}
}
