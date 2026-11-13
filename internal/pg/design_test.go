package pg_test

import (
	"context"
	"slices"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

func TestDesignBlock(t *testing.T) {
	pool := open(t)
	// no design block anywhere: silent
	id := compileLoad(t, pool, "scene 1\n  enter a\nscene 2\n  enter a\n")
	if got := of(lint(t, pool, id), "scene-no-turn", id); len(got) != 0 {
		t.Errorf("no design: got %v", got)
	}
	// design on one scene only: the other visible scene without a turn warns
	id = compileLoad(t, pool, "scene 1\n  enter a\n  design\n    goal a: \"cross the river\"\n    turn \"fear\" -> courage\n    tempo ellipsis\nscene 2\n  enter a\n")
	if got := of(lint(t, pool, id), "scene-no-turn", id); !slices.Equal(got, []string{"t.s/2/warn"}) {
		t.Errorf("partial design: got %v", got)
	}
	b, err := pg.RenderScene(context.Background(), pool, id, "1")
	if err != nil {
		t.Fatal(err)
	}
	s, err := render.ParseScene(b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Goal.Who != "a" || s.Goal.Text != "cross the river" || s.Turn.From != "fear" || s.Turn.To != "courage" || s.Tempo != "ellipsis" {
		t.Errorf("scene design: %+v %+v %q", s.Goal, s.Turn, s.Tempo)
	}
	b, _ = pg.RenderScene(context.Background(), pool, id, "2")
	if s2, _ := render.ParseScene(b); s2.Goal.Text != "" || s2.Turn.From != "" || s2.Tempo != "" {
		t.Errorf("scene 2 should carry no design: %+v", s2)
	}
}

const manyTerms = `term t1 "일호" = "d1"
term t2 "이호" = "d2"
term t3 "삼호" = "d3"
term t4 "사호" = "d4"
`

// term-overload: a scene that first explains more than 3 terms; recaps do not count.
func TestTermOverload(t *testing.T) {
	pool := open(t)
	for _, tc := range []struct {
		name, scenes string
		want         []string
	}{
		{"four new", "scene 1\n  enter a\n  render\n    explain t1, t2, t3, t4\n", []string{"t.s/1/warn"}},
		{"three new", "scene 1\n  enter a\n  render\n    explain t1, t2, t3\n", nil},
		{"recap not counted", "scene 1\n  enter a\n  render\n    explain t1, t2, t3\nscene 2\n  enter a\n  render\n    recap t1, t2, t3, t4\n", nil},
		{"re-explain not new", "scene 1\n  enter a\n  render\n    explain t1, t2, t3\nscene 2\n  enter a\n  render\n    explain t1, t2, t3, t4\n", nil},
	} {
		id := compileLoad(t, pool, manyTerms+tc.scenes)
		if got := of(lint(t, pool, id), "term-overload", id); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// scene-no-turn is checked only in modules that contain a design block.
func TestSceneNoTurnModuleScope(t *testing.T) {
	pool := open(t)
	id := compileLoadFiles(t, pool,
		"scene 1\n  enter a\n  design\n    turn \"x\" -> y\nscene 2\n  enter a\n",
		"module t.s2\nimport t.s\nscene 3\n  when after 1\n  enter a\n")
	if got := of(lint(t, pool, id), "scene-no-turn", id); !slices.Equal(got, []string{"t.s/2/warn"}) {
		t.Errorf("got %v, want only t.s/2", got)
	}
}
