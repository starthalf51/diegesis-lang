package pg_test

import (
	"context"
	"slices"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

// TestRenderSceneJSON: the render request for Macbeth 1.3 is read strictly as render.Scene and passes Validate,
// and the cast and act counts equal the original rows (acts = act rows + one per entry/exit bound).
func TestRenderSceneJSON(t *testing.T) {
	pool := open(t)
	w, id := corpusIR(t, "macbeth")
	cleanup(t, pool, id)
	ctx := context.Background()
	if err := pg.Load(ctx, pool, w); err != nil {
		t.Fatal(err)
	}
	b, err := pg.RenderScene(ctx, pool, id, "1.3")
	if err != nil {
		t.Fatal(err)
	}
	s, err := render.ParseScene(b)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	scene := id + "/macbeth.act1/1.3"
	var acts, bounds, cast int
	pool.QueryRow(ctx, `SELECT count(*) FROM act WHERE scene_id = $1`, scene).Scan(&acts)
	pool.QueryRow(ctx, `SELECT (SELECT count(DISTINCT from_pt) FROM presence p JOIN scene s ON s.id = p.scene_id
	                             WHERE p.scene_id = $1 AND p.from_pt <> s.start_pt)
	                         + (SELECT count(DISTINCT to_pt) FROM presence WHERE scene_id = $1)`, scene).Scan(&bounds)
	pool.QueryRow(ctx, `SELECT count(DISTINCT entity_id) FROM presence WHERE scene_id = $1`, scene).Scan(&cast)
	got := 0
	var verbs []string
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			got++
			verbs = append(verbs, a.Verb)
		}
	}
	if acts != 22 || got != acts+bounds { // 1.3.74 and 1.3.123 split per fact, 19 → 22
		t.Errorf("acts: got %d, want %d rows + %d presence bounds", got, acts, bounds)
	}
	if len(s.Cast) != cast {
		t.Errorf("cast: got %d, want %d", len(s.Cast), cast)
	}
	if s.ID != "1.3" || s.Directives.POV != "macbeth" || s.Directives.Pace != "slow" || s.Directives.Tension != 0.8 {
		t.Errorf("header: id=%q directives=%+v", s.ID, s.Directives)
	}
	if !slices.Equal(s.OnStage, []string{"witch1", "witch2", "witch3", "witches"}) || verbs[1] != "enter" {
		t.Errorf("on_stage=%v verbs=%v", s.OnStage, verbs)
	}
	if e, _ := s.Entity("macbeth"); len(e.Names["ko"]) == 0 || e.Names["ko"][0] != "맥베스" {
		t.Errorf("macbeth names: %v", e.Names)
	}
	// The POV character knows at the start of 1.3 that he is Thane of Glamis. The prophecy that he will be king is first heard in 1.3.
	if !slices.Contains(s.POVBeliefs, "holds(thane_of_glamis,macbeth)") || slices.Contains(s.POVBeliefs, "king_hereafter") {
		t.Errorf("pov_beliefs: %v", s.POVBeliefs)
	}
	var proph []string
	for _, bt := range s.Beats {
		if bt.ID == "prophecy" {
			for _, a := range bt.Acts {
				proph = append(proph, a.Actor+":"+a.Fact)
			}
		}
	}
	if !slices.Equal(proph, []string{"witch1:holds(thane_of_glamis,macbeth)", "witch2:holds(thane_of_cawdor,macbeth)",
		"witch3:king_hereafter", "witch3:banquo_line"}) {
		t.Errorf("prophecy beat: %v", proph)
	}
	// Fact IDs and sentences are the author's (king_hereafter, cawdor_in_favour); the canonical proposition goes only in canon, for comparison.
	if f, _ := s.FactByID("king_hereafter"); f.Text != "Macbeth is the King" || f.Canon != "holds(king,macbeth)" {
		t.Errorf("king_hereafter: %+v", f)
	}
	if f, _ := s.FactByID("cawdor_in_favour"); f.Canon != "not cawdor_traitor" || f.True {
		t.Errorf("cawdor_in_favour: %+v", f)
	}
	if _, ok := s.FactByID("not cawdor_traitor"); ok {
		t.Error("catalog lists the canonical id not cawdor_traitor")
	}
	if _, err := pg.RenderScene(ctx, pool, id, "nope"); err == nil {
		t.Error("unknown scene: want error")
	}
}

