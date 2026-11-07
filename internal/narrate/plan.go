// Package narrate 는 장면 IR 을 계층형으로 렌더한다. 이 파일은 [1] 서술 설계: 결정적·순수 코드.
package narrate

import (
	"errors"
	"fmt"
	"math"

	"diegesis/internal/render"
)

type Form string

const (
	FormDialogue   Form = "dialogue"
	FormThought    Form = "thought"
	FormPerception Form = "perception"
	FormAction     Form = "action"
	FormNarration  Form = "narration"
)

type Modality string

const (
	ModFact   Modality = "fact"
	ModBelief Modality = "belief"
	ModLie    Modality = "lie"
)

type BeatAct struct {
	Act      render.Act
	Form     Form
	Modality Modality
	FactText string
}

type Beat struct {
	ID         string
	Acts       []BeatAct
	Paragraphs int
	Forbidden  []string
	Allowed    []string
}

type Plan struct {
	Scene       render.Scene
	Beats       []Beat
	TargetChars int
}

type PlanOpts struct{ TargetChars, MaxActsPerBeat int }

var allowed = []string{"감각 묘사(소리·빛·냄새·온도)", "인물의 몸짓·표정", "장소의 모습", "시간의 흐름을 나타내는 짧은 표현"}

func formOf(a render.Act, free bool) Form {
	switch a.Verb {
	case "says", "tells", "hails", "asks", "whispers", "orders", "decrees", "lies":
		if a.Quote != "" || free {
			return FormDialogue
		}
		return FormNarration
	case "thinks", "aside":
		return FormThought
	case "sees":
		return FormPerception
	}
	return FormAction
}

func PlanScene(s render.Scene, o PlanOpts) (Plan, error) {
	if o.TargetChars <= 0 {
		o.TargetChars = 2500
	}
	if o.MaxActsPerBeat <= 0 {
		o.MaxActsPerBeat = 3
	}
	// 지은이가 비트에 이름을 붙였으면 그대로, 모두 무명이면 행위를 MaxActsPerBeat 씩 자른다.
	named := false
	for _, b := range s.Beats {
		named = named || b.ID != ""
	}
	var groups []render.Beat
	if named {
		groups = s.Beats
	} else {
		var all []render.Act
		for _, b := range s.Beats {
			all = append(all, b.Acts...)
		}
		for i := 0; i < len(all); i += o.MaxActsPerBeat {
			groups = append(groups, render.Beat{ID: fmt.Sprintf("beat-%d", len(groups)+1), Acts: all[i:min(i+o.MaxActsPerBeat, len(all))]})
		}
	}

	covered := map[string]bool{} // 장면의 어느 행위든 다루는 사실
	for _, g := range groups {
		for _, a := range g.Acts {
			covered[a.Fact] = true
		}
	}
	known := map[string]bool{}
	for _, id := range s.POVBeliefs {
		known[id] = true
	}
	var forbidden []string
	for _, id := range s.Withheld {
		if f, ok := s.FactByID(id); ok {
			forbidden = append(forbidden, f.Text)
		}
	}
	if s.Directives.POV != "" {
		// ponytail: 시점 인물이 모르는 사실 = POVBeliefs 에 없는 참 사실 근사. 정밀화는 PG 독자 모델로.
		for _, f := range s.Facts {
			if f.True && !known[f.ID] && !covered[f.ID] && !contains(s.Withheld, f.ID) {
				forbidden = append(forbidden, f.Text)
			}
		}
	}

	para := max(1, int(math.Round(float64(o.TargetChars)/float64(max(1, len(groups)))/350)))
	p := Plan{Scene: s, TargetChars: o.TargetChars}
	for _, g := range groups {
		b := Beat{ID: g.ID, Paragraphs: para, Forbidden: forbidden, Allowed: allowed}
		for _, a := range g.Acts {
			ba := BeatAct{Act: a, Form: formOf(a, s.FreeDialogue), Modality: ModFact}
			if f, ok := s.FactByID(a.Fact); ok {
				ba.FactText = f.Text
				if !f.True {
					ba.Modality = ModBelief
					if a.Verb == "lies" {
						ba.Modality = ModLie
					}
				}
			}
			b.Acts = append(b.Acts, ba)
		}
		p.Beats = append(p.Beats, b)
	}
	return p, CheckPlan(p)
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func actKey(a render.Act) string { return fmt.Sprintf("%#v", a) }

// CheckPlan: 장면의 모든 행위가 계획에 정확히 한 번씩 있어야 한다.
func CheckPlan(p Plan) error {
	want := map[string]int{}
	for _, b := range p.Scene.Beats {
		for _, a := range b.Acts {
			want[actKey(a)]++
		}
	}
	for _, b := range p.Beats {
		for _, a := range b.Acts {
			want[actKey(a.Act)]--
		}
	}
	var errs []error
	for k, n := range want {
		if n > 0 {
			errs = append(errs, fmt.Errorf("행위 누락 x%d: %s", n, k))
		} else if n < 0 {
			errs = append(errs, fmt.Errorf("행위 중복 x%d: %s", -n, k))
		}
	}
	return errors.Join(errs...)
}

// BeatScene 은 비트 i 의 행위와 그 사실만 담은 소장면(검증용).
func (p Plan) BeatScene(i int) render.Scene {
	s := p.Scene
	b := render.Beat{ID: p.Beats[i].ID}
	use := map[string]bool{}
	for _, a := range p.Beats[i].Acts {
		b.Acts = append(b.Acts, a.Act)
		use[a.Act.Fact] = true
	}
	s.Beats = []render.Beat{b}
	s.Facts = nil
	for _, f := range p.Scene.Facts {
		if use[f.ID] {
			s.Facts = append(s.Facts, f)
		}
	}
	keep := func(ids []string) (r []string) {
		for _, id := range ids {
			if use[id] {
				r = append(r, id)
			}
		}
		return
	}
	s.POVBeliefs, s.ReaderKnows, s.Withheld = keep(s.POVBeliefs), keep(s.ReaderKnows), keep(s.Withheld)
	return s
}
