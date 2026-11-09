package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// TestCorpusPromptsUnchanged: a cached corpus render's RenderHash must still equal the hash of today's prompt.
func TestCorpusPromptsUnchanged(t *testing.T) {
	raw, err := os.ReadFile("../../out/render/macbeth/1.3.en-doyle.json")
	if err != nil {
		t.Skip("no cached render record")
	}
	var r render.Render
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	_, s, err := sceneRequest(filepath.Join("..", "..", "corpus", "macbeth"), "1.3")
	if err != nil {
		t.Skip("no PG: ", err)
	}
	b, err := backend(filepath.Join("..", "..", "styles"), r.Backend)
	if err != nil {
		t.Skip(err)
	}
	got, err := render.RenderHash(s, b)
	if err != nil {
		t.Fatal(err)
	}
	if got != r.RenderHash {
		t.Errorf("RenderHash %s != cached %s", got, r.RenderHash)
	}
}
