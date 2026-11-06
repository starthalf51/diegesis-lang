package mutate_test

import (
	"slices"
	"testing"

	"diegesis/internal/mutate"
)

// lasts 간선(장면 start→end)은 shift-duration 대상이고, swap-order 대상이 아니다(뒤집어도 같은 간선이다).
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
