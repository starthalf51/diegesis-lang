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
