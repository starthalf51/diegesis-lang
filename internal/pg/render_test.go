package pg_test

import (
	"context"
	"slices"
	"testing"

	"story-lang/internal/pg"
	"story-lang/internal/render"
)

// TestRenderSceneJSON: 맥베스 1.3 의 렌더 요청이 render.Scene 으로 엄격하게 읽히고 Validate 를 통과하며,
// 출연진·행위 수가 원본 행과 같다(행위 = act 행 + 입·퇴장 경계마다 하나).
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
	if acts != 18 || got != acts+bounds {
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
	// 시점 인물은 1.3 시작에 글래미스 영주임을 안다. 왕이 된다는 예언은 1.3 에서 처음 듣는다.
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
	// 사실 ID·문장은 지은이의 것(king_hereafter, cawdor_in_favour), 정규 명제는 비교용 canon 에만.
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

// 렌더 판정에 쓰는 나머지 세 장면(expect.yaml 의 render_scene)도 엄격하게 읽히고 Validate 를 통과한다.
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
