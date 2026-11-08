package render

import (
	"context"
	"strings"
	"testing"
)

func TestApplyPatch(t *testing.T) {
	prose := "가는 갔다. 나는 왔다. 다는 웃었다."
	got, err := ApplyPatch(prose, []Edit{{"가는 갔다.", "가는 멈췄다."}, {"다는 웃었다.", "다는 울었다."}})
	if err != nil || got != "가는 멈췄다. 나는 왔다. 다는 울었다." {
		t.Fatalf("%q %v", got, err)
	}
	if got, err := ApplyPatch(prose, nil); err != nil || got != prose {
		t.Fatalf("empty: %q %v", got, err)
	}
	for name, es := range map[string][]Edit{
		"missing":  {{"없는 문장.", "x"}},
		"twice":    {{"는", "x"}},
		"overlap":  {{"가는 갔다. 나는", "x"}, {"나는 왔다.", "y"}},
		"emptyold": {{"", "x"}},
	} {
		if _, err := ApplyPatch(prose, es); !IsBadExtract(err) {
			t.Errorf("%s: err=%v, want badExtract", name, err)
		}
	}
}

func TestParsePatch(t *testing.T) {
	es, err := ParsePatch("```json\n{\"edits\":[{\"old\":\"a\",\"new\":\"b\"}]}\n```")
	if err != nil || len(es) != 1 || es[0] != (Edit{"a", "b"}) {
		t.Fatalf("%v %v", es, err)
	}
	if es, err := ParsePatch(`{"edits":[]}`); err != nil || len(es) != 0 {
		t.Fatalf("%v %v", es, err)
	}
	for _, bad := range []string{"산문", `{}`, `{"edits":[{"old":"a"}]}`} {
		if _, err := ParsePatch(bad); !IsBadExtract(err) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

// scripted answers by call kind: card prompts (BuildCardPrompt system) vs patch/render prompts.
type seqCompleter struct {
	replies []string
	log     *[]string
	kind    string
	users   []string
}

func (c *seqCompleter) Complete(_ context.Context, _, user string) (Result, error) {
	*c.log = append(*c.log, c.kind)
	c.users = append(c.users, user)
	r := c.replies[0]
	if len(c.replies) > 1 {
		c.replies = c.replies[1:]
	}
	return Result{Text: r, StopReason: "end_turn"}, nil
}

const passBody = `{"items":[
{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"},
{"id":"E2","fact_present":"yes","as_stated":"yes","quote":"q"},
{"id":"E3","fact_present":"yes","as_stated":"yes","quote":"q"}],
"withheld":[{"id":"W1","revealed":"no","quote":""}],"other":[]}`

func patchFixture(t *testing.T) (Backend, Scene, Scene) {
	b := mustBackends(t)[0]
	nw := cardScene()
	nw.ID = "s1"
	old := nw
	old.Beats = []Beat{{Acts: nw.Beats[0].Acts[:2]}}
	return b, old, nw
}

func TestPatchVerifiedRetryThenFallback(t *testing.T) {
	b, old, nw := patchFixture(t)
	var log []string
	rc := &seqCompleter{replies: []string{`{"edits":[{"old":"없다","new":"x"}]}`, `{"edits":[{"old":"또없다","new":"x"}]}`, "전체 렌더 산문"}, log: &log, kind: "R"}
	xc := &seqCompleter{replies: []string{passBody}, log: &log, kind: "X"}
	o := VerifyOpts{K: 1, Retries: 2, Verifier: "card", Card: CardOpts{Tier2: "off"}}
	r, n, err := PatchVerified(context.Background(), Completers{rc, xc}, "w", old, nw, b, "옛 산문입니다.", "", o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.Patched || r.Prose != "전체 렌더 산문" || n != 3 || strings.Join(log, "") != "RRRX" {
		t.Fatalf("prose=%q n=%d log=%v", r.Prose, n, log)
	}
	if !strings.Contains(rc.users[0], "변경 내역") || strings.Contains(rc.users[2], "변경 내역") {
		t.Errorf("prompts: patch must carry the diff, fallback must not")
	}
}

func TestPatchVerifiedCardFeedback(t *testing.T) {
	b, old, nw := patchFixture(t)
	var log []string
	edit := `{"edits":[{"old":"옛 산문","new":"새 산문"}]}`
	rc := &seqCompleter{replies: []string{edit}, log: &log, kind: "P"}
	xc := &seqCompleter{replies: []string{tierReply(t1Body).Text, passBody}, log: &log, kind: "C"}
	o := VerifyOpts{K: 1, Retries: 2, Verifier: "card", Card: CardOpts{Tier2: "off"}}
	r, n, err := PatchVerified(context.Background(), Completers{rc, xc}, "w", old, nw, b, "옛 산문입니다.", "\n\n## DIGEST-TEXT\n", o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(log, "") != "PCPC" || n != 2 || !r.Report.Patched || r.Prose != "새 산문입니다." || !Passed(r) {
		t.Fatalf("log=%v n=%d prose=%q %+v", log, n, r.Prose, r.Report)
	}
	if !strings.Contains(rc.users[1], "검수 결과") || strings.Contains(rc.users[0], "검수 결과") {
		t.Errorf("feedback only on the second patch")
	}
	if !strings.Contains(rc.users[0], "DIGEST-TEXT") {
		t.Errorf("patch prompt lacks digest")
	}
}

func TestSceneDiff(t *testing.T) {
	_, old, nw := patchFixture(t)
	rm, ad := SceneDiff(old, nw)
	if len(rm) != 0 || len(ad) != 1 {
		t.Fatalf("rm=%v ad=%v", rm, ad)
	}
}

func TestApplyPatchOverlappingOccurrences(t *testing.T) {
	if _, err := ApplyPatch("aaa", []Edit{{"aa", "x"}}); !IsBadExtract(err) {
		t.Fatalf("overlapping repeats must be ambiguous: %v", err)
	}
}
