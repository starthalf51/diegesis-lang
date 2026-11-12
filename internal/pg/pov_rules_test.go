package pg_test

import (
	"slices"
	"testing"
)

// pov-offstage: render pov X, but X is not on stage (entered, not yet exited) in the scene; a mention is not presence.
// pov-mind-leak: in a first-person scene, an act of `thinks` by someone other than the pov character.
func TestPovRules(t *testing.T) {
	pool := open(t)
	for _, tc := range []struct {
		name, scene      string
		offstage, leaked []string
	}{
		{"pov absent", "scene 1\n  enter b\n  render\n    pov a\n", []string{"t.s/1/error"}, nil},
		{"mention only", "scene 1\n  enter b\n  mention a\n  render\n    pov a\n", []string{"t.s/1/error"}, nil},
		{"pov present", "scene 1\n  enter a, b\n  render\n    pov a\n", nil, nil},
		{"first person, other thinks", "scene 1\n  enter a, b\n  render\n    pov a\n    narration first-person\n  b thinks: \"hm\"\n", nil, []string{"t.s/1#a1/error"}},
		{"first person, pov thinks", "scene 1\n  enter a, b\n  render\n    pov a\n    narration first-person\n  a thinks: \"hm\"\n", nil, nil},
		{"third person, other thinks", "scene 1\n  enter a, b\n  render\n    pov a\n    narration third-person\n  b thinks: \"hm\"\n", nil, nil},
		{"no pov", "scene 1\n  enter a, b\n  render\n    narration first-person\n  b thinks: \"hm\"\n", nil, nil},
	} {
		id := compileLoad(t, pool, tc.scene)
		ds := lint(t, pool, id)
		if got := of(ds, "pov-offstage", id); !slices.Equal(got, tc.offstage) {
			t.Errorf("%s: pov-offstage got %v, want %v", tc.name, got, tc.offstage)
		}
		if got := of(ds, "pov-mind-leak", id); !slices.Equal(got, tc.leaked) {
			t.Errorf("%s: pov-mind-leak got %v, want %v", tc.name, got, tc.leaked)
		}
	}
}
