// Package accept is the corpus acceptance judge. It compares expect.yaml (expected values from the source work and scholarship) with the actual results
// (compile error count, diagnostics, named queries, mutation detection). The judging rules themselves only check the results PG produced
// (ADR-010: belief, time and lint verdicts live in PG).
package accept

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/starthalf51/diegesis-lang/internal/mutate"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// Expect is expect.yaml.
type Expect struct {
	Work                        string  `yaml:"work"`
	CompileErrors               int     `yaml:"compile_errors"`
	Expected                    []Want  `yaml:"expected"`
	Queries                     []Query `yaml:"queries"`
	MutationMinRate             float64 `yaml:"mutation_min_rate"`
	MutationMinRateWoMoveBefore float64 `yaml:"mutation_min_rate_wo_move_before"`
	RenderScene                 string  `yaml:"render_scene"` // the scene dieg render-check renders (short ID)
	Patterns                    []WantPattern `yaml:"patterns"`
}

// Want is one diagnostic that must appear. Target is the target ID without the work ID (`macbeth.act1/1.3#a5`).
// Severity defaults to warn. Why is the source/scholarly basis (carried into the report).
type Want struct {
	Rule     string `yaml:"rule"`
	Target   string `yaml:"target"`
	Severity string `yaml:"severity"`
	Waived   bool   `yaml:"waived"`
	Why      string `yaml:"why"`
}

// WantPattern is one pattern match that must appear. Target is the target ID without the work ID and must match exactly (Review Focus 1).
// If SpanFrom is given, the span start (discourse_ord) must match too. Why is the evidence in the source text (line number).
type WantPattern struct {
	Pattern  string `yaml:"pattern"`
	Target   string `yaml:"target"`
	SpanFrom *int   `yaml:"span_from"`
	Why      string `yaml:"why"`
}

// Query is a named SQL ($1 = work ID) and the expected rows (must match including order).
type Query struct {
	Name string     `yaml:"name"`
	SQL  string     `yaml:"sql"`
	Rows [][]string `yaml:"rows"`
	Why  string     `yaml:"why"`
}

// Parse treats unknown keys as errors (so a typo cannot silently erase an expectation).
func Parse(b []byte) (*Expect, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var e Expect
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("expect.yaml: %w", err)
	}
	if e.Work == "" {
		return nil, fmt.Errorf("expect.yaml: work is required")
	}
	if e.MutationMinRateWoMoveBefore == 0 {
		e.MutationMinRateWoMoveBefore = 0.7 // ADR-014
	}
	for i := range e.Expected {
		if e.Expected[i].Severity == "" {
			e.Expected[i].Severity = "warn"
		}
	}
	return &e, nil
}

// Outcome is the result of one mutant.
type Outcome string

const (
	Detected Outcome = "detected" // one of the expected rules produced a new warn at the mutation site (Site)
	// DetectedByQuery: no expected warn within the site, but the expect.yaml query results differ from the original (R1) — counted as detection
	DetectedByQuery Outcome = "detected-by-query"
	Caught          Outcome = "caught"     // the compiler (syntax/semantic analysis) rejected the mutant — counted as detection. Does not occur for IR mutations
	Equivalent      Outcome = "equivalent" // not detected and derived rows equal the original — excluded from the rate
	Survived        Outcome = "survived"
	// NotApplicable: mutants that PG confirms produce no defect (the received information is not used later, no source).
	// Not run, only left in the list. Excluded from the rate.
	NotApplicable Outcome = "n/a"
)

// Mutant is one mutation and its result. Rules = the rules of the new unsuppressed warns from the mutant.
type Mutant struct {
	Mutation mutate.Mutation
	Outcome  Outcome
	Rules    []string
	Queries  []string
	Note     string
}

