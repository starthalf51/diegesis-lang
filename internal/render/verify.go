package render

import (
	"context"
	"strings"
)

// VerifyOpts: K is the number of re-extractions, Retries the maximum number of re-renders after the first render.
// Verifier "card" scores with a checklist (ScoreCard, Card option) instead of re-extraction; anything else (""·"open") is open re-extraction.
type VerifyOpts struct {
	K, Retries int
	Verifier   string
	Card       CardOpts
	// UserSuffix is appended to the render user prompt (after Prompt, before retry feedback) for generation only.
	// It is not part of RenderHash.
	UserSuffix string
	// StateRm, StateAd (PatchVerified only): world-state lines that left / entered the digest since the old prose was
	// written. They join the scene diff, so a patch for a scene whose own request did not change still has a diff.
	StateRm, StateAd []string
}

// Passed is whether the fact criteria pass. The open path is ADR-019 (hallucination 0, leak 0, missing ≤ MaxMissing); the card path is ADR-028 (leak 0, missing+wrong kind ≤ 1, G2 is a warning only — Hallucinated is always empty).
// The open path is stricter than render-check's ADR-012 (missing ≤ 1 allowed). If scoring broke (ExtractError) it is not a pass.
func Passed(r Render) bool {
	p := r.Report
	return p.ExtractError == "" && len(p.Hallucinated) == 0 && len(p.Leaked) == 0 && len(p.Missing) <= MaxMissing
}

// RenderVerified repeats render → re-extract → diff up to 1+Retries times until it passes. The failed diff is attached as feedback to the end of the next render's
// user prompt. At the limit it returns the last render with a nil error (see Passed for whether it passed).
// A Completer error is returned as is.
func RenderVerified(ctx context.Context, c Completer, work string, s Scene, b Backend, o VerifyOpts) (Render, int, error) {
	return RenderVerified2(ctx, Completers{c, c}, work, s, b, o)
}

// Completers are the per-role models: render calls go to Render, re-extraction calls to Extract.
type Completers struct{ Render, Extract Completer }

func RenderVerified2(ctx context.Context, cs Completers, work string, s Scene, b Backend, o VerifyOpts) (Render, int, error) {
	if o.K < 1 {
		o.K = 1
	}
	var r Render
	extra := ""
	for n := 1; ; n++ {
		var err error
		if r, err = renderWith(ctx, cs.Render, cs.Extract, work, s, b, o.K, o.UserSuffix, extra, o.card()); err != nil {
			return r, n, err
		}
		if Passed(r) || n > o.Retries {
			return r, n, nil
		}
		extra = feedback(s, b.Lang, r.Report)
	}
}

// feedback is the "## Fix from the previous draft" section written with human-readable fact sentences and character names.
func feedback(s Scene, lang string, rep Report) string {
	var rm, inc []string
	for _, h := range rep.Hallucinated {
		rm = append(rm, describe(s, lang, strings.TrimPrefix(strings.TrimPrefix(h, "extra-act:"), "unknown-fact:")))
	}
	for _, l := range rep.Leaked {
		rm = append(rm, describe(s, lang, l)+" (withheld: the narration must not reveal it)")
	}
	for _, m := range rep.Missing {
		inc = append(inc, describe(s, lang, m))
	}
	var b strings.Builder
	b.WriteString("\n\n## Fix from the previous draft\n")
	if len(rm) > 0 {
		b.WriteString("The previous draft stated things that are not in the scene. Items to remove:\n")
		for _, x := range rm {
			b.WriteString("- " + x + "\n")
		}
	}
	if len(inc) > 0 {
		b.WriteString("The previous draft left out these acts. Items to include:\n")
		for _, x := range inc {
			b.WriteString("- " + x + "\n")
		}
	}
	return b.String()
}

// describe turns an "actor|class|fact" key into "name (class): fact sentence", and a lone fact ID into a sentence. Unknown ones stay as is.
func describe(s Scene, lang, key string) string {
	p := strings.Split(key, "|")
	text := func(id string) string {
		if f, ok := s.FactByID(id); ok && f.Text != "" {
			return f.Text
		}
		return id
	}
	if len(p) != 3 {
		return text(key)
	}
	name := p[0]
	if e, ok := s.Entity(p[0]); ok {
		if ns := e.Names[lang]; len(ns) > 0 {
			name = ns[0]
		}
	}
	return name + " (" + p[1] + "): " + text(p[2])
}

func (o VerifyOpts) card() *CardOpts {
	if o.Verifier == "card" {
		return &o.Card
	}
	return nil
}
