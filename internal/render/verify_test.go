package render

import (
	"fmt"
	"encoding/json"
	"strings"
	"testing"
)

func verifyFixture(t *testing.T) (Scene, Backend, string, string) {
	t.Helper()
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	b := mustBackends(t)[0]
	bad := extractedFromScene(s)
	bad.NewEvents = []NewEvent{{Text: "주유가 장간을 죽인다"}}
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

func TestRenderVerified2UsesBoth(t *testing.T) {
	s, b, good, bad := verifyFixture(t)
	rc := &recorder{replies: []string{"산문1", "산문2"}}
	xc := &recorder{replies: []string{bad, good}}
	got, n, err := RenderVerified2(ctx, Completers{Render: rc, Extract: xc}, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 2 || !Passed(got) {
		t.Fatal(n, err, got.Report)
	}
	if len(rc.prompts) != 2 || len(xc.prompts) != 2 {
		t.Fatalf("render calls %d, extract calls %d", len(rc.prompts), len(xc.prompts))
	}
}

// cardReply 는 체크리스트 답: miss 번호(E1..)는 없음, leak 이면 W1 누출, 늘 G2 후보 하나.
func cardReply(s Scene, miss map[int]bool, leak bool) string {
	var it []string
	for i := range CardItems(s) {
		v := "yes"
		if miss[i+1] {
			v = "no"
		}
		it = append(it, fmt.Sprintf(`{"id":"E%d","fact_present":%q,"as_stated":%q}`, i+1, v, v))
	}
	lv := "no"
	if leak {
		lv = "yes"
	}
	return "```json\n" + fmt.Sprintf(`{"items":[%s],"withheld":[{"id":"W1","revealed":%q}],"other":[{"actor":"zz","verb":"does","action":"새일","text":"G2 후보 문장","quote":"x"}]}`, strings.Join(it, ","), lv) + "\n```"
}

func TestRenderVerifiedCard(t *testing.T) {
	s, b, _, _ := verifyFixture(t)
	rc := &recorder{replies: []string{"산문1", "산문2"}}
	xc := &recorder{replies: []string{cardReply(s, map[int]bool{1: true, 2: true}, true), cardReply(s, nil, false)}}
	o := VerifyOpts{K: 1, Retries: 2, Verifier: "card", Card: CardOpts{Tier2: "off"}}
	got, n, err := RenderVerified2(ctx, Completers{Render: rc, Extract: xc}, "w", s, b, o)
	if err != nil || n != 2 || !Passed(got) || got.Prose != "산문2" {
		t.Fatalf("n=%d err=%v %+v", n, err, got.Report)
	}
	if len(got.Report.Warnings) == 0 || got.Extractions != nil || got.ExtractHash != "" || len(got.Extracted.Acts) != 0 {
		t.Errorf("card mode: want warnings and empty extraction, got %+v", got)
	}
	fb := rc.prompts[1][strings.Index(rc.prompts[1], "## Fix from the previous draft"):]
	if !strings.Contains(fb, "include") || !strings.Contains(fb, "withheld") || strings.Contains(fb, "G2 후보 문장") || strings.Contains(fb, "새일") {
		t.Errorf("feedback:\n%s", fb)
	}
}

func TestRenderVerifiedTruncated(t *testing.T) {
	s, b, _, _ := verifyFixture(t)
	rc := &fakeCompleter{replies: []Result{{Text: "잘림", StopReason: "max_tokens"}}}
	xc := &fakeCompleter{}
	got, _, err := RenderVerified2(ctx, Completers{Render: rc, Extract: xc}, "w", s, b, VerifyOpts{K: 1, Verifier: "card"})
	if err != nil || xc.calls != 0 || Passed(got) {
		t.Fatalf("err=%v score calls=%d %+v", err, xc.calls, got.Report)
	}
}
