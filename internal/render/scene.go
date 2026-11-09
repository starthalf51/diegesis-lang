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
	ID    string `json:"id"` // ID the author wrote
	Text  string `json:"text"`
	True  bool   `json:"true"`
	Canon string `json:"canon,omitempty"` // canonical proposition (aliases and negation resolved). Used only for comparison. If empty, ID
}

type Act struct {
	Actor   string   `json:"actor"`
	Verb    string   `json:"verb"`
	Targets []string `json:"targets"`
	Fact    string   `json:"fact"`
	Quote   string   `json:"quote"`
	Action  string   `json:"action,omitempty"` // the action of does, the name of the document handled
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
	// remaining verbs of the compiler IR (render_scene passes them through as is)
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

// ParseScene strictly reads and validates Scene JSON (a file or PG render_scene): unknown keys and trailing data are errors.
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

// Canon is the canonical proposition of a fact ID (the comparison key). If it is not in the catalog or there is no canon, the ID as is.
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
