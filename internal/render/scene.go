package render

import (
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
	ID   string `json:"id"`
	Text string `json:"text"`
	True bool   `json:"true"`
}

type Act struct {
	Actor   string   `json:"actor"`
	Verb    string   `json:"verb"`
	Targets []string `json:"targets"`
	Fact    string   `json:"fact"`
	Quote   string   `json:"quote"`
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
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Place       string     `json:"place"`
	Previously  string     `json:"previously"`
	Cast        []Entity   `json:"cast"`
	OnStage     []string   `json:"on_stage"` // entity IDs on stage at scene start
	Facts       []Fact     `json:"facts"`
	Beats       []Beat     `json:"beats"`
	Directives  Directives `json:"directives"`
	POVBeliefs  []string   `json:"pov_beliefs"`
	ReaderKnows []string   `json:"reader_knows"`
	Withheld    []string   `json:"withheld"`
	Narration   string     `json:"narration"` // "first-person" | "third-person" (default)
}

var verbs = map[string]bool{
	"enter": true, "exit": true, "tells": true, "says": true, "hails": true, "lies": true,
	"whispers": true, "sees": true, "reads": true, "does": true, "aside": true,
}

func LoadScene(path string) (Scene, error) {
	var s Scene
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
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
