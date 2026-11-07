package narrate

import (
	"fmt"
	"strings"
	"testing"

	"diegesis/internal/render"
)

func testScene(nActs int, groups []int) render.Scene {
	s := render.Scene{
		Cast:    []render.Entity{{ID: "a"}, {ID: "b"}},
		OnStage: []string{"a", "b"},
		Facts: []render.Fact{
			{ID: "f1", Text: "첫째 사실", True: true},
			{ID: "f2", Text: "둘째 거짓", True: false},
			{ID: "x", Text: "숨긴 사실", True: true},
		},
	}
	n := 0
	mk := func() render.Act {
		n++
		return render.Act{Actor: "a", Verb: "does", Action: fmt.Sprintf("act%d", n)}
	}
	if groups == nil {
		b := render.Beat{}
		for i := 0; i < nActs; i++ {
			b.Acts = append(b.Acts, mk())
		}
		s.Beats = []render.Beat{b}
		return s
	}
	for gi, g := range groups {
		b := render.Beat{ID: fmt.Sprintf("b%d", gi)}
		for i := 0; i < g; i++ {
			b.Acts = append(b.Acts, mk())
		}
		s.Beats = append(s.Beats, b)
	}
	return s
}

func sizes(p Plan) []int {
	var r []int
	for _, b := range p.Beats {
		r = append(r, len(b.Acts))
	}
	return r
}

func TestPlanCoversEveryActOnce(t *testing.T) {
	p, err := PlanScene(testScene(7, []int{4, 3}), PlanOpts{})
	if err != nil || fmt.Sprint(sizes(p)) != "[4 3]" {
		t.Fatalf("named beats: %v %v", sizes(p), err)
	}
	p, err = PlanScene(testScene(7, nil), PlanOpts{})
	if err != nil || fmt.Sprint(sizes(p)) != "[3 3 1]" {
		t.Fatalf("unbeated: %v %v", sizes(p), err)
	}
	if err := CheckPlan(p); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPlanRejectsDropAndDup(t *testing.T) {
	p, _ := PlanScene(testScene(7, []int{4, 3}), PlanOpts{})
	drop := p
	drop.Beats = []Beat{{ID: "b0", Acts: p.Beats[0].Acts[1:]}, p.Beats[1]}
	if CheckPlan(drop) == nil {
		t.Error("drop accepted")
	}
	dup := p
	dup.Beats = []Beat{{ID: "b0", Acts: append(append([]BeatAct{}, p.Beats[0].Acts...), p.Beats[0].Acts[0])}, p.Beats[1]}
	if CheckPlan(dup) == nil {
		t.Error("dup accepted")
	}
}

func TestFormRules(t *testing.T) {
	for _, c := range []struct {
		name string
		act  render.Act
		free bool
		want Form
	}{
		{"quote", render.Act{Verb: "says", Quote: "안녕"}, false, FormDialogue},
		{"indirect", render.Act{Verb: "says"}, false, FormNarration},
		{"free", render.Act{Verb: "says"}, true, FormDialogue},
		{"thinks", render.Act{Verb: "thinks"}, false, FormThought},
		{"sees", render.Act{Verb: "sees"}, false, FormPerception},
		{"does", render.Act{Verb: "does"}, false, FormAction},
		{"enter", render.Act{Verb: "enter"}, false, FormAction},
	} {
		if got := formOf(c.act, c.free); got != c.want {
			t.Errorf("%s: %s want %s", c.name, got, c.want)
		}
	}
}

func TestModalityRules(t *testing.T) {
	s := testScene(0, []int{0})
	s.Beats[0].Acts = []render.Act{
		{Actor: "a", Verb: "says", Fact: "f1"},
		{Actor: "a", Verb: "lies", Fact: "f2"},
		{Actor: "a", Verb: "says", Fact: "f2"},
	}
	p, err := PlanScene(s, PlanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var got []Modality
	for _, a := range p.Beats[0].Acts {
		got = append(got, a.Modality)
	}
	if fmt.Sprint(got) != fmt.Sprint([]Modality{ModFact, ModLie, ModBelief}) {
		t.Fatal(got)
	}
	if p.Beats[0].Acts[1].FactText != "둘째 거짓" {
		t.Error("FactText")
	}
}

func TestForbiddenHasWithheld(t *testing.T) {
	s := testScene(7, []int{4, 3})
	s.Withheld = []string{"x"}
	p, _ := PlanScene(s, PlanOpts{})
	for i, b := range p.Beats {
		if !strings.Contains(strings.Join(b.Forbidden, "|"), "숨긴 사실") {
			t.Errorf("beat %d: %v", i, b.Forbidden)
		}
		if len(b.Allowed) != 4 {
			t.Errorf("allowed %v", b.Allowed)
		}
	}
}

func TestBeatSceneOnlyItsActs(t *testing.T) {
	s := testScene(0, []int{1, 1})
	s.Beats[0].Acts = []render.Act{{Actor: "a", Verb: "says", Fact: "f1"}}
	s.Beats[1].Acts = []render.Act{{Actor: "a", Verb: "says", Fact: "f2"}}
	p, _ := PlanScene(s, PlanOpts{})
	bs := p.BeatScene(1)
	if len(bs.Beats) != 1 || len(bs.Beats[0].Acts) != 1 || bs.Beats[0].Acts[0].Fact != "f2" {
		t.Fatalf("%+v", bs.Beats)
	}
	if len(bs.Facts) != 1 || bs.Facts[0].ID != "f2" {
		t.Fatalf("%+v", bs.Facts)
	}
}
