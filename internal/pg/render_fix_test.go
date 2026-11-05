package pg_test

import (
	"context"
	"slices"
	"testing"

	"story-lang/internal/pg"
	"story-lang/internal/render"
)

// withhold 의 until 이 비트(장면#비트)면 그 장면을 접두어로 찾는다. LIKE 는 장면 ID 의 '_' 를 와일드카드로 읽어
// a_1 이 aX1#b 를 잡는다: aX1(앞) 에서 풀린 사실이 a_1(뒤) 까지 숨겨진 것으로 남으면 안 된다.
func TestRenderWithheldUntilBeatIsPrefix(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene aX1
  enter a, b
  beat b:
    a tells b: p
scene mid
  withhold p until aX1.b
  enter a, b
  a tells b: np
scene a_1
  enter a, b
  a tells b: np
`)
	b, err := pg.RenderScene(context.Background(), pool, id, "mid")
	if err != nil {
		t.Fatal(err)
	}
	s, err := render.ParseScene(b)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if len(s.Withheld) != 0 {
		t.Errorf("withheld = %v, want none (released in aX1)", s.Withheld)
	}
	// 사실은 지은이의 ID(np)로 싣고 정규 명제(not p)는 canon 에만 둔다.
	if !slices.ContainsFunc(s.Facts, func(f render.Fact) bool { return f.ID == "np" && f.Canon == "not p" }) {
		t.Errorf("facts = %+v, want authored np with canon \"not p\"", s.Facts)
	}
}
