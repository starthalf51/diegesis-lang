package sema

import (
	"slices"
	"strings"
	"testing"
)

func TestDesignSema(t *testing.T) {
	w, diags := build(t, "module t.d\nimport t.cast\nscene 1 at room\n  design\n    goal a: \"reach the gate\"\n    turn \"despair\" -> hope\n    tempo summary\n    clock \"before the bell\"\n")
	noErrors(t, diags)
	got := map[string][]string{}
	for _, d := range w.Directives {
		got[d.Key] = d.Values
	}
	if !slices.Equal(got["design.goal"], []string{"t/t.cast/a", "reach the gate"}) ||
		!slices.Equal(got["design.turn"], []string{"despair", "hope"}) || !slices.Equal(got["design.tempo"], []string{"summary"}) ||
		!slices.Equal(got["design.clock"], []string{"before the bell"}) {
		t.Errorf("directives: %v", got)
	}
	for _, tc := range []struct{ body, want string }{
		{"    goal room: \"x\"\n", "character"},
		{"    tempo fast\n", "tempo"},
	} {
		_, diags := build(t, "module t.d\nimport t.cast\nscene 1 at room\n  design\n"+tc.body)
		found := false
		for _, d := range diags {
			found = found || d.Severity == "error" && strings.Contains(d.Msg, tc.want)
		}
		if !found {
			t.Errorf("%q: want an error mentioning %q, got %v", tc.body, tc.want, diags)
		}
	}
}
