package main

import (
	"fmt"
	"strings"
)

type Variant string

const (
	VariantIROnly          Variant = "a"
	VariantStyle           Variant = "b"
	VariantStyleDirectives Variant = "c"
)

var langName = map[string]string{"ko": "Korean", "en": "English"}

// BuildPrompt returns a cacheable system prompt (fixed per variant/lang/style)
// and a per-scene user prompt. Variant a ignores style; b and c require a style in lang.
func BuildPrompt(s Scene, v Variant, lang string, style *StyleSample) (system, user string, err error) {
	ln, ok := langName[lang]
	if !ok {
		return "", "", fmt.Errorf("unsupported lang %q", lang)
	}
	switch v {
	case VariantIROnly:
		style = nil
	case VariantStyle, VariantStyleDirectives:
		if style == nil {
			return "", "", fmt.Errorf("variant %s requires a style sample", v)
		}
		if style.Lang != lang {
			return "", "", fmt.Errorf("style %q is %q, want %q", style.ID, style.Lang, lang)
		}
	default:
		return "", "", fmt.Errorf("unknown variant %q", v)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "You are a fiction writer. You will receive a scene specification (places, characters, an ordered list of story acts, and facts). Write the scene as continuous prose in %s.\n\n", ln)
	sb.WriteString("Rules:\n")
	sb.WriteString("- Narrate every act, in the given order. Do not omit any act and do not add events, actions, or facts that are not in the specification.\n")
	sb.WriteString("- Dialogue may only use the provided `quote` strings. A quote that reads like a stage direction (e.g. \"read aloud to Watson\") is a cue for what the act conveys, not dialogue to print; render it as narration.\n")
	sb.WriteString("- Never state or imply any fact listed under \"Must not state or imply\", not even as a hint, a foreshadowing, or a character's guess.\n")
	sb.WriteString("- Stay in the stated point of view. Use the people's names in the scene's language.\n")
	fmt.Fprintf(&sb, "- Output only the prose, written in %s. No title, headings, notes, or commentary.\n", ln)
	if style != nil {
		sb.WriteString("\nWrite in the prose style of the following sample. Imitate its voice, rhythm, and diction, not its content.\n\n<style_sample>\n")
		sb.WriteString(style.Text)
		sb.WriteString("\n</style_sample>\n")
	}
	system = sb.String()

	name := func(id string) string {
		if e, ok := s.Entity(id); ok && len(e.Names[lang]) > 0 {
			return e.Names[lang][0]
		}
		return id
	}
	names := func(ids []string) string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = name(id)
		}
		return strings.Join(out, ", ")
	}
	factList := func(h string, ids []string) {
		if len(ids) == 0 {
			return
		}
		fmt.Fprintf(&sb, "\n## %s\n", h)
		for _, id := range ids {
			f, _ := s.FactByID(id)
			fmt.Fprintf(&sb, "- %s\n", f.Text)
		}
	}

	sb.Reset()
	fmt.Fprintf(&sb, "# Scene: %s\n", s.Title)
	if s.Previously != "" {
		fmt.Fprintf(&sb, "\n## Previously\n%s\n", s.Previously)
	}
	fmt.Fprintf(&sb, "\n## Place\n%s\n\n## Characters\n", s.Place)
	for _, e := range s.Cast {
		fmt.Fprintf(&sb, "- %s\n", name(e.ID))
	}
	sb.WriteString("\n## Acts (in order)\n")
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			parts := []string{}
			if a.Actor != "" {
				parts = append(parts, name(a.Actor))
			}
			parts = append(parts, a.Verb)
			if len(a.Targets) > 0 {
				parts = append(parts, names(a.Targets))
			}
			line := strings.Join(parts, " ")
			if a.Fact != "" {
				f, _ := s.FactByID(a.Fact)
				line += ": " + f.Text
			}
			if a.Quote != "" {
				line += fmt.Sprintf(" quote: %q", a.Quote)
			}
			fmt.Fprintf(&sb, "- %s\n", line)
		}
	}
	if s.Directives.POV != "" {
		fmt.Fprintf(&sb, "\n## Point of view\nNarrate from %s's point of view.\n", name(s.Directives.POV))
	}
	factList("The point-of-view character believes", s.POVBeliefs)
	factList("The reader already knows", s.ReaderKnows)
	factList("Must not state or imply", s.Withheld)
	if v == VariantStyleDirectives {
		d := s.Directives
		fmt.Fprintf(&sb, "\n## Directives\n- pace: %s\n- tension: %g (0 to 1)\n", d.Pace, d.Tension)
		if len(d.Focus) > 0 {
			fmt.Fprintf(&sb, "- focus: %s\n", strings.Join(d.Focus, "; "))
		}
		if len(d.Omit) > 0 {
			fmt.Fprintf(&sb, "- omit: %s\n", strings.Join(d.Omit, "; "))
		}
	}
	return system, sb.String(), nil
}
