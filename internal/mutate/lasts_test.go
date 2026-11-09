package mutate_test

import (
	"slices"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/mutate"
)

// A lasts edge (scene start→end) is a shift-duration target and not a swap-order target (flipped, it is the same edge).
func TestLastsMutants(t *testing.T) {
	w := build(t, map[string]string{"t.dieg": "module t.s\nwork t \"T\"\nscene 1\n  lasts 0d..1d\n"})
	ms := mutate.Generate(w, 1, 0)
	if got := targets(ms, "shift-duration"); !slices.Equal(got, []string{"1.start 1.end"}) {
		t.Errorf("shift-duration targets = %v", got)
	}
	if got := targets(ms, "swap-order"); len(got) != 0 {
		t.Errorf("swap-order targets = %v, want none", got)
	}
}
