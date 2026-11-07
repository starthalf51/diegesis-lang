package pg_test

import (
	"testing"
)

const goneCast = `place room "R"
character minjae "M"
`

// 떠난 인물(status = gone)이 이후 장면에서 말하면 경고한다.
func TestStaysGoneWarns(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, goneCast+`scene 2 at room
  enter a
  attr minjae.status = gone
scene 6 at room
  when after 2
  enter a
  minjae says a: "나 왔어."
`)
	got := msgs(lint(t, pool, id), "stays-gone")
	if len(got) != 1 || got[0] != "minjae says after leaving (status gone at t.dieg:17)" {
		t.Errorf("stays-gone = %q, want one says warning", got)
	}
}

// present 로 되돌린 뒤의 행위는 경고하지 않는다.
func TestStaysGoneReturn(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, goneCast+`scene 2 at room
  enter a
  attr minjae.status = gone
scene 4 at room
  when after 2
  enter a
  attr minjae.status = present
scene 6 at room
  when after 4
  enter a
  minjae says a: "나 왔어."
`)
	ds := lint(t, pool, id)
	if got := append(of(ds, "stays-gone", id), of(ds, "attribute-conflict", id)...); len(got) != 0 {
		t.Errorf("stays-gone = %v, want none", got)
	}
}

// 떠난 인물의 enter 도 경고한다.
func TestStaysGoneEnter(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, goneCast+`scene 2 at room
  enter a
  attr minjae.status = gone
scene 6 at room
  when after 2
  enter minjae
`)
	if got := of(lint(t, pool, id), "stays-gone", id); len(got) != 1 {
		t.Errorf("stays-gone = %v, want one", got)
	}
}

