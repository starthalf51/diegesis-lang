package pg_test

import (
	"context"
	"slices"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

// If withhold's until is a beat (scene#beat), the scene is found by prefix. LIKE reads the '_' of a scene ID as a wildcard,
// so a_1 would catch aX1#b: a fact resolved in aX1 (earlier) must not stay hidden through a_1 (later).
func TestRenderWithheldUntilBeatIsPrefix(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene aX1
  enter a, b
  beat b:
    a tells b: p
scene mid
  withhold p until aX1.b
  enter a, b
  a tells b: np
scene a_1
  enter a, b
  a tells b: np
`)
	b, err := pg.RenderScene(context.Background(), pool, id, "mid")
	if err != nil {
		t.Fatal(err)
	}
	s, err := render.ParseScene(b)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if len(s.Withheld) != 0 {
		t.Errorf("withheld = %v, want none (released in aX1)", s.Withheld)
	}
	// The fact is carried by the author's ID (np) and the canonical proposition (not p) is placed only in canon.
	if !slices.ContainsFunc(s.Facts, func(f render.Fact) bool { return f.ID == "np" && f.Canon == "not p" }) {
		t.Errorf("facts = %+v, want authored np with canon \"not p\"", s.Facts)
	}
}
