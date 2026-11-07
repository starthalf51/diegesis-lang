package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diegesis/internal/render"
)

var abText = map[string]string{"f1": "문이 열렸다", "f2": "등불이 꺼졌다", "f3": "바람이 불었다"}

// abFake: 글쓰기 호출은 user 에 나온 사실 문장마다 "F<id> 가온이 보았다." 를 쓰고, 문체·연결은 받은 글을 그대로 돌려준다.
// 재추출은 글에 "F<id> 가온이" 가 있으면 그 사실의 sees 행위를 낸다. haiku 는 비트 1에서 한 번 환각한다(재시도를 본다).
type abFake struct {
	model string
	n     int
}

func (f *abFake) Complete(_ context.Context, system, user string) (render.Result, error) {
	f.n++
	ok := func(t string) (render.Result, error) {
		return render.Result{Text: t, StopReason: "end_turn", Usage: render.Usage{Input: 10, Output: 5}}, nil
	}
	switch {
	case strings.Contains(system, "structured record"):
		e := render.Extracted{Acts: []render.ExtractedAct{}, FactsStated: []string{}, NewEvents: []render.NewEvent{}}
		for _, id := range []string{"f1", "f2", "f3"} {
			if strings.Contains(user, "F"+id+" 가온이") {
				e.Acts = append(e.Acts, render.ExtractedAct{Actor: "a", Verb: "sees", Targets: []string{}, Fact: id})
				e.FactsStated = append(e.FactsStated, id)
			}
		}
		if strings.Contains(user, "누군가 쓰러졌다") {
			e.NewEvents = append(e.NewEvents, render.NewEvent{Text: "someone collapses"})
		}
		b, _ := json.Marshal(e)
		return ok("```json\n" + string(b) + "\n```")
	case strings.Contains(system, "문체를 바꾼다"):
		return ok(strings.TrimPrefix(user, "## 바꿀 글\n"))
	case strings.Contains(system, "이음새만 다듬는다"):
		return ok(strings.TrimPrefix(user, "## 비트\n"))
	}
	var ps []string
	for _, id := range []string{"f1", "f2", "f3"} {
		if strings.Contains(user, abText[id]) {
			ps = append(ps, "F"+id+" 가온이 보았다.")
		}
	}
	if f.model == haiku && strings.Contains(system, "한 비트를 쓴다") && strings.Contains(user, abText["f1"]) && !strings.Contains(user, "Fix") && !strings.Contains(user, "고칠") {
		ps = append(ps, "누군가 쓰러졌다.")
	}
	return ok(strings.Join(ps, "\n\n"))
}

func abScene(id string) render.Scene {
	return render.Scene{
		ID:      id,
		Cast:    []render.Entity{{ID: "a", Names: map[string][]string{"ko": {"가온"}}}},
		OnStage: []string{"a"},
		Facts: []render.Fact{{ID: "f1", Text: abText["f1"], True: true}, {ID: "f2", Text: abText["f2"], True: true},
			{ID: "f3", Text: abText["f3"], True: true}},
		Beats: []render.Beat{
			{ID: "b1", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f1"}}},
			{ID: "b2", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f2"}}},
			{ID: "b3", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f3"}}},
		},
	}
}

func TestABDryRun(t *testing.T) {
	loadScenes = func(context.Context, bool) ([]Scene, error) {
		return []Scene{{"w1", abScene("s1")}, {"w2", abScene("s2")}}, nil
	}
	out := t.TempDir()
	fakes := map[string]*abFake{}
	c := &cfg{out: out, budget: 200, weights: map[string][2]float64{sonnet: {3, 15}, haiku: {1, 5}},
		mk: func(m, e string) render.Completer {
			if fakes[m+e] == nil {
				fakes[m+e] = &abFake{model: m}
			}
			return fakes[m+e]
		}}
	sum, err := run(context.Background(), c, "../../styles", false)
	if err != nil {
		t.Fatal(err)
	}
	real := 0
	for _, f := range fakes {
		real += f.n
	}
	if sum.Used != real || real == 0 {
		t.Fatalf("used %d, fake calls %d", sum.Used, real)
	}
	for _, a := range sum.Arms {
		if a.N != 2 {
			t.Errorf("%s: n=%d, want 2", a.Arm, a.N)
		}
		if a.Renderer == "layered" && a.AvgCalls < 9 {
			t.Errorf("%s: avg calls %.1f, want ≥ 9 (3 beats × 2 + style + stitch + final)", a.Arm, a.AvgCalls)
		}
		if a.Weighted <= 0 || a.Tokens[sonnet].In == 0 {
			t.Errorf("%s: no tokens %+v", a.Arm, a.Tokens)
		}
		if a.Errs != 0 {
			t.Errorf("%s: %d errors", a.Arm, a.Errs)
		}
	}
	if len(sum.Extra) != 2 || sum.Extra[0].Arm != "L-hai-swap" || sum.Extra[0].Calls == 0 || sum.Extra[0].Calls > 5 {
		t.Errorf("extra runs: %+v", sum.Extra)
	}
	tab, _ := os.ReadFile(filepath.Join(out, "table.md"))
	for _, want := range []string{"| S-son |", "| L-hai |", "L-hai-swap", "판정:"} {
		if !strings.Contains(string(tab), want) {
			t.Errorf("table missing %q:\n%s", want, tab)
		}
	}
	if sum.Decision.Renderer == "" || sum.Decision.RenderModel == "" {
		t.Errorf("decision %+v", sum.Decision)
	}
	if _, err := os.Stat(filepath.Join(out, "L-son", "w1_s1.ko-haera.md")); err != nil {
		t.Error(err)
	}
	t.Log("\n" + string(tab))
}

func TestABBudget(t *testing.T) {
	loadScenes = func(context.Context, bool) ([]Scene, error) {
		return []Scene{{"w1", abScene("s1")}, {"w2", abScene("s2")}}, nil
	}
	f := &abFake{model: sonnet}
	c := &cfg{out: t.TempDir(), budget: 20, mk: func(string, string) render.Completer { return f }}
	sum, err := run(context.Background(), c, "../../styles", false)
	if err != nil {
		t.Fatal(err)
	}
	if f.n > 20 || sum.Used != f.n {
		t.Fatalf("calls %d used %d, budget 20", f.n, sum.Used)
	}
}

func TestDecide(t *testing.T) {
	a := func(id, r, m string, p, w float64) ArmStat {
		return ArmStat{Arm: id, Renderer: r, Model: m, N: 4, PassRate: p, Weighted: w}
	}
	for _, c := range []struct {
		arms        []ArmStat
		rend, model string
	}{
		{[]ArmStat{a("S-son", "single", sonnet, 0.5, 10), a("L-son", "layered", sonnet, 0.75, 30), a("L-hai", "layered", haiku, 0.75, 20)}, "layered", haiku},
		{[]ArmStat{a("S-son", "single", sonnet, 0.75, 10), a("L-son", "layered", sonnet, 0.75, 30), a("L-hai", "layered", haiku, 0.5, 20), a("S-hai", "single", haiku, 0.25, 5)}, "single", sonnet},
		{[]ArmStat{a("S-son", "single", sonnet, 1, 10), a("L-son", "layered", sonnet, 0.5, 30), a("L-hai", "layered", haiku, 0.5, 20)}, "single", haiku},
	} {
		d := Decide(c.arms)
		if d.Renderer != c.rend || d.RenderModel != c.model {
			t.Errorf("got %+v, want %s/%s", d, c.rend, c.model)
		}
	}
}
