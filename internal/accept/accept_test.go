package accept

import (
	"strings"
	"testing"

	"silmari/internal/mutate"
	"silmari/internal/pg"
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
	return pg.Diagnostic{Rule: rule, TargetID: "w/" + target, Severity: sev, Waived: waived, File: "f.sil.md", Line: 3}
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

// 예상 밖의 억제되지 않은 warn 하나 → 불합격(오탐). info 와 억제된 warn 은 오탐이 아니다.
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

// 억제된 warn 도 expect.yaml 에 선언돼야 한다(Review Focus 1: 억제가 진짜 오류를 조용히 덮지 않게).
func TestAcceptUndeclaredWaiver(t *testing.T) {
	in := passing()
	in.Diags = append(in.Diags, diag("info-flow", "m/1.4#a2", "warn", true))
	r := Judge(parse(t), in)
	if r.Pass || len(r.Undeclared) != 1 {
		t.Errorf("pass=%v undeclared=%v, want FAIL with 1 undeclared waiver", r.Pass, r.Undeclared)
	}
}
