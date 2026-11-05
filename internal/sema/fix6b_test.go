package sema

import (
	"slices"
	"testing"
)

// `X sees p`(콜론 없음): 사실 객체는 내용(with)으로 옮기고, 엔티티 객체는 그대로 둔다.
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

// 배경 지식: 최상위 `X believes P` 와 인물·그룹 선언 블록의 `believes P` 는 시점 없는(처음부터) 믿음이다.
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

// 무대 밖 sends: 전령만 듣는다. 받는 사람은 전령이 말해 줄 때 안다.
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
