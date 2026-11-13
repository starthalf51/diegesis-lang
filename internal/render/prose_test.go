package render

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestProseDefinitions(t *testing.T) {
	for _, tc := range []struct{ prose, lang, surf string }{
		{"돔은 사람이 숨 쉬는 구역이다.", "ko", "돔"},
		{"족보란 시민 원장을 말한다.", "ko", "족보"},
		{"A dome is a sealed habitat.", "en", "dome"},
		{"This is called the Ledger.", "en", "Ledger"},
		{"족보란 시민 원장을 말했다.", "ko", "족보"},
		{"족보는 시민 원장이라고 불렸다.", "ko", "족보"},
		{`"그만해." 돔은 사람이 숨 쉬는 구역이다.`, "ko", "돔"},
	} {
		if got := ProseDefinitions(tc.prose, tc.lang, []string{tc.surf}); len(got) != 1 || !strings.HasSuffix(tc.prose, got[0]) {
			t.Errorf("%q: got %v, want the sentence", tc.prose, got)
		}
	}
	for _, tc := range []struct{ prose, lang, surf string }{
		{`그가 말했다. "족보는 우리 목숨줄이야."`, "ko", "족보"},
		{"그 돔은 감옥 같았다.", "ko", "돔"},
		{"나는 돔 안에 있었다.", "ko", "돔"},
		{`He said, "A dome is a cage." Then he left.`, "en", "dome"},
		{"The dome was like a prison.", "en", "dome"},
		{"돔은 텅 빈 공간이었다.", "ko", "돔"},
		{"The dome was a mess.", "en", "dome"},
		{"The dome was a long way off.", "en", "dome"},
	} {
		if got := ProseDefinitions(tc.prose, tc.lang, []string{tc.surf}); len(got) != 0 {
			t.Errorf("%q: should not be flagged, got %v", tc.prose, got)
		}
	}
	got := ProseDefinitions("문이 열렸다. 돔은 사람이 숨 쉬는 구역이다. 그는 걸었다.", "ko", []string{"돔"})
	if !slices.Equal(got, []string{"돔은 사람이 숨 쉬는 구역이다."}) {
		t.Errorf("multi-sentence: %v", got)
	}
}

func TestUnrenderedTerms(t *testing.T) {
	got := UnrenderedTerms("그는 돔을 올려다보았다.", []string{"돔", "족보"})
	if !slices.Equal(got, []string{"족보"}) {
		t.Errorf("got %v", got)
	}
	if got := UnrenderedTerms("The Dome loomed.", []string{"dome"}); len(got) != 0 {
		t.Errorf("case: %v", got)
	}
	if got := UnrenderedTerms("돔 족보", []string{"돔", "족보"}); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

// A new term makes the scene's prose checked: a definition sentence or a missing term fails the attempt, and the next prompt names it.
func TestVerifyProseChecksFeedback(t *testing.T) {
	s, b, good, _ := verifyFixture(t)
	s.TermsNew = []Term{{Surface: "dome", Gloss: "a sealed habitat"}}
	r := &recorder{replies: verifyReplies("A dome is a sealed habitat.", good, "He knocked on the dome door.", good)}
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if err != nil || n != 2 || !Passed(got) || got.Prose != "He knocked on the dome door." {
		t.Fatalf("n=%d err=%v %+v", n, err, got.Report)
	}
	second := r.prompts[2]
	i := strings.Index(second, "## Fix from the previous draft")
	if i < 0 || !strings.Contains(second[i:], "define a term") || !strings.Contains(second[i:], "A dome is a sealed habitat.") {
		t.Errorf("no definition feedback:\n%s", second)
	}
	// missing term
	r = &recorder{replies: verifyReplies("He walked.", good, "The dome loomed.", good)}
	got, n, _ = RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2})
	if n != 2 || !strings.Contains(r.prompts[2], "dome") || !strings.Contains(r.prompts[2], "do not appear") {
		t.Errorf("n=%d unrendered feedback missing:\n%s", n, r.prompts[2])
	}
	// at the cap the report says why it failed
	r = &recorder{replies: verifyReplies("He walked.", good)}
	got, _, _ = RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 0})
	if Passed(got) || len(got.Report.Prose) != 1 {
		t.Errorf("want prose failure, got %+v", got.Report)
	}
	// no terms: unchanged
	s.TermsNew = nil
	r = &recorder{replies: verifyReplies("A dome is a sealed habitat.", good)}
	if _, n, _ = RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2}); n != 1 {
		t.Errorf("no-terms scene retried: n=%d", n)
	}
}

// Every attempt's prose-check hits reach VerifyOpts.Log, tagged, and stay in Report.Prose.
func TestVerifyLogsProseHits(t *testing.T) {
	s, b, good, _ := verifyFixture(t)
	s.TermsNew = []Term{{Surface: "dome", Gloss: "a sealed habitat"}}
	r := &recorder{replies: verifyReplies("A dome is a sealed habitat.", good, "He walked.", good, "The dome loomed.", good)}
	var log []string
	got, n, err := RenderVerified(ctx, r, "w", s, b, VerifyOpts{K: 1, Retries: 2, Log: func(n int, hits []string) {
		for _, h := range hits {
			log = append(log, fmt.Sprintf("%d %s", n, h))
		}
	}})
	want := []string{"1 prose-definition: A dome is a sealed habitat.", "2 term-unrendered: dome"}
	if err != nil || n != 3 || !Passed(got) || !slices.Equal(log, want) {
		t.Fatalf("n=%d err=%v log=%q", n, err, log)
	}
}

// A failed attempt logs its missing, added (with kind) and leaked items; a passed one logs nothing.
func TestVerifyLogsFactFailures(t *testing.T) {
	s, b, good, _ := verifyFixture(t)
	e := extractedFromScene(s)
	for i, a := range e.Acts {
		if a.Fact != "" {
			e.Acts = append(e.Acts[:i], e.Acts[i+1:]...)
			break
		}
	}
	e.FactsStated = append(e.FactsStated, "letter_forged")
	e.NewEvents = []NewEvent{{Kind: "possession", Text: "Zhou Yu takes the key"}}
	raw, _ := json.Marshal(e)
	var log []string
	o := VerifyOpts{K: 1, Retries: 0, Log: func(n int, hits []string) {
		for _, h := range hits {
			log = append(log, fmt.Sprintf("%d %s", n, h))
		}
	}}
	got, _, err := RenderVerified(ctx, &recorder{replies: verifyReplies("p", "```json\n"+string(raw)+"\n```")}, "w", s, b, o)
	if err != nil || Passed(got) {
		t.Fatalf("err=%v %+v", err, got.Report)
	}
	joined := strings.Join(log, "\n")
	for _, want := range []string{"1 missing: ", "1 added(possession): Zhou Yu takes the key", "1 leaked: "} {
		if !strings.Contains(joined, want) {
			t.Errorf("log lacks %q:\n%s", want, joined)
		}
	}
	log = nil
	if _, _, err := RenderVerified(ctx, &recorder{replies: verifyReplies("p", good)}, "w", s, b, o); err != nil || len(log) != 0 {
		t.Errorf("passed render logged %v", log)
	}
}
