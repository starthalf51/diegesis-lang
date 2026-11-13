package pg_test

import (
	"slices"
	"testing"
)

// pov-offstage: render pov X, but X never enters the scene (any presence row counts; a mention is not presence).
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
		{"qualified pov enters", "scene 1\n  enter a, b\n  render\n    pov t.s/a\n", nil, nil},
		{"qualified pov thinks", "scene 1\n  enter a, b\n  render\n    pov t.s/a\n    narration first-person\n  a thinks: \"hm\"\n", nil, nil},
		{"unresolved role thinks", "role chief \"Chief\"\nscene 1\n  enter a, b\n  render\n    pov a\n    narration first-person\n  role:chief thinks: \"hm\"\n", nil, nil},
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

// scene-ends-passive: a design scene with a pov must end on an active act by the pov character.
func TestSceneEndsPassive(t *testing.T) {
	pool := open(t)
	const hdr = "scene 1\n  enter a, b\n  render\n    pov a\n  design\n    turn \"fear\" -> courage\n"
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"pov thinks last", hdr + "  a thinks: \"hm\"\n", []string{"t.s/1/warn"}},
		{"other does last", hdr + "  a does go\n  b does leave\n", []string{"t.s/1/warn"}},
		{"pov does last", hdr + "  b does leave\n  a does go\n", nil},
		{"no design", "scene 1\n  enter a, b\n  render\n    pov a\n  a thinks: \"hm\"\n", nil},
		{"no pov", "scene 1\n  enter a, b\n  design\n    turn \"fear\" -> courage\n  a thinks: \"hm\"\n", nil},
		{"beat last active", hdr + "  a thinks: \"hm\"\n  beat x:\n    a does go\n", nil},
		{"beat last passive", hdr + "  a does go\n  beat x:\n    a thinks: \"hm\"\n", []string{"t.s/1/warn"}},
	} {
		id := compileLoad(t, pool, tc.src)
		if got := of(lint(t, pool, id), "scene-ends-passive", id); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
