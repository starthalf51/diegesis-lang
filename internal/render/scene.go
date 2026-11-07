package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Entity struct {
	ID    string              `json:"id"`
	Names map[string][]string `json:"names"`
}

type Fact struct {
	ID    string `json:"id"` // 지은이가 쓴 ID
	Text  string `json:"text"`
	True  bool   `json:"true"`
	Canon string `json:"canon,omitempty"` // 정규 명제(별칭·부정을 푼 꼴). 비교에만 쓴다. 비면 ID
}

type Act struct {
	Actor   string   `json:"actor"`
	Verb    string   `json:"verb"`
	Targets []string `json:"targets"`
	Fact    string   `json:"fact"`
	Quote   string   `json:"quote"`
	Action  string   `json:"action,omitempty"` // does 의 동작, 다루는 문서 이름
}

type Beat struct {
	ID   string `json:"id"`
	Acts []Act  `json:"acts"`
}

type Directives struct {
	POV     string   `json:"pov"`
	Pace    string   `json:"pace"`
	Tension float64  `json:"tension"`
	Focus   []string `json:"focus"`
	Omit    []string `json:"omit"`
}

type Scene struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Place        string     `json:"place"`
	Previously   string     `json:"previously"`
	Cast         []Entity   `json:"cast"`
	OnStage      []string   `json:"on_stage"` // entity IDs on stage at scene start
	Facts        []Fact     `json:"facts"`
	Beats        []Beat     `json:"beats"`
	Directives   Directives `json:"directives"`
	POVBeliefs   []string   `json:"pov_beliefs"`
	ReaderKnows  []string   `json:"reader_knows"`
	Withheld     []string   `json:"withheld"`
	Narration    string     `json:"narration"`               // "first-person" | "third-person" (default)
	FreeDialogue bool       `json:"free_dialogue,omitempty"` // renderer may write dialogue for speech acts (skeleton mode)
}

var verbs = map[string]bool{
	"enter": true, "exit": true, "tells": true, "says": true, "hails": true, "lies": true,
	"whispers": true, "sees": true, "reads": true, "does": true, "aside": true,
	// 컴파일러 IR 의 나머지 동사(render_scene 이 그대로 넘긴다)
	"asks": true, "orders": true, "shows": true, "decrees": true, "sends": true, "writes": true, "thinks": true, "enacts": true,
}

func LoadScene(path string) (Scene, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Scene{}, err
	}
	s, err := ParseScene(b)
	if err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// ParseScene 은 Scene JSON(파일 또는 PG render_scene)을 엄격하게 읽고 검사한다: 모르는 키, 뒤에 붙은 데이터는 오류다.
func ParseScene(b []byte) (Scene, error) {
	var s Scene
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, err
	}
	if dec.More() {
		return s, errors.New("trailing data after scene JSON")
	}
	return s, s.Validate()
}

func (s Scene) Entity(id string) (Entity, bool) {
	for _, e := range s.Cast {
		if e.ID == id {
			return e, true
		}
	}
	return Entity{}, false
}

func (s Scene) FactByID(id string) (Fact, bool) {
	for _, f := range s.Facts {
		if f.ID == id {
			return f, true
		}
	}
	return Fact{}, false
}

// Canon 은 사실 ID 의 정규 명제(비교 키). 카탈로그에 없거나 canon 이 없으면 ID 그대로.
func (s Scene) Canon(id string) string {
	if f, ok := s.FactByID(id); ok && f.Canon != "" {
		return f.Canon
	}
	return id
}

// Validate collects every violation, with field paths, via errors.Join.
func (s Scene) Validate() error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	inCast := func(path, id string) bool {
		_, ok := s.Entity(id)
		if !ok {
			bad("%s %q: unknown entity", path, id)
		}
		return ok
	}
	factList := func(name string, ids []string) {
		for i, id := range ids {
			if _, ok := s.FactByID(id); !ok {
				bad("%s[%d] %q: unknown fact", name, i, id)
			}
		}
	}

	if s.Narration != "" && s.Narration != "first-person" && s.Narration != "third-person" {
		bad("narration %q: want first-person or third-person", s.Narration)
	}
	if s.Directives.POV != "" {
		inCast("directives.pov", s.Directives.POV)
	}
	stage := map[string]bool{}
	for i, id := range s.OnStage {
		if inCast(fmt.Sprintf("on_stage[%d]", i), id) {
			stage[id] = true
		}
	}
	factList("pov_beliefs", s.POVBeliefs)
	factList("reader_knows", s.ReaderKnows)
	factList("withheld", s.Withheld)

	for bi, b := range s.Beats {
		for ai, a := range b.Acts {
			p := fmt.Sprintf("beats[%d].acts[%d]", bi, ai)
			if !verbs[a.Verb] {
				bad("%s.verb %q: unknown verb", p, a.Verb)
				continue
			}
			for ti, t := range a.Targets {
				inCast(fmt.Sprintf("%s.targets[%d]", p, ti), t)
			}
			if a.Fact != "" {
				if _, ok := s.FactByID(a.Fact); !ok {
					bad("%s.fact %q: unknown fact", p, a.Fact)
				}
			}
			switch a.Verb {
			case "enter", "exit":
				if a.Actor != "" {
					bad("%s.actor %q: must be empty for %s", p, a.Actor, a.Verb)
				}
				for _, t := range a.Targets {
					stage[t] = a.Verb == "enter"
				}
			default:
				if a.Actor == "" || !stage[a.Actor] {
					bad("%s.actor %q: not on stage", p, a.Actor)
				}
				if a.Verb == "aside" && len(a.Targets) > 0 {
					bad("%s.targets: must be empty for aside", p)
				}
			}
		}
	}
	return errors.Join(errs...)
}
