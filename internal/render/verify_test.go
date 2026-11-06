package render

import (
	"encoding/json"
	"strings"
	"testing"
)

func verifyFixture(t *testing.T) (Scene, Backend, string, string) {
	t.Helper()
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	b := mustBackends(t)[0]
	bad := extractedFromScene(s)
	bad.NewEvents = []string{"주유가 장간을 죽인다"}
	raw, _ := json.Marshal(bad)
	return s, b, perfectJSON(t, s), "```json\n" + string(raw) + "\n```"
}

func verifyReplies(parts ...string) []string {
	var out []string
	for i := 0; i < len(parts); i += 2 {
		out = append(out, parts[i], parts[i+1]) // K=1: 렌더 1 + 재추출 1
	}
	return out
}

func TestVerifyPassesFirstTry(t *testing.T) {
	s, b, good, _ := verifyFixture(t)
	r := &recorder{replies: verifyReplies("산문1", good)}
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 1 || !Passed(got) || len(r.prompts) != 2 {
		t.Fatalf("n=%d err=%v prompts=%d %+v", n, err, len(r.prompts), got.Report)
	}
}

func TestVerifyRetriesWithFeedback(t *testing.T) {
	s, b, good, bad := verifyFixture(t)
	r := &recorder{replies: verifyReplies("산문1", bad, "산문2", good)}
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 2 || !Passed(got) || got.Prose != "산문2" {
		t.Fatalf("n=%d err=%v %+v", n, err, got.Report)
	}
	first, second := r.prompts[0], r.prompts[2]
	if strings.Contains(first, "## Fix from the previous draft") {
		t.Error("first render must have no feedback")
	}
	i := strings.Index(second, "## Fix from the previous draft")
	if i < 0 || !strings.Contains(second[i:], "remove") || !strings.Contains(second[i:], "주유가 장간을 죽인다") {
		t.Errorf("second prompt lacks remove feedback:\n%s", second)
	}
}

func TestVerifyGivesUpAtLimit(t *testing.T) {
	s, b, _, bad := verifyFixture(t)
	r := &recorder{replies: verifyReplies("a", bad, "b", bad, "c", bad)}
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 3 || Passed(got) || got.Prose != "c" || len(r.prompts) != 6 {
		t.Fatalf("n=%d err=%v prompts=%d %+v", n, err, len(r.prompts), got.Report)
	}
}

func TestVerifyMissingWithinBudget(t *testing.T) {
	s, b, _, _ := verifyFixture(t)
	e := extractedFromScene(s)
	e.Acts = e.Acts[1:]
	raw, _ := json.Marshal(e)
	r := &recorder{replies: verifyReplies("산문1", "```json\n"+string(raw)+"\n```")}
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 1 || len(got.Report.Missing) != 1 || !Passed(got) {
		t.Fatalf("n=%d err=%v %+v", n, err, got.Report)
	}
}
