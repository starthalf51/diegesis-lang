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

// Term is a work-specific word: Surface is its first written form, Gloss its meaning.
type Term struct {
	Surface string `json:"surface"`
	Gloss   string `json:"gloss"`
	Via     string `json:"via,omitempty"` // how to weave it in: action | dialogue | consequence | sense (terms_new only)
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
	TermsNew     []Term     `json:"terms_new"`               // terms this scene introduces to the reader
	TermsKnown   []Term     `json:"terms_known"`             // terms explained in earlier scenes
	TermsRecap   []Term     `json:"terms_recap"`             // terms this scene only reminds the reader of
	Goal         Goal       `json:"goal"`                    // scene design: what a character wants (empty without a design block)
	Turn         Turn       `json:"turn"`                    // scene design: the value change across the scene
	Tempo        string     `json:"tempo"`                   // scene design: "" | scene | summary | ellipsis
	Opening      bool       `json:"opening,omitempty"`       // dieg render --opening: ask for a hooking first sentence
	Handoff      *Handoff   `json:"handoff,omitempty"`       // where the previous scene ended (design scenes with a preceding shown scene only)
	Flesh        string     `json:"-"`                       // the backend style's flesh mode; selects the re-extraction and card prompts
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

// Goal is a scene-design goal: Who (entity ID) wants Text.
type Goal struct {
	Who  string `json:"who"`
	Text string `json:"text"`
}

// Turn is the value change a scene builds toward (McKee), medium-neutral.
type Turn struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Handoff is the previous scene's end state, passed into the next scene's render prompt so the boundary does not break.
// Characters carry their names so they resolve in the scene language even when absent from this scene's cast.
type Handoff struct {
	Place   string      `json:"place"`
	Last    *HandoffAct `json:"last,omitempty"` // the previous scene's last act; absent when it had none
	Present []Entity    `json:"present"`        // still on stage when the scene ended
}

// HandoffAct is an act: Text is its fact sentence, or its action text when it has no fact (may be empty).
type HandoffAct struct {
	Actor Entity `json:"actor"`
	Verb  string `json:"verb"`
	Text  string `json:"text,omitempty"`
}
