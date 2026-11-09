package sema

import (
	"slices"
	"testing"
)

// `X sees p` (no colon): a fact object moves into the content (with), and an entity object stays.
func TestSeesFactObject(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
fact q = p
scene 1 at room
  enter a
  a sees p
  a sees b
  a sees q, b
`)
	noErrors(t, diags)
	want := map[string][2][]string{
		"t/t.s/1#a1": {{}, {"t/t.cast/p"}},
		"t/t.s/1#a2": {{"t/t.cast/b"}, {}},
		"t/t.s/1#a3": {{"t/t.cast/b"}, {"t/t.s/q"}},
	}
	for _, a := range w.Acts {
		if x, ok := want[a.ID]; ok && (!slices.Equal(a.Objects, x[0]) || !slices.Equal(a.With, x[1])) {
			t.Errorf("%s objects %v with %v, want %v %v", a.ID, a.Objects, a.With, x[0], x[1])
		}
	}
}

func TestSeesArgKind(t *testing.T) {
	_, diags := build(t, `module t.s
import t.cast
artifact l "L"
scene 1 at room
  enter a
  a sees l
`)
	if len(diags) != 1 || diags[0].Rule != "arg-kind" || diags[0].Severity != "error" {
		t.Errorf("diags = %v, want one arg-kind error", diags)
	}
}

// Background knowledge: a top-level `X believes P` and a `believes P` in a character/group declaration block are beliefs with no point (from the start).
func TestBackgroundBelief(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
a believes p
b doubts p
character d "D"
  believes p
group g "G"
  members c
  believes not p
`)
	noErrors(t, diags)
	var got []string
	for _, b := range w.Beliefs {
		if b.AtPt != "" || b.SceneID != "" {
			t.Errorf("%s %s: at %q scene %q, want none", b.EntityID, b.Verb, b.AtPt, b.SceneID)
		}
		got = append(got, b.EntityID+" "+b.Verb+" "+b.PropID)
	}
	want := []string{"t/t.cast/a believes t/t.cast/p", "t/t.cast/b doubts t/t.cast/p",
		"t/t.s/d believes t/t.cast/p", "t/t.s/g believes t/not t/t.cast/p"}
	if !slices.Equal(got, want) {
		t.Errorf("beliefs =\n %v\nwant\n %v", got, want)
	}
	_, diags = build(t, `module t.s
import t.cast
place yard "Y"
  believes p
`)
	if len(diags) != 1 || diags[0].Rule != "arg-kind" {
		t.Errorf("believes on a place: diags = %v, want one arg-kind", diags)
	}
}

// Offstage sends: only the messenger hears. The recipient knows when the messenger tells them.
func TestOffstageSendsAudience(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
event e "E"
  a sends b to c with p
`)
	noErrors(t, diags)
	if got := audience(w, "t/t.s/e#a1"); !slices.Equal(got, []string{"b"}) {
		t.Errorf("audience = %v, want [b]", got)
	}
}

// Task 2: one cannot die before being born. If both born and dies exist, it emits an order edge @x.born → @x.died (min 0)
// (even if they are in different modules). Without this edge the time solver cannot see the defect "death comes before birth".
func TestLifeOrder(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
born a when after start by 0y..0y
born b when after @a.born by 20y..20y
dies a when after @b.born by 10y..10y
dies b
`)
	noErrors(t, diags)
	var got []string
	for _, e := range w.TimeEdges {
		if e.Min != nil && *e.Min == 0 && e.Max == nil && e.From != "t/start" {
			got = append(got, e.From+">"+e.To)
		}
	}
	if !slices.Contains(got, "t/t.cast/a.born>t/t.cast/a.died") || !slices.Contains(got, "t/t.cast/b.born>t/t.cast/b.died") {
		t.Errorf("order edges = %v, want a.born>a.died and b.born>b.died", got)
	}
}

// Allows an object as the object of `shows`. An object does not become an audience.
func TestShowsThing(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
thing hairpin "H"
scene 1 at room
  enter a, b
  a shows b, hairpin
`)
	noErrors(t, diags)
	for _, a := range w.Acts {
		if a.Verb == "shows" && !slices.Equal(a.Objects, []string{"t/t.cast/b", "t/t.s/hairpin"}) {
			t.Errorf("objects = %v", a.Objects)
		}
	}
	for _, au := range w.Audience {
		if au.EntityID == "t/t.s/hairpin" {
			t.Errorf("thing in audience")
		}
	}
}
