package accept

import (
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/mutate"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

const expectYAML = `work: w
compile_errors: 0
expected:
  - {rule: belief-contradiction, target: "m/1.3#a5", waived: true, why: "Cawdor"}
queries:
  - {name: q1, sql: "SELECT 1", rows: [["banquo", "1.3"]]}
mutation_min_rate: 0.8
`

func diag(rule, target, sev string, waived bool) pg.Diagnostic {
	return pg.Diagnostic{Rule: rule, TargetID: "w/" + target, Severity: sev, Waived: waived, File: "f.dieg.md", Line: 3}
}

func passing() Input {
	return Input{
		Diags: []pg.Diagnostic{diag("belief-contradiction", "m/1.3#a5", "warn", true), diag("info-flow", "m/1.2#a1", "info", false)},
		Rows:  map[string][][]string{"q1": {{"banquo", "1.3"}}},
		Mutants: []Mutant{
			{Mutation: mutate.Mutation{Op: "drop-tells"}, Outcome: Detected},
			{Mutation: mutate.Mutation{Op: "drop-enter"}, Outcome: Caught},
			{Mutation: mutate.Mutation{Op: "drop-enter"}, Outcome: Equivalent},
		},
	}
}

func parse(t *testing.T) *Expect {
	t.Helper()
	e, err := Parse([]byte(expectYAML))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAcceptPasses(t *testing.T) {
	r := Judge(parse(t), passing())
	if !r.Pass {
		t.Fatalf("want PASS, failures %v", r.Failures)
	}
	if r.Rate != 1 || r.Counted != 2 {
		t.Errorf("rate %v over %d, want 1 over 2 (equivalent excluded)", r.Rate, r.Counted)
	}
}

// One unexpected unsuppressed warn → fail (false positive). info and suppressed warns are not false positives.
func TestAcceptReportsFalsePositive(t *testing.T) {
	in := passing()
	in.Diags = append(in.Diags, diag("info-flow", "m/1.3#a7", "warn", false))
	r := Judge(parse(t), in)
	if r.Pass {
		t.Fatal("want FAIL on an unexpected unwaived warn")
	}
	if len(r.FalsePositives) != 1 || r.FalsePositives[0].TargetID != "w/m/1.3#a7" {
		t.Errorf("false positives = %v", r.FalsePositives)
	}
	if !strings.Contains(Report(parse(t), in, r), "1.3#a7") {
		t.Error("report does not name the false positive")
	}
}

func TestAcceptMissingExpected(t *testing.T) {
	for name, ds := range map[string][]pg.Diagnostic{
		"absent":         nil,
		"not waived":     {diag("belief-contradiction", "m/1.3#a5", "warn", false)},
		"other target":   {diag("belief-contradiction", "m/1.3#a6", "warn", true)},
		"other severity": {diag("belief-contradiction", "m/1.3#a5", "info", true)},
	} {
		in := passing()
		in.Diags = ds
		r := Judge(parse(t), in)
		if r.Pass || len(r.Missing) != 1 {
			t.Errorf("%s: pass=%v missing=%v, want FAIL with 1 missing", name, r.Pass, r.Missing)
		}
	}
}

func TestAcceptQueryMismatch(t *testing.T) {
	in := passing()
	in.Rows = map[string][][]string{"q1": {{"banquo", "1.4"}}}
	if r := Judge(parse(t), in); r.Pass {
		t.Error("want FAIL on a query row mismatch")
	}
	in.Rows = map[string][][]string{}
	if r := Judge(parse(t), in); r.Pass {
		t.Error("want FAIL on a query that did not run")
	}
}

func TestAcceptCompileErrors(t *testing.T) {
	in := passing()
	in.CompileErrors = 1
	if r := Judge(parse(t), in); r.Pass {
		t.Error("want FAIL on compile errors")
	}
}

func TestAcceptMutationRate(t *testing.T) {
	in := passing()
	in.Mutants = append(in.Mutants, Mutant{Mutation: mutate.Mutation{Op: "kill-early"}, Outcome: Survived})
	r := Judge(parse(t), in)
	if r.Pass || r.Rate >= 0.8 {
		t.Errorf("2/3 detected: pass=%v rate=%v, want FAIL", r.Pass, r.Rate)
	}
	in.Mutants = nil
	if r := Judge(parse(t), in); r.Pass {
		t.Error("want FAIL when a minimum rate is set but no mutant ran")
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	if _, err := Parse([]byte("work: w\nexpectd: []\n")); err == nil {
		t.Error("want an error for an unknown key")
	}
}

// Suppressed warns must also be declared in expect.yaml (Review Focus 1: so a suppression does not quietly cover a real error).
func TestAcceptUndeclaredWaiver(t *testing.T) {
	in := passing()
	in.Diags = append(in.Diags, diag("info-flow", "m/1.4#a2", "warn", true))
	r := Judge(parse(t), in)
	if r.Pass || len(r.Undeclared) != 1 {
		t.Errorf("pass=%v undeclared=%v, want FAIL with 1 undeclared waiver", r.Pass, r.Undeclared)
	}
}

// ADR-014: overall ≥ 0.8 and excluding move-before ≥ 0.7 (defaults). Even if all move-before are caught, 0.65 for the rest fails.
func TestJudgeRequiresBothRates(t *testing.T) {
	e := parse(t)
	if e.MutationMinRateWoMoveBefore != 0.7 {
		t.Errorf("default mutation_min_rate_wo_move_before = %v, want 0.7", e.MutationMinRateWoMoveBefore)
	}
	in := passing()
	in.Mutants = nil
	add := func(op string, n int, o Outcome) {
		for range n {
			in.Mutants = append(in.Mutants, Mutant{Mutation: mutate.Mutation{Op: op}, Outcome: o})
		}
	}
	add("move-before", 20, Detected)
	add("drop-tells", 13, Detected)
	add("drop-tells", 7, Survived)
	r := Judge(e, in) // overall 33/40 = 0.825, excluding 13/20 = 0.65
	if r.Pass || r.Rate < 0.8 || r.RateWoMoveBefore != 0.65 {
		t.Errorf("pass=%v rate=%v wo=%v, want FAIL with 0.825 / 0.65", r.Pass, r.Rate, r.RateWoMoveBefore)
	}
	in.Mutants = in.Mutants[:0]
	add("move-before", 3, Detected)
	add("drop-tells", 15, Detected)
	add("drop-tells", 2, DetectedByQuery)
	add("drop-tells", 3, Survived)
	if r := Judge(e, in); !r.Pass || r.HitWoMoveBefore != 17 || r.CountedWoMoveBefore != 20 {
		t.Errorf("pass=%v wo=%d/%d, want PASS with 17/20 (query detections count, R1)", r.Pass, r.HitWoMoveBefore, r.CountedWoMoveBefore)
	} else if r.HitLint != 18 || r.HitLintWoMoveBefore != 15 || r.RateLintWoMoveBefore != 0.75 {
		// The lint-only rate (query detection excluded) is only reported: same denominator, and detected-by-query is subtracted from the numerator
		t.Errorf("lint-only %d/%d, wo %d/%d = %v, want 18/23, 15/20 = 0.75", r.HitLint, r.Counted, r.HitLintWoMoveBefore, r.CountedWoMoveBefore, r.RateLintWoMoveBefore)
	}
}