// The other three scenes used for the render verdict (expect.yaml's render_scene) are also read strictly and pass Validate.
func TestRenderScenesValidate(t *testing.T) {
	pool := open(t)
	ctx := context.Background()
	for work, scene := range map[string]string{"chibi": "45.88", "bible": "mark_denial", "odyssey": "19.1"} {
		w, id := corpusIR(t, work)
		cleanup(t, pool, id)
		if err := pg.Load(ctx, pool, w); err != nil {
			t.Fatal(err)
		}
		b, err := pg.RenderScene(ctx, pool, id, scene)
		if err != nil {
			t.Fatal(err)
		}
		s, err := render.ParseScene(b)
		if err != nil {
			t.Errorf("%s %s: %v\n%s", work, scene, err, b)
			continue
		}
		var facts int
		for _, bt := range s.Beats {
			for _, a := range bt.Acts {
				if a.Fact != "" {
					facts++
				}
			}
		}
		if facts == 0 || len(s.Cast) == 0 {
			t.Errorf("%s %s: %d acts with facts, %d cast", work, scene, facts, len(s.Cast))
		}
	}
}

// Render cards carry the terms explained in this scene (new) and in earlier told scenes (known); `discourse after` is honored.
func TestRenderSceneTerms(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, terms+"scene 1\n  enter a, b\n  render\n    explain jokbo\n  a tells b: fj\nscene 2\n  enter a\n  render\n    explain chip\n")
	card := func(sc string) render.Scene {
		b, err := pg.RenderScene(context.Background(), pool, id, sc)
		if err != nil {
			t.Fatal(err)
		}
		s, err := render.ParseScene(b)
		if err != nil {
			t.Fatalf("%v\n%s", err, b)
		}
		return s
	}
	s1, s2 := card("1"), card("2")
	want := func(got []render.Term, surf, gloss string) {
		t.Helper()
		if len(got) != 1 || got[0].Surface != surf || got[0].Gloss != gloss {
			t.Errorf("got %+v, want %s", got, surf)
		}
	}
	want(s1.TermsNew, "족보", "모든 시민의 신원과 배급을 묶은 국가 기록")
	want(s2.TermsNew, "칩", "손목에 박힌 신분 칩")
	want(s2.TermsKnown, "족보", "모든 시민의 신원과 배급을 묶은 국가 기록")
	if len(s1.TermsKnown) != 0 {
		t.Errorf("s1 known: %+v", s1.TermsKnown)
	}
}

// A term explained again in a later scene is known there, not new (plan 16); a scene between them still knows it.
func TestRenderSceneTermsReexplain(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, terms+"scene 1\n  enter a, b\n  render\n    explain jokbo\n  a tells b: fj\nscene 2\n  enter a\nscene 3\n  enter a\n  render\n    explain jokbo\n")
	surf := func(ts []render.Term) (out []string) {
		for _, x := range ts {
			out = append(out, x.Surface)
		}
		return
	}
	for _, tc := range []struct {
		sc          string
		isNew, know []string
	}{{"1", []string{"족보"}, nil}, {"2", nil, []string{"족보"}}, {"3", nil, []string{"족보"}}} {
		b, err := pg.RenderScene(context.Background(), pool, id, tc.sc)
		if err != nil {
			t.Fatal(err)
		}
		s, err := render.ParseScene(b)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(surf(s.TermsNew), tc.isNew) || !slices.Equal(surf(s.TermsKnown), tc.know) {
			t.Errorf("scene %s: new %v known %v, want %v / %v", tc.sc, surf(s.TermsNew), surf(s.TermsKnown), tc.isNew, tc.know)
		}
	}
}
