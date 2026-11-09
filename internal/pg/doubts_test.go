package pg_test

import (
	"slices"
	"testing"
)

// Task 9 engine fix: `X doubts P` withdraws a P heard before it (from that point). A later source (sees, heard again, reads,
// explicit believes) makes them believe again. Perception and background knowledge are not withdrawn. A background doubts is not believed from the start (no row).
func TestDoubtsCancelHearsay(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body string
		want map[string][]string // rule → diagnostics
	}{
		"doubt cancels earlier hearsay": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  a tells b: p
`, map[string][]string{"info-flow": {"t.s/1#a2/warn"}}},
		"doubt cancels hearsay for because": {`scene 1
  enter a, o
  o tells a: p
  a doubts p
  a does flee because p
`, map[string][]string{"because-unbelieved": {"t.s/1#a2/warn"}}},
		"doubt cancels hearsay for writes": {`scene 1
  enter a, o
  o lies a: p
  a doubts p
  a writes letter
`, map[string][]string{"letter-unknown-info": {"t.s/1#a2/warn"}}},
		"use before the doubt still knows": {`scene 1
  enter a, b, o
  o tells a: p
  a tells b: p
  a doubts p
`, map[string][]string{"info-flow": nil}},
		"later tells re-establishes": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  o tells a: p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"later sees re-establishes": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  a sees p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"later reads re-establishes": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  a reads letter
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"later believes re-establishes": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  a believes p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"perception is not cancelled": {`scene 1
  enter a, b
  a sees p
  a doubts p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"background knowledge is not cancelled": {`a believes p
scene 1
  enter a, b
  a doubts p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"background doubt: later hearsay still counts": {`a doubts p
scene 1
  enter a, b, o
  o tells a: p
  a tells b: p
`, map[string][]string{"info-flow": nil}},
		"background doubt: nothing believed": {`a doubts p
scene 1
  enter a, b
  a tells b: p
`, map[string][]string{"info-flow": {"t.s/1#a1/warn"}}},
		"doubt of another proposition does nothing": {`scene 1
  enter a, b, o
  o tells a: p
  a doubts np
  a tells b: p
`, map[string][]string{"info-flow": nil}},
	} {
		id := compileLoad(t, pool, c.body)
		ds := lint(t, pool, id)
		for rule, want := range c.want {
			if got := of(ds, rule, id); !slices.Equal(got, want) {
				t.Errorf("%s: %s = %v, want %v", name, rule, got, want)
			}
		}
	}
}

// Keep the withdrawn row but record the doubt point in until_pt (so queries and explanations can see "until when they believed").
func TestDoubtsUntilPt(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  o tells a: p
`)
	got := strs(t, pool, `SELECT replace(b.cause_act, $1||'/', '')||' '||coalesce(replace(b.until_pt, $1||'/', ''), '-')
		FROM believes b WHERE b.work_id=$1 AND b.entity_id=$2 ORDER BY 1`, id, id+"/t.s/a")
	want := []string{"t.s/1#a1 t.s/1#a1", "t.s/1#a2 -"}
	if !slices.Equal(got, want) {
		t.Errorf("a's beliefs = %v, want %v", got, want)
	}
}
