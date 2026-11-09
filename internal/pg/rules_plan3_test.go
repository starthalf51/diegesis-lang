package pg_test

import (
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"slices"
	"testing"
)

const bellCast = `place tower "T"
place staffroom "S"
thing bell "B"
  attr bell.at = tower
`

// Warn if the object's location (X.at) differs from the scene's place. Quiet if the same place.
func TestObjectLocationWarns(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, bellCast+`scene 1 at staffroom
  enter a
  a does ring bell
scene 2 at tower
  when after 1
  enter a
  a does ring bell
`)
	if got := of(lint(t, pool, id), "object-location", id); !slices.Equal(got, []string{"t.s/1#a1/warn"}) {
		t.Errorf("object-location = %v, want scene 1 only", got)
	}
}

// An act after moving it by attr inside the scene does not warn.
func TestObjectLocationMovedThing(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, bellCast+`scene 1 at staffroom
  enter a
  attr bell.at = staffroom
  a does ring bell
`)
	if got := of(lint(t, pool, id), "object-location", id); len(got) != 0 {
		t.Errorf("object-location = %v, want none", got)
	}
}

// A reveal of a fact declared false warns. A true fact is quiet.
func TestRevealFalseWarns(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `place room "R"
fact x "X" false
scene 1 at room
  enter a
  reveal x
  reveal p
`)
	if got := of(lint(t, pool, id), "reveal-false", id); len(got) != 1 {
		t.Errorf("reveal-false = %v, want one", got)
	}
}

func msgs(ds []pg.Diagnostic, rule string) []string {
	var out []string
	for _, d := range ds {
		if d.Rule == rule {
			out = append(out, d.Message)
		}
	}
	slices.Sort(out)
	return out
}

// Message strings and boundaries: does is the action name, an ordinary verb is the verb; a character object, an object with no location, and a location decided later are quiet.
func TestPlan3RuleMessagesAndEdges(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, bellCast+`thing cup "C"
fact x "X" false
scene 1 at staffroom
  enter a
  a does ring bell
  a shows bell
  a does look b
  a does drink cup
  reveal x
`)
	_ = id
	ds := lint(t, pool, id)
	want := []string{"a ring bell in staffroom, but bell is at tower", "a shows bell in staffroom, but bell is at tower"}
	if got := msgs(ds, "object-location"); !slices.Equal(got, want) {
		t.Errorf("object-location = %q, want %q", got, want)
	}
	if got := msgs(ds, "reveal-false"); !slices.Equal(got, []string{"reveal of false fact x"}) {
		t.Errorf("reveal-false = %q", got)
	}
}

// If X.at is decided later than the act, it is not valid for that act.
func TestObjectLocationLaterAt(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `place tower "T"
place staffroom "S"
thing bell "B"
scene 1 at staffroom
  enter a
  a does ring bell
scene 2 at tower
  when after 1
  attr bell.at = tower
`)
	if got := of(lint(t, pool, id), "object-location", id); len(got) != 0 {
		t.Errorf("object-location = %v, want none", got)
	}
}
