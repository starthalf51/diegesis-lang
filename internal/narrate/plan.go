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
	Chars      int // 목표 분량: TargetChars / 비트 수
	Forbidden  []string
	Allowed    []string
}

type Plan struct {
	Scene       render.Scene
	Beats       []Beat
	TargetChars int
	Notes       []string
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
	// 비트에 하나라도 이름이 있으면 지은이 비트를 쓰되 MaxActsPerBeat 를 넘으면 name.1, name.2 … 로 쪼갠다.
	// 모두 무명이면 행위를 모아 MaxActsPerBeat 씩 자른다.
	named := false
	for _, b := range s.Beats {
		named = named || b.ID != ""
	}
	var groups []render.Beat
	if named {
		for i, b := range s.Beats {
			if b.ID == "" {
				b.ID = fmt.Sprintf("b%d", i)
			}
			if len(b.Acts) <= o.MaxActsPerBeat {
				groups = append(groups, b)
				continue
			}
			for j := 0; j < len(b.Acts); j += o.MaxActsPerBeat {
				groups = append(groups, render.Beat{ID: fmt.Sprintf("%s.%d", b.ID, j/o.MaxActsPerBeat+1), Acts: b.Acts[j:min(j+o.MaxActsPerBeat, len(b.Acts))]})
			}
		}
	} else {
		var all []render.Act
		for _, b := range s.Beats {
			all = append(all, b.Acts...)
		}
		for i := 0; i < len(all); i += o.MaxActsPerBeat {
			groups = append(groups, render.Beat{ID: fmt.Sprintf("beat-%d", len(groups)+1), Acts: all[i:min(i+o.MaxActsPerBeat, len(all))]})
		}
	}

	para := max(1, int(math.Round(float64(o.TargetChars)/float64(max(1, len(groups)))/350)))
	p := Plan{Scene: s, TargetChars: o.TargetChars}
	for _, g := range groups {
		b := Beat{ID: g.ID, Paragraphs: para, Chars: o.TargetChars / len(groups), Allowed: append([]string(nil), allowed...)}
		used := map[string]bool{}
		for _, a := range g.Acts {
			used[a.Fact] = true
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
		for _, id := range p.forbidIDs(used) {
			if f, ok := s.FactByID(id); ok {
				b.Forbidden = append(b.Forbidden, f.Text)
			}
		}
		for _, id := range s.Withheld {
			if used[id] {
				p.Notes = append(p.Notes, fmt.Sprintf("숨김 사실 %q 를 비트 %s 의 행위가 다룬다: 행위는 두고 그 비트에서만 금지하지 않는다", id, g.ID))
			}
		}
		p.Beats = append(p.Beats, b)
	}
	return p, CheckPlan(p)
}

// forbidIDs: 숨김 사실 + (시점 인물이 있으면) 시점 인물이 모르고 장면 어느 행위도 다루지 않는 참 사실. used 에 있는 사실은 뺀다.
func (p Plan) forbidIDs(used map[string]bool) []string {
	s := p.Scene
	covered := map[string]bool{}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			covered[a.Fact] = true
		}
	}
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] && !used[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range s.Withheld {
		add(id)
	}
	if s.Directives.POV != "" {
		// ponytail: 시점 인물이 모르는 사실 = POVBeliefs 에 없는 참 사실 근사. 정밀화는 PG 독자 모델로.
		for _, f := range s.Facts {
			if f.True && !contains(s.POVBeliefs, f.ID) && !covered[f.ID] {
				add(f.ID)
			}
		}
	}
	return ids
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
	forbid := p.forbidIDs(use)
	keepF := map[string]bool{}
	for id := range use {
		keepF[id] = true
	}
	for _, id := range forbid {
		keepF[id] = true // 금지 사실도 항목은 있어야 Diff 가 누출(Leaked)로 보고한다
	}
	for _, f := range p.Scene.Facts {
		if keepF[f.ID] {
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
	s.POVBeliefs, s.ReaderKnows, s.Withheld = keep(s.POVBeliefs), keep(s.ReaderKnows), forbid
	return s
}