// Input is the actual result. Diags holds lint diagnostics and compile warnings together.
// Rows maps query name → rows (queries that could not run are absent). Mutants nil = mutants were not run.
type Input struct {
	CompileErrors int
	Diags         []pg.Diagnostic
	Rows          map[string][][]string
	QueryErrors   map[string]string
	Patterns      []pg.Match // result of pg.Patterns (whole work)
	Mutants       []Mutant
}

type Result struct {
	Pass                                 bool
	Failures                             []string
	Missing                              []Want
	FalsePositives                       []pg.Diagnostic
	Undeclared                           []pg.Diagnostic // suppressed but not in expect.yaml warns
	BadQueries                           []string
	MissingPatterns                      []WantPattern
	Rate                                 float64
	Counted                              int // denominator of the rate (excluding equivalent mutants)
	Hit                                  int // numerator (detected + caught)
	RateWoMoveBefore                     float64
	CountedWoMoveBefore, HitWoMoveBefore int
	// Lint only (the numerator without detected-by-query, same denominator). For reporting and not used in the verdict (R1).
	HitLint, HitLintWoMoveBefore   int
	RateLint, RateLintWoMoveBefore float64
}

// Judge: compile error count matches ∧ all expected present ∧ warns outside expected = 0 (unsuppressed ones are false positives,
// suppressed ones are undeclared suppressions)
// ∧ query results match ∧ mutation detection rate ≥ threshold.
func Judge(e *Expect, in Input) Result {
	var r Result
	fail := func(f string, a ...any) { r.Failures = append(r.Failures, fmt.Sprintf(f, a...)) }
	if in.CompileErrors != e.CompileErrors {
		fail("compile errors: %d, want %d", in.CompileErrors, e.CompileErrors)
	}
	matched := make([]bool, len(in.Diags))
	for _, w := range e.Expected {
		found := false
		for i, d := range in.Diags {
			if d.Rule == w.Rule && short(e.Work, d.TargetID) == w.Target && d.Severity == w.Severity && d.Waived == w.Waived {
				found, matched[i] = true, true
			}
		}
		if !found {
			r.Missing = append(r.Missing, w)
			fail("missing expected %s %s (%s, waived=%v)", w.Rule, w.Target, w.Severity, w.Waived)
		}
	}
	for i, d := range in.Diags {
		switch {
		case matched[i] || !finding(d.Severity):
		case !d.Waived:
			r.FalsePositives = append(r.FalsePositives, d)
			fail("false positive: %s", d)
		default: // every suppression must be declared as an expectation (Review Focus 1)
			r.Undeclared = append(r.Undeclared, d)
			fail("waived warn not declared in expect.yaml: %s", d)
		}
	}
	for _, q := range e.Queries {
		got, ok := in.Rows[q.Name]
		if !ok || !rowsEqual(got, q.Rows) {
			r.BadQueries = append(r.BadQueries, q.Name)
			fail("query %s: got %v, want %v %s", q.Name, got, q.Rows, in.QueryErrors[q.Name])
		}
	}
	for _, w := range e.Patterns {
		if !slices.ContainsFunc(in.Patterns, w.matches(e.Work)) {
			r.MissingPatterns = append(r.MissingPatterns, w)
			fail("missing expected pattern %s %s%s", w.Pattern, w.Target, spanText(w.SpanFrom))
		}
	}
	for _, m := range in.Mutants {
		hit, lint, counted := 0, 0, 0
		switch m.Outcome {
		case Detected, Caught:
			hit, lint, counted = 1, 1, 1
		case DetectedByQuery:
			hit, counted = 1, 1
		case Survived:
			counted = 1
		}
		r.Hit, r.HitLint, r.Counted = r.Hit+hit, r.HitLint+lint, r.Counted+counted
		if m.Mutation.Op != "move-before" {
			r.HitWoMoveBefore, r.HitLintWoMoveBefore, r.CountedWoMoveBefore =
				r.HitWoMoveBefore+hit, r.HitLintWoMoveBefore+lint, r.CountedWoMoveBefore+counted
		}
	}
	if r.Counted > 0 {
		r.Rate, r.RateLint = float64(r.Hit)/float64(r.Counted), float64(r.HitLint)/float64(r.Counted)
	}
	if r.CountedWoMoveBefore > 0 {
		r.RateWoMoveBefore = float64(r.HitWoMoveBefore) / float64(r.CountedWoMoveBefore)
		r.RateLintWoMoveBefore = float64(r.HitLintWoMoveBefore) / float64(r.CountedWoMoveBefore)
	}
	if e.MutationMinRate > 0 && (r.Counted == 0 || r.Rate < e.MutationMinRate) {
		fail("mutation detection %d/%d = %.2f, want ≥ %.2f", r.Hit, r.Counted, r.Rate, e.MutationMinRate)
	}
	// move-before is caught almost always (moving before the source means not knowing). The other operators alone must also clear the threshold (ADR-014)
	if e.MutationMinRate > 0 && (r.CountedWoMoveBefore == 0 || r.RateWoMoveBefore < e.MutationMinRateWoMoveBefore) {
		fail("mutation detection without move-before %d/%d = %.2f, want ≥ %.2f", r.HitWoMoveBefore, r.CountedWoMoveBefore,
			r.RateWoMoveBefore, e.MutationMinRateWoMoveBefore)
	}
	r.Pass = len(r.Failures) == 0
	return r
}

