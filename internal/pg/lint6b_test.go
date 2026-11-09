package pg_test

import (
	"slices"
	"testing"
)

// `macbeth sees cawdor_traitor` (no colon) also makes a perception belief.
func TestSeesWithoutColonBelieves(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  a sees p
  a tells b: p
`)
	got := strs(t, pool, `SELECT entity_id FROM believes WHERE work_id = $1 AND prop_id = $2 AND value`, id, id+"/t.s/p")
	if !slices.Contains(got, id+"/t.s/a") {
		t.Errorf("believers of p = %v, want a (sees)", got)
	}
	if got := of(lint(t, pool, id), "info-flow", id); len(got) != 0 {
		t.Errorf("info-flow = %v, want none", got)
	}
}

// Background knowledge (believes in top-level/declaration blocks) is known from the start. A group's beliefs are inherited by its members.
func TestBackgroundKnowledge(t *testing.T) {
	pool := open(t)
	for name, body := range map[string]string{
		"top-level": `a believes p
scene 1
  enter a, b
  a tells b: p
`,
		"declaration": `character d "D"
  believes p
scene 1
  enter d, b
  d tells b: p
`,
		"group": `group g "G"
  members h
  believes p
group h "H"
  members a
scene 1
  enter a, b
  a tells b: p
`,
	} {
		id := compileLoad(t, pool, body)
		if got := of(lint(t, pool, id), "info-flow", id); len(got) != 0 {
			t.Errorf("%s: info-flow = %v, want none", name, got)
		}
	}
	// Control: the same situation without background knowledge is a warn
	id := compileLoad(t, pool, `group g "G"
  members a
scene 1
  enter a, b
  a tells b: p
`)
	if got := of(lint(t, pool, id), "info-flow", id); !slices.Equal(got, []string{"t.s/1#a1/warn"}) {
		t.Errorf("no background: info-flow = %v, want one warn", got)
	}
}

// The recipient of an offstage sends does not know until the messenger tells them.
func TestOffstageSendsRecipientUnaware(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `event ev "E"
  o tells a: p
  a sends m to b with p
scene 1
  when after ev
  enter b, c
  b tells c: p
`)
	ds := lint(t, pool, id)
	for rule, want := range map[string][]string{
		"info-flow":           {"t.s/1#a1/warn"},
		"undelivered-message": {"t.s/ev#a2/warn"},
	} {
		if got := of(ds, rule, id); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", rule, got, want)
		}
	}
}
