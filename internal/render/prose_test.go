package render

import (
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
	} {
		if got := ProseDefinitions(tc.prose, tc.lang, []string{tc.surf}); len(got) != 1 || got[0] != tc.prose {
			t.Errorf("%q: got %v, want the sentence", tc.prose, got)
		}
	}
	for _, tc := range []struct{ prose, lang, surf string }{
		{`그가 말했다. "족보는 우리 목숨줄이야."`, "ko", "족보"},
		{"그 돔은 감옥 같았다.", "ko", "돔"},
		{"나는 돔 안에 있었다.", "ko", "돔"},
		{`He said, "A dome is a cage." Then he left.`, "en", "dome"},
		{"The dome was like a prison.", "en", "dome"},
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