func short(work, id string) string { return strings.TrimPrefix(id, work+"/") }

func rowsEqual(a, b [][]string) bool {
	return slices.EqualFunc(a, b, func(x, y []string) bool { return slices.Equal(x, y) })
}

// Report is the body of out/accept/<work>.md.
func Report(e *Expect, in Input, r Result) string {
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	verdict := "FAIL"
	if r.Pass {
		verdict = "PASS"
	}
	p("# Acceptance verdict — %s: **%s**\n\n", e.Work, verdict)
	p("| item | result | criterion |\n|---|---|---|\n")
	p("| compile errors | %d | %d |\n", in.CompileErrors, e.CompileErrors)
	p("| expected diagnostics | %d/%d | all |\n", len(e.Expected)-len(r.Missing), len(e.Expected))
	p("| false positives (unsuppressed warns outside expected) | %d | 0 |\n", len(r.FalsePositives))
	p("| undeclared suppressions (suppressed warns outside expected) | %d | 0 |\n", len(r.Undeclared))
	p("| queries | %d/%d | all |\n", len(e.Queries)-len(r.BadQueries), len(e.Queries))
	p("| expected patterns | %d/%d | all |\n", len(e.Patterns)-len(r.MissingPatterns), len(e.Patterns))
	p("| mutation detection rate | %d/%d = %.2f | ≥ %.2f |\n", r.Hit, r.Counted, r.Rate, e.MutationMinRate)
	p("| mutation detection rate (excluding move-before) | %d/%d = %.2f | ≥ %.2f |\n\n", r.HitWoMoveBefore, r.CountedWoMoveBefore,
		r.RateWoMoveBefore, e.MutationMinRateWoMoveBefore)
	if len(r.Failures) > 0 {
		p("## Reasons for failure\n\n")
		for _, f := range r.Failures {
			p("- %s\n", f)
		}
		p("\n")
	}
	p("## Expected diagnostics\n\n| rule | target | severity | suppression | result | basis |\n|---|---|---|---|---|---|\n")
	for _, w := range e.Expected {
		st := "present"
		if slices.Contains(r.Missing, w) {
			st = "**absent**"
		}
		p("| %s | `%s` | %s | %v | %s | %s |\n", w.Rule, w.Target, w.Severity, w.Waived, st, cell(w.Why))
	}
	p("\n## All diagnostics\n\n")
	if len(in.Diags) == 0 {
		p("(none)\n")
	}
	for _, d := range in.Diags {
		p("- `%s` — target `%s`\n", d, short(e.Work, d.TargetID))
	}
	p("\n## Queries\n\n")
	for _, q := range e.Queries {
		st := "match"
		if slices.Contains(r.BadQueries, q.Name) {
			st = "**mismatch**"
		}
		p("### %s — %s\n\n%s\n\n```sql\n%s\n```\n\nexpected `%v` · actual `%v` %s\n\n", q.Name, st, q.Why, strings.TrimSpace(q.SQL),
			q.Rows, in.Rows[q.Name], in.QueryErrors[q.Name])
	}
	patternTables(&b, e, in, r)
	p("## Mutants\n\n")
	if in.Mutants == nil {
		p("(not run)\n")
		return b.String()
	}
	mutantTables(&b, e.Work, in.Mutants)
	return b.String()
}

