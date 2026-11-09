package accept

import (
	"fmt"
	"slices"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// matches: pattern and target match exactly, and if SpanFrom is given the span start matches too.
func (w WantPattern) matches(work string) func(pg.Match) bool {
	return func(m pg.Match) bool {
		return m.Pattern == w.Pattern && short(work, m.TargetID) == w.Target &&
			(w.SpanFrom == nil || m.SpanFrom != nil && *m.SpanFrom == *w.SpanFrom)
	}
}

func spanText(from *int) string {
	if from == nil {
		return ""
	}
	return fmt.Sprintf(" (span_from %d)", *from)
}

func spanOf(m pg.Match) string {
	s := func(p *int) string {
		if p == nil {
			return "-"
		}
		return fmt.Sprint(*p)
	}
	return s(m.SpanFrom) + ".." + s(m.SpanTo)
}

// patternTables is the report's expected-pattern table and all detected patterns (including whether declared).
func patternTables(b *strings.Builder, e *Expect, in Input, r Result) {
	fmt.Fprintf(b, "## Expected patterns\n\n")
	if len(e.Patterns) == 0 {
		fmt.Fprintf(b, "(none)\n\n")
	} else {
		fmt.Fprintf(b, "| pattern | target | span_from | result | basis |\n|---|---|---|---|---|\n")
		for _, w := range e.Patterns {
			st := "present"
			if slices.ContainsFunc(r.MissingPatterns, func(x WantPattern) bool { return x.Pattern == w.Pattern && x.Target == w.Target }) {
				st = "**absent**"
			}
			from := "-"
			if w.SpanFrom != nil {
				from = fmt.Sprint(*w.SpanFrom)
			}
			fmt.Fprintf(b, "| %s | `%s` | %s | %s | %s |\n", w.Pattern, w.Target, from, st, cell(w.Why))
		}
		fmt.Fprintf(b, "\n")
	}
	fmt.Fprintf(b, "## Detected patterns\n\n")
	if len(in.Patterns) == 0 {
		fmt.Fprintf(b, "(none)\n\n")
		return
	}
	fmt.Fprintf(b, "| pattern | target | span | declared | expected |\n|---|---|---|---|---|\n")
	for _, m := range in.Patterns {
		exp := slices.ContainsFunc(e.Patterns, func(w WantPattern) bool { return w.matches(e.Work)(m) })
		fmt.Fprintf(b, "| %s | `%s` | %s | %v | %v |\n", m.Pattern, short(e.Work, m.TargetID), spanOf(m), m.Declared, exp)
	}
	fmt.Fprintf(b, "\n")
}
