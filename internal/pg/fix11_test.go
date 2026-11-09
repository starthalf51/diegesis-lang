package pg_test

import (
	"context"
	"slices"
	"testing"
)

// Task 11 (a): aliases are resolved inside a negation too. `not q` (q = p) ≡ `not p`, double negation `not r` (r = not p) ≡ p.
func TestCanonNegatedAlias(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body string
		want []string
	}{
		"not alias = not target": {`fact q = p
scene 1
  enter a, b, o
  o tells a: not q
  a tells b: not p
`, nil},
		"double negation through alias": {`fact r = not p
scene 1
  enter a, b, o
  o tells a: p
  a tells b: not r
`, nil},
		"different proposition still warns": {`fact q = p
fact s "S" true
scene 1
  enter a, b, o
  o tells a: not q
  a tells b: not s
`, []string{"t.s/1#a2/warn"}},
	} {
		id := compileLoad(t, pool, c.body)
		if got := of(lint(t, pool, id), "info-flow", id); !slices.Equal(got, c.want) {
			t.Errorf("%s: info-flow = %v, want %v", name, got, c.want)
		}
	}
	// The opposite verdict uses the same canonical form: someone who saw `not q` and says p is a contradiction.
	id := compileLoad(t, pool, `fact q = p
scene 1
  enter a, b
  a sees: not q
  a tells b: np
`)
	if got := of(lint(t, pool, id), "info-flow", id); got != nil {
		t.Errorf("seen not q, says np (= not p): info-flow = %v, want none", got)
	}
}

// Task 11 (b): a derive diagnostic (time-cycle) carries the target of the `when` statement that made the cycle edge. Suppression must match that target exactly.
func TestTimeCycleTarget(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body string
		want []string
	}{
		"no waiver": {`scene 1
  when after 2
scene 2
  when after 1
`, []string{"t.s/1/warn"}},
		"waiver on another statement does not waive": {`scene 1
  when after 2
scene 2
  when after 1
scene 3
  #! allow time-cycle: "other"
  when after 1
`, []string{"t.s/1/warn"}},
		"waiver on a cycle statement": {`scene 1
  when after 2
scene 2
  #! allow time-cycle: "loop"
  when after 1
`, []string{"t.s/2/warn/waived"}},
	} {
		id := compileLoad(t, pool, c.body)
		ds := lint(t, pool, id)
		if got := of(ds, "time-cycle", id); !slices.Equal(got, c.want) {
			t.Errorf("%s: time-cycle = %v, want %v", name, got, c.want)
		}
		for _, d := range ds {
			if d.Rule == "time-cycle" && d.TargetKind != "when" {
				t.Errorf("%s: target_kind = %q, want when", name, d.TargetKind)
			}
		}
	}
}

// Task 11 (c): a graph BELIEVES edge has the withdrawal point (until).
func TestGraphBelievesUntil(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b, o
  o tells a: p
  a tells b: p
  a doubts p
`)
	got := strs(t, pool, `SELECT replace(btrim(x.u::text, '"'), $1 || '/', '') FROM cypher('g_`+id+`', $$
	  MATCH (e:Entity)-[r:BELIEVES]->(:Fact) WHERE e.id ENDS WITH '/t.s/a' RETURN r.until $$) AS x(u agtype)`, id)
	if !slices.Equal(got, []string{"t.s/1#a2"}) {
		t.Errorf("BELIEVES.until = %v, want [t.s/1#a2]", got)
	}
}

// Task 11 (d): one info per document that 007 judged to be written knowing it was false (so an author's mistake shows too).
func TestInferredDeceptionInfo(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `a believes np
scene 1
  enter a
  a writes letter
`)
	if got := of(lint(t, pool, id), "inferred-deception", id); !slices.Equal(got, []string{"t.s/1#a1/info"}) {
		t.Errorf("inferred-deception = %v, want [t.s/1#a1/info]", got)
	}
	var n int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM deception WHERE work_id = $1`, id).Scan(&n)
	if n != 1 {
		t.Errorf("deception rows = %d, want 1", n)
	}
}
