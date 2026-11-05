package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type ExtractedAct struct {
	Actor   string   `json:"actor"`
	Verb    string   `json:"verb"`
	Targets []string `json:"targets"`
	Fact    string   `json:"fact"`
}

type Extracted struct {
	Acts        []ExtractedAct `json:"acts"`
	FactsStated []string       `json:"facts_stated"`
	NewEvents   []string       `json:"new_events"`
}

// BuildExtractPrompt shows the extractor only the prose and an unlabeled
// catalog (all entities with all names, all facts). No answer key.
func BuildExtractPrompt(s Scene, lang, prose string) (system, user string) {
	var vs []string
	for v := range verbs {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	system = `You convert a piece of narrative prose back into a structured record of what happens in it.
Answer only with IDs from the catalog you are given. Do not invent IDs.
- acts: every action in the prose, in order. actor is an entity ID (empty for enter/exit), verb is one of: ` + strings.Join(vs, ", ") + `, targets are entity IDs, fact is the ID of the catalog fact the act conveys (empty if none).
- Perception is an act: when the prose narrates that a character sees, hears, notices or recognises something (a sound reaching someone's ear, a searching look that settles a question), record it as verb "sees" with that character as actor and the perceived catalog fact as fact. Hearing what another character says is part of that speaker's act, not a separate "sees" act.
- A line can convey a fact without stating it: how a speaker addresses someone, what a speaker admits about themselves, or what a deceiver wants the listener to believe. Use that fact for the act. If one act conveys several catalog facts, list that act once per fact (same actor, verb and targets).
- facts_stated: the ID of every catalog fact the prose states or strongly implies. Report in facts_stated every listed fact the prose states or clearly implies, whether true or not in the story; do not report facts the prose does not convey.
- new_events: plot events only: an action or revelation that changes the situation, not already reported in acts, and not covered by any listed fact. Ignore description, gesture, setting, and manner. Each as one short sentence. Use [] if none.
- Judge only from what this prose says or implies to a reader who does not know the source story; ignore your knowledge of any original work.
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"acts":[{"actor":"","verb":"","targets":[],"fact":""}],"facts_stated":[],"new_events":[]}.`

	var b strings.Builder
	b.WriteString("Prose language: " + lang + "\n\n## Entities (ID: names in every language)\n")
	for _, e := range s.Cast {
		var langs []string
		for l := range e.Names {
			langs = append(langs, l)
		}
		sort.Strings(langs)
		var parts []string
		for _, l := range langs {
			parts = append(parts, strings.Join(e.Names[l], ", "))
		}
		fmt.Fprintf(&b, "- %s: %s\n", e.ID, strings.Join(parts, " / "))
	}
	b.WriteString("\n## Facts (ID: text)\n")
	for _, f := range s.Facts {
		fmt.Fprintf(&b, "- %s: %s\n", f.ID, f.Text)
	}
	b.WriteString("\n## Prose\n" + prose + "\n")
	return system, b.String()
}

// ParseExtracted is strict: first closed ```json fence, else the whole text;
// unknown fields and trailing data are errors. Never returns zero value with nil error.
func ParseExtracted(text string) (Extracted, error) {
	body := strings.TrimSpace(text)
	if i := strings.Index(text, "```json"); i >= 0 {
		rest := text[i+len("```json"):]
		j := strings.Index(rest, "```")
		if j < 0 {
			return Extracted{}, errors.New("extract: unclosed ```json fence")
		}
		body = strings.TrimSpace(rest[:j])
	}
	if body == "" {
		return Extracted{}, errors.New("extract: empty output")
	}
	var e Extracted
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return Extracted{}, fmt.Errorf("extract: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Extracted{}, errors.New("extract: trailing data after JSON object")
	}
	var keys map[string]json.RawMessage // nil for `null`
	_ = json.Unmarshal([]byte(body), &keys)
	for _, k := range []string{"acts", "facts_stated"} {
		if v, ok := keys[k]; !ok || string(v) == "null" {
			return Extracted{}, fmt.Errorf("extract: missing required key %q", k)
		}
	}
	return e, nil
}