// mutantTables is the per-operator tally (generated/detected/equivalent/survived) and the per-mutant table.
func mutantTables(b *strings.Builder, work string, ms []Mutant) {
	type tally struct{ gen, det, query, caught, eq, surv, na int }
	var ops []string
	by := map[string]*tally{}
	for _, m := range ms {
		t := by[m.Mutation.Op]
		if t == nil {
			t = &tally{}
			by[m.Mutation.Op] = t
			ops = append(ops, m.Mutation.Op)
		}
		t.gen++
		switch m.Outcome {
		case Detected:
			t.det++
		case DetectedByQuery:
			t.query++
		case Caught:
			t.caught++
		case Equivalent:
			t.eq++
		case NotApplicable:
			t.na++
		default:
			t.surv++
		}
	}
	fmt.Fprintf(b, "| operator | generated | not applicable (excluded) | detected (rule, in site) | detected (query) | detected (compiler) | equivalent (excluded) | survived |\n|---|---|---|---|---|---|---|---|\n")
	var all, noMove tally
	for _, op := range ops {
		t := by[op]
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", op, t.gen, t.na, t.det, t.query, t.caught, t.eq, t.surv)
		for _, s := range []*tally{&all, &noMove} {
			if s == &noMove && op == "move-before" {
				continue
			}
			s.det, s.query, s.caught, s.surv = s.det+t.det+t.query, s.query+t.query, s.caught+t.caught, s.surv+t.surv
		}
	}
	rate := func(t tally) string {
		n := t.det + t.caught + t.surv
		if n == 0 {
			return "-"
		}
		return fmt.Sprintf("%d/%d = %.2f", t.det+t.caught, n, float64(t.det+t.caught)/float64(n))
	}
	lintOnly := func(t tally) string { // the numerator without query detection, same denominator
		n := t.det + t.caught + t.surv
		if n == 0 {
			return "-"
		}
		k := t.det - t.query + t.caught
		return fmt.Sprintf("%d/%d = %.2f", k, n, float64(k)/float64(n))
	}
	fmt.Fprintf(b, "\ndetection rate: all %s · excluding move-before %s\n", rate(all), rate(noMove))
	fmt.Fprintf(b, "\nlint only (query detection excluded, not used in the verdict): all %s · excluding move-before %s\n", lintOnly(all), lintOnly(noMove))
	fmt.Fprintf(b, "\n| # | operator | target | expected rule | result | new warn rule | note |\n|---|---|---|---|---|---|---|\n")
	for i, m := range ms {
		fmt.Fprintf(b, "| m%d | %s | `%s` | %s | %s | %s | %s |\n", i+1, m.Mutation.Op, cell(shortAll(work, m.Mutation.Target)),
			strings.Join(m.Mutation.Expect, ", "), m.Outcome, strings.Join(m.Rules, ", "), cell(m.Note))
	}
}

func shortAll(work, s string) string { return strings.ReplaceAll(s, work+"/", "") }

func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }

// finding: warn and error diagnostics are findings (expectations, false positives, mutant kills); info is a report.
func finding(sev string) bool { return sev == "warn" || sev == "error" }
