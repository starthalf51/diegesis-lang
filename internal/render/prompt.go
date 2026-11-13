package render

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

// BuildPrompt returns a cacheable system prompt (fixed per variant/lang/style and the reader-stack flags: terms present, narration)
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
	if style != nil && style.Flesh == "novel" { // flesh contract: the bone stays fixed, texture is allowed, a reaction is required
		sb.WriteString("- Narrate every act, in the given order. Do not omit any act.\n")
		sb.WriteString("- After each key act (speech, discovery, decision, reveal), give the point-of-view character's reaction — body, feeling, or thought.\n")
		sb.WriteString("- You may add: sensory detail, gestures, small movements within the same place, the point-of-view character's inner thoughts, atmosphere.\n")
		sb.WriteString("- Never add: new events; any change of possession, location (leaving the place), life or death, or relationship; new named characters or places; anything that contradicts the acts or facts; another character's unspoken thoughts in first person.\n")
	} else {
		sb.WriteString("- Narrate every act, in the given order. Do not omit any act and do not add events, actions, or facts that are not in the specification.\n")
	}
	sb.WriteString("- Do not add any speech, even reported speech, that is not an act in the specification, and do not draw on any known version of this story.\n")
	sb.WriteString("- Do not restate a fact through a character's thoughts or reactions unless an act gives it; convey each fact through the act that carries it.\n")
	sb.WriteString("- An act with a fact (after the colon) must make that fact clear to the reader through that act and its actor. For a `sees` act, say that its actor sees or hears the thing, not only that it happens.\n")
	if s.FreeDialogue {
		sb.WriteString("- A speech act may be voiced as dialogue you write. It must say only that act's fact (or, without a fact, only small talk that states no fact). Use a provided quote verbatim when there is one.\n")
	} else {
		sb.WriteString("- Dialogue may only use the provided `quote` strings. Speech acts without a quote are reported speech; never invent quoted dialogue.\n")
	}
	sb.WriteString("- Never state or imply any fact listed under \"Must not state or imply\", not even as a hint, a foreshadowing, or a character's guess.\n")
	sb.WriteString("- A fact marked false is only what a character says, believes or pretends. Never state it in the narrator's own voice as what happened.\n")
	sb.WriteString("- Stay in the stated point of view. Use the people's names in the scene's language.\n")
	hasTerms := len(s.TermsNew)+len(s.TermsKnown)+len(s.TermsRecap) > 0
	if hasTerms {
		sb.WriteString("- Do not coin new jargon: any work-specific word must be one of the listed terms.\n")
	}
	if hasTerms || s.Narration != "" {
		sb.WriteString("- Never nest narration inside a quote; a quote is only what the character says aloud.\n")
	}
	if s.Narration == "first-person" {
		sb.WriteString("- In first-person narration, never report another character's unspoken thoughts.\n")
	}
	fmt.Fprintf(&sb, "- Output only the prose, written in %s. No title, headings, notes, or commentary.\n", ln)
	if style != nil && style.Text != "" {
		sb.WriteString("\nWrite in the prose style of the following sample. Imitate its voice, rhythm, and diction, not its content.\n\n<style_sample>\n")
		sb.WriteString(style.Text)
		sb.WriteString("\n</style_sample>\n")
	}
	if style != nil && len(style.Rules) > 0 {
		sb.WriteString("\nStyle rules:\n")
		for _, r := range style.Rules {
			fmt.Fprintf(&sb, "- %s\n", r)
		}
	}
	if style != nil && len(style.Lexicon) > 0 {
		sb.WriteString("\nWord choices:\n")
		for _, l := range style.Lexicon {
			fmt.Fprintf(&sb, "- use %q for %s", l.Use, l.For)
			if len(l.Avoid) > 0 {
				q := make([]string, len(l.Avoid))
				for i, a := range l.Avoid {
					q[i] = fmt.Sprintf("%q", a)
				}
				fmt.Fprintf(&sb, "; never %s", strings.Join(q, ", "))
			}
			sb.WriteString("\n")
		}
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
			if a.Action != "" {
				parts = append(parts, "("+a.Action+")")
			}
			if len(a.Targets) > 0 {
				parts = append(parts, names(a.Targets))
			}
			line := strings.Join(parts, " ")
			if a.Fact != "" {
				f, _ := s.FactByID(a.Fact)
				line += ": " + f.Text
				if !f.True {
					line += " (false — the speaker believes or claims it; the narrator must not assert it)"
				}
			}
			if a.Quote != "" {
				line += fmt.Sprintf(" quote: %q", a.Quote)
			}
			fmt.Fprintf(&sb, "- %s\n", line)
		}
	}
	if s.Directives.POV != "" {
		fmt.Fprintf(&sb, "\n## Point of view\nNarrate from %s's point of view.\n", name(s.Directives.POV))
		if s.Narration == "first-person" {
			fmt.Fprintf(&sb, "Narration: first person, as %s\n", name(s.Directives.POV))
		}
	}
	if len(s.TermsNew) > 0 {
		sb.WriteString("\n## New terms — introduce each to a reader who has never heard it, inside the story, using its meaning\n")
		sb.WriteString("Weave each into the scene through action, dialogue, consequence or the senses (the given `via`, if any). Never write a definition sentence; partial understanding is fine.\n")
		for _, t := range s.TermsNew {
			if t.Via != "" {
				fmt.Fprintf(&sb, "- %s: %s (via: %s)\n", t.Surface, t.Gloss, t.Via)
				continue
			}
			fmt.Fprintf(&sb, "- %s: %s\n", t.Surface, t.Gloss)
		}
	}
	if len(s.TermsKnown) > 0 {
		sb.WriteString("\n## Terms the reader already knows\n")
		for _, t := range s.TermsKnown {
			fmt.Fprintf(&sb, "- %s\n", t.Surface)
		}
	}
	if len(s.TermsRecap) > 0 {
		sb.WriteString("\n## Remind briefly (the reader already met these; one short phrase, never a definition)\n")
		for _, t := range s.TermsRecap {
			fmt.Fprintf(&sb, "- %s\n", t.Surface)
		}
	}
	factList("The point-of-view character believes", s.POVBeliefs)
	factList("The reader already knows", s.ReaderKnows)
	factList("Must not state or imply", s.Withheld)
	// For a scene with no directives at all (render_scene's default), do not write an empty section.
	if d := s.Directives; v == VariantStyleDirectives && (d.Pace != "" || d.Tension != 0 || len(d.Focus)+len(d.Omit) > 0) {
		fmt.Fprintf(&sb, "\n## Directives\n- pace: %s\n- tension: %g (0 to 1)\n", d.Pace, d.Tension)
		if len(d.Focus) > 0 {
			fmt.Fprintf(&sb, "- focus: %s\n", strings.Join(d.Focus, "; "))
		}
		if len(d.Omit) > 0 {
			fmt.Fprintf(&sb, "- omit: %s\n", strings.Join(d.Omit, "; "))
		}
	}
	if s.Goal.Text != "" || s.Turn.To != "" || s.Tempo != "" {
		sb.WriteString("\n## Scene design\n")
		if s.Goal.Text != "" {
			fmt.Fprintf(&sb, "- goal: %s wants %s\n", name(s.Goal.Who), s.Goal.Text)
		}
		if s.Turn.To != "" {
			fmt.Fprintf(&sb, "- turn: from %s to %s — build the scene toward this change\n", s.Turn.From, s.Turn.To)
		}
		switch s.Tempo {
		case "scene":
			sb.WriteString("- tempo: scene\n")
		case "summary":
			sb.WriteString("- tempo: summary — compress: report the acts briefly\n")
		case "ellipsis":
			sb.WriteString("- tempo: ellipsis — skip time: open after the acts, mention them in passing\n")
		}
	}
	if s.Opening {
		op := "The first sentence must hook a first-time reader: one concrete, surprising situation in the narrator's voice, no setting summary."
		if style != nil && style.Opening != "" {
			op = style.Opening
		}
		fmt.Fprintf(&sb, "\n## Opening sentence\n%s\n", op)
	}
	return system, sb.String(), nil
}
