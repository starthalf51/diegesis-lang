package pg_test

import (
	"slices"
	"testing"
)

// Task 9(적벽): 읽은 문서도 증언이다. 의심은 그 앞에 읽은 문서의 믿음도 거둔다(47.14-16 조조가 황개의 서신을 읽고 의심한다).
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

// Task 9(적벽): 거짓인 줄 알면서 쓴 문서(위조 서신, 거짓 항복 서신)는 글로 한 거짓말이다.
// 쓴 사람이 쓰는 시점에 ¬P 를 확정적으로 알면 deception 한 행이 생기고, letter-unknown-info·belief-contradiction 은 내지 않는다.
// ¬P 를 모르는 채 쓴 P 는 여전히 letter-unknown-info 다.
func TestKnowinglyFalseWritesAreDeception(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body    string
		dec     []string // deception: 행위/명제
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
