package pg_test

import (
	"slices"
	"testing"
)

// Task 9 (Red Cliffs): a document read is also testimony. Doubt withdraws the belief from a document read before it too (47.14-16 Cao Cao reads Huang Gai's letter and doubts it).
func TestDoubtsCancelReads(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  a reads letter
  a doubts p
  a tells b: p
`)
	if got := of(lint(t, pool, id), "info-flow", id); !slices.Equal(got, []string{"t.s/1#a2/warn"}) {
		t.Errorf("info-flow = %v, want [t.s/1#a2/warn]", got)
	}
}

// Task 9 (Red Cliffs): a document written knowing it is false (a forged letter, a false surrender letter) is a lie told in writing.
// If the writer definitely knows ¬P at the time of writing, one deception row is made, and letter-unknown-info and belief-contradiction do not emit.
// A P written without knowing ¬P is still letter-unknown-info.
func TestKnowinglyFalseWritesAreDeception(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body    string
		dec     []string // deception: act/proposition
		unknown []string // letter-unknown-info
		contra  []string // belief-contradiction
	}{
		"forger believes the opposite": {`a believes np
scene 1
  enter a
  a writes letter
`, []string{"t.s/1#a1 t.s/p"}, nil, nil},
		"forger saw the opposite": {`scene 1
  enter a
  a sees: np
  a writes letter
`, []string{"t.s/1#a2 t.s/p"}, nil, nil},
		"opposite learned only after writing": {`scene 1
  enter a
  a writes letter
  a sees: np
`, nil, []string{"t.s/1#a1/warn"}, nil},
		"honest writer": {`scene 1
  enter a, o
  o tells a: p
  a writes letter
`, nil, nil, nil},
	} {
		id := compileLoad(t, pool, c.body)
		ds := lint(t, pool, id)
		dec := strs(t, pool, `SELECT replace(act_id, $1||'/', '')||' '||replace(prop_id, $1||'/', '') FROM deception WHERE work_id=$1 ORDER BY 1`, id)
		if !slices.Equal(dec, c.dec) {
			t.Errorf("%s: deception = %v, want %v", name, dec, c.dec)
		}
		if got := of(ds, "letter-unknown-info", id); !slices.Equal(got, c.unknown) {
			t.Errorf("%s: letter-unknown-info = %v, want %v", name, got, c.unknown)
		}
		if got := of(ds, "belief-contradiction", id); !slices.Equal(got, c.contra) {
			t.Errorf("%s: belief-contradiction = %v, want %v", name, got, c.contra)
		}
	}
}
