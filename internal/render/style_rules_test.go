package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBackend(t *testing.T, body string) string {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "x.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestStyleRulesOnly(t *testing.T) {
	d := writeBackend(t, `{"id":"x","lang":"en","tone":"","sample":{"id":"x","lang":"en",
	 "rules":["Short sentences."],
	 "lexicon":[{"use":"sol","for":"a day on Mars","avoid":["day","24 hours"]}]}}`)
	bs, err := LoadBackends(d)
	if err != nil {
		t.Fatal(err)
	}
	s := mustLoad(t, "testdata/scenes/"+firstScene(t))
	sys, _, err := bs[0].Prompt(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Style rules:", "- Short sentences.", "Word choices:", `use "sol" for a day on Mars; never "day", "24 hours"`} {
		if !strings.Contains(sys, w) {
			t.Errorf("system prompt missing %q", w)
		}
	}
	if strings.Contains(sys, "<style_sample>") {
		t.Error("rules-only style must not emit a sample block")
	}
	empty := writeBackend(t, `{"id":"x","lang":"en","sample":{"id":"x","lang":"en"}}`)
	if _, err := LoadBackends(empty); err == nil {
		t.Error("empty style must be an error")
	}
	if _, err := LoadStyle(filepath.Join(empty, "x.json")); err == nil {
		t.Log("LoadStyle on a backend file is not the style shape; ignored")
	}
}

func firstScene(t *testing.T) string {
	t.Helper()
	m, _ := filepath.Glob("testdata/scenes/*.json")
	if len(m) == 0 {
		t.Fatal("no scenes")
	}
	return filepath.Base(m[0])
}

func TestOpening(t *testing.T) {
	st := &StyleSample{ID: "x", Lang: "en", Rules: []string{"r"}}
	s := mustLoad(t, "testdata/scenes/"+firstScene(t))
	_, u0, err := BuildPrompt(s, VariantStyle, "en", st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(u0, "## Opening sentence") {
		t.Error("opening section without --opening")
	}
	s.Opening = true
	_, u1, _ := BuildPrompt(s, VariantStyle, "en", st)
	if !strings.Contains(u1, "## Opening sentence\nThe first sentence must hook a first-time reader") {
		t.Errorf("default opening missing:\n%s", u1)
	}
	st.Opening = "Open with the cold."
	_, u2, _ := BuildPrompt(s, VariantStyle, "en", st)
	if !strings.Contains(u2, "## Opening sentence\nOpen with the cold.") {
		t.Error("style opening not used")
	}
}
