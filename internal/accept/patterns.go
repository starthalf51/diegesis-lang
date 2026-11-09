package accept

import (
	"fmt"
	"slices"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// matches: 패턴과 대상이 정확히 같고, SpanFrom 을 적었으면 구간 시작도 같다.
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

// patternTables 는 보고서의 기대 패턴 표와 검출된 패턴 전체(선언 여부 포함).
func patternTables(b *strings.Builder, e *Expect, in Input, r Result) {
	fmt.Fprintf(b, "## 기대 패턴\n\n")
	if len(e.Patterns) == 0 {
		fmt.Fprintf(b, "(없음)\n\n")
	} else {
		fmt.Fprintf(b, "| 패턴 | 대상 | span_from | 결과 | 근거 |\n|---|---|---|---|---|\n")
		for _, w := range e.Patterns {
			st := "있음"
			if slices.ContainsFunc(r.MissingPatterns, func(x WantPattern) bool { return x.Pattern == w.Pattern && x.Target == w.Target }) {
				st = "**없음**"
			}
			from := "-"
			if w.SpanFrom != nil {
				from = fmt.Sprint(*w.SpanFrom)
			}
			fmt.Fprintf(b, "| %s | `%s` | %s | %s | %s |\n", w.Pattern, w.Target, from, st, cell(w.Why))
		}
		fmt.Fprintf(b, "\n")
	}
	fmt.Fprintf(b, "## 검출된 패턴\n\n")
	if len(in.Patterns) == 0 {
		fmt.Fprintf(b, "(없음)\n\n")
		return
	}
	fmt.Fprintf(b, "| 패턴 | 대상 | 구간 | 선언 | 기대 |\n|---|---|---|---|---|\n")
	for _, m := range in.Patterns {
		exp := slices.ContainsFunc(e.Patterns, func(w WantPattern) bool { return w.matches(e.Work)(m) })
		fmt.Fprintf(b, "| %s | `%s` | %s | %v | %v |\n", m.Pattern, short(e.Work, m.TargetID), spanOf(m), m.Declared, exp)
	}
	fmt.Fprintf(b, "\n")
}
