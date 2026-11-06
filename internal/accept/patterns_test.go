package accept

import (
	"strings"
	"testing"

	"diegesis/internal/pg"
)

const patternYAML = expectYAML + `patterns:
  - {pattern: dramatic-irony, target: "m/1.4", why: "1.4.13"}
  - {pattern: chekhov-gun, target: "m/guns", span_from: 3, why: "1.3.69"}
`

func intp(n int) *int { return &n }

// Review Focus 1: 기대 패턴은 대상까지 같아야 한다. 같은 패턴이 다른 대상에서 매치되면 FAIL.
// span_from 을 적으면 구간 시작도 같아야 한다.
func TestPatternTargetMustMatch(t *testing.T) {
	e, err := Parse([]byte(patternYAML))
	if err != nil {
		t.Fatal(err)
	}
	ok := []pg.Match{
		{Pattern: "dramatic-irony", TargetID: "w/m/1.4", SpanFrom: intp(4), SpanTo: intp(4)},
		{Pattern: "chekhov-gun", TargetID: "w/m/guns", SpanFrom: intp(3), SpanTo: intp(9)},
	}
	in := passing()
	in.Patterns = ok
	if r := Judge(e, in); !r.Pass {
		t.Fatalf("want PASS, failures %v", r.Failures)
	}
	for name, ms := range map[string][]pg.Match{
		"other target": {{Pattern: "dramatic-irony", TargetID: "w/m/1.6", SpanFrom: intp(6)}, ok[1]},
		"other span":   {ok[0], {Pattern: "chekhov-gun", TargetID: "w/m/guns", SpanFrom: intp(4)}},
		"none":         nil,
	} {
		in.Patterns = ms
		r := Judge(e, in)
		if r.Pass || len(r.MissingPatterns) == 0 || !strings.Contains(strings.Join(r.Failures, "\n"), "missing expected pattern") {
			t.Errorf("%s: want FAIL on the missing pattern, got pass=%v failures=%v", name, r.Pass, r.Failures)
		}
	}
}
