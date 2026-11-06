package pg_test

import (
	"diegesis/internal/pg"
	"slices"
	"testing"
)

const bellCast = `place tower "T"
place staffroom "S"
thing bell "B"
  attr bell.at = tower
`

// 사물 위치(X.at)와 장면 장소가 다르면 경고. 같은 장소면 조용하다.
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

// 장면 안에서 attr 로 옮긴 뒤의 행위는 경고하지 않는다.
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

// 거짓으로 선언된 사실의 reveal 은 경고. 참 사실은 조용하다.
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

// 메시지 문자열과 경계: does 는 동작 이름, 일반 동사는 동사, 인물 목적어·위치 없는 사물·나중에 정해진 위치는 조용하다.
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

// X.at 가 행위보다 나중에 정해지면 그 행위에는 유효하지 않다.
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
