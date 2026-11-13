package render

import (
	"reflect"
	"strings"
	"testing"
)

var fleshContract = []string{
	"After each key act (speech, discovery, decision, reveal), give the point-of-view character's reaction — body, feeling, or thought.",
	"sensory detail, gestures, small movements within the same place, the point-of-view character's inner thoughts, atmosphere.",
	"new events; new facts; any change of possession, location (leaving the place), life or death, or relationship; new named characters or places; anything that contradicts the acts or facts; another character's unspoken thoughts in first person.",
}

func TestFleshContract(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/"+firstScene(t))
	legacy := &StyleSample{ID: "x", Lang: "en", Rules: []string{"r"}}
	novel := &StyleSample{ID: "x", Lang: "en", Rules: []string{"r"}, Flesh: "novel"}
	sysL, userL, err := BuildPrompt(s, VariantStyleDirectives, "en", legacy)
	if err != nil {
		t.Fatal(err)
	}
	sysN, userN, err := BuildPrompt(s, VariantStyleDirectives, "en", novel)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fleshContract {
		if !strings.Contains(sysN, c) {
			t.Errorf("novel system prompt missing %q", c)
		}
		if strings.Contains(sysL, c) {
			t.Errorf("legacy system prompt has %q", c)
		}
	}
	const noAdd = "do not add events, actions, or facts that are not in the specification"
	if !strings.Contains(sysL, noAdd) || strings.Contains(sysN, noAdd) {
		t.Errorf("no-add rule: legacy %v, novel %v", strings.Contains(sysL, noAdd), strings.Contains(sysN, noAdd))
	}
	if userN != userL {
		t.Error("flesh must not change the user prompt")
	}

	// re-extraction: flesh is carried on the scene; legacy prompt is byte-pinned
	xs, xu := BuildExtractPrompt(Scene{}, "en", "")
	if got := sum(xs, xu); got != "fe61d04bcbf5dabdc0542cafd82381456ac57710952256dc880877f60b6ad312" {
		t.Errorf("legacy extract prompt changed: %s", got)
	}
	sn := s
	sn.Flesh = "novel"
	xL, _ := BuildExtractPrompt(s, "en", "p")
	xN, _ := BuildExtractPrompt(sn, "en", "p")
	for _, w := range fleshVerifierPhrases {
		if !strings.Contains(xN, w) || strings.Contains(xL, w) {
			t.Errorf("extract prompt %q: novel %v, legacy %v", w, strings.Contains(xN, w), strings.Contains(xL, w))
		}
	}
	if ExtractHash(s, "en") == ExtractHash(sn, "en") {
		t.Error("flesh must change ExtractHash")
	}

	// card verifier: legacy pinned, novel asks for kinds
	if got := sum(cardSys1); got != "e73ff7bd6918a69d0a344cf3ac9f6d0e37175632de3cececd9794b7b0327104c" {
		t.Errorf("legacy card prompt changed: %s", got)
	}
	cL, _ := BuildCardPrompt(s, "en", "p")
	cN, _ := BuildCardPrompt(sn, "en", "p")
	if cL != cardSys1 || !containsAll(cN, fleshVerifierPhrases) {
		t.Error("card prompt does not follow flesh")
	}

	// backend file
	ok := writeBackend(t, `{"id":"x","lang":"en","sample":{"id":"x","lang":"en","rules":["r"],"flesh":"novel"}}`)
	bs, err := LoadBackends(ok)
	if err != nil || bs[0].Sample.Flesh != "novel" {
		t.Fatalf("flesh novel backend: %v", err)
	}
	if _, err := LoadBackends(writeBackend(t, `{"id":"x","lang":"en","sample":{"id":"x","lang":"en","rules":["r"],"flesh":"poem"}}`)); err == nil {
		t.Error("unknown flesh must be an error")
	}
}

func fleshScene() Scene {
	return Scene{
		Cast:  []Entity{{ID: "min", Names: map[string][]string{"ko": {"민"}}}, {ID: "jun", Names: map[string][]string{"ko": {"준"}}}},
		Facts: []Fact{{ID: "f1", Text: "the key is lost", True: true}},
		Beats: []Beat{{Acts: []Act{{Actor: "min", Verb: "tells", Targets: []string{"jun"}, Fact: "f1"}}}},
	}
}

// Schema pinned: new_events items carry "kind", one of event, possession, location, life, relationship, name, contradiction, fact, mind, speech
// (plot) or texture (never counted under flesh "novel"). Empty or unknown kind counts (fail closed).
func TestFleshDiff(t *testing.T) {
	e, err := ParseExtracted(`{"acts":[{"actor":"min","verb":"tells","targets":["jun"],"fact":"f1"}],"facts_stated":["f1"],"new_events":[
{"kind":"texture","actor":"min","verb":"does","targets":[],"action":"tremble","text":"민의 손이 떨렸다"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	s := fleshScene()
	s.Flesh = "novel"
	if r := Diff(s, e); len(r.Hallucinated) != 0 {
		t.Errorf("texture counted: %v", r.Hallucinated)
	}
	if r := Diff(fleshScene(), e); len(r.Hallucinated) != 1 {
		t.Errorf("legacy must ignore kind: %v", r.Hallucinated)
	}
	for _, ev := range []NewEvent{
		{Kind: "location", Actor: "min", Verb: "exit", Text: "민이 방을 나갔다"},
		{Kind: "name", Actor: "kim", Verb: "enter", Text: "낯선 사람 김이 들어왔다"},
		{Kind: "", Actor: "min", Verb: "does", Action: "steal", Text: "민이 열쇠를 훔쳤다"},
	} {
		x := e
		x.NewEvents = append(append([]NewEvent{}, e.NewEvents...), ev)
		if r := Diff(s, x); len(r.Hallucinated) != 1 {
			t.Errorf("%q: hallucinated %v", ev.Text, r.Hallucinated)
		}
	}
	// vote drops texture before counting, so it cannot hide a plot addition
	loc := NewEvent{Kind: "location", Actor: "min", Verb: "exit", Text: "민이 방을 나갔다"}
	xs := []Extracted{e, {Acts: e.Acts, FactsStated: e.FactsStated, NewEvents: []NewEvent{loc}}, {Acts: e.Acts, FactsStated: e.FactsStated, NewEvents: []NewEvent{loc}}}
	if v := Vote(s, xs); !reflect.DeepEqual(v.NewEvents, []NewEvent{loc}) {
		t.Errorf("vote: %+v", v.NewEvents)
	}
	// Score (judging cached records) takes flesh from the record
	r := Score(fleshScene(), Render{Lang: "en", Flesh: "novel", Extractions: []Extracted{e, e, e}})
	if len(r.Report.Hallucinated) != 0 {
		t.Errorf("score: %v", r.Report.Hallucinated)
	}
}

func TestFleshCard(t *testing.T) {
	s := fleshScene()
	f := &fakeCompleter{replies: []Result{tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"}],"withheld":[],
"other":[{"kind":"texture","actor":"min","verb":"does","action":"tremble","text":"손이 떨렸다","quote":"x"},
{"kind":"location","actor":"min","verb":"exit","text":"방을 나갔다","quote":"y"}]}`)}}
	r, err := ScoreCard(ctx, f, s, Backend{Lang: "en", Sample: StyleSample{Flesh: "novel"}}, "prose", CardOpts{Tier2: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Hallucinated, []string{"방을 나갔다"}) || len(r.Warnings) != 0 {
		t.Errorf("hallucinated %v, warnings %v", r.Hallucinated, r.Warnings)
	}
}

// fleshVerifierPhrases must appear in both novel verifier prompts (re-extraction and card).
var fleshVerifierPhrases = []string{
	`"kind"`,
	"event, possession, location, life, relationship, name, contradiction, fact, mind, speech",
	"any line of speech or reported speech that is not one of the acts",
	"a revelation of a new fact",
	"another character's unspoken thoughts in first-person narration",
	"Do not report sensory detail",
	"the point-of-view character's reactions or inner thoughts that state no new fact",
}

func containsAll(s string, ws []string) bool {
	for _, w := range ws {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

func TestFleshFactMind(t *testing.T) {
	s := fleshScene()
	s.Flesh = "novel"
	base := Extracted{Acts: []ExtractedAct{{Actor: "min", Verb: "tells", Targets: []string{"jun"}, Fact: "f1"}}, FactsStated: []string{"f1"}}
	for _, ev := range []NewEvent{
		{Kind: "fact", Actor: "min", Verb: "thinks", Text: "준은 처음부터 알고 있었다"},
		{Kind: "mind", Actor: "jun", Verb: "thinks", Text: "준은 속으로 민을 의심했다"},
		{Kind: "speech", Actor: "jun", Verb: "says", Text: "준이 \"여기 있다.\" 하고 말했다"},
	} {
		x := base
		x.NewEvents = []NewEvent{ev}
		if r := Diff(s, x); len(r.Hallucinated) != 1 {
			t.Errorf("%s: hallucinated %v", ev.Kind, r.Hallucinated)
		}
	}
	f := &fakeCompleter{replies: []Result{tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"}],"withheld":[],
"other":[{"kind":"fact","actor":"min","verb":"thinks","text":"준은 알고 있었다","quote":"x"},
{"kind":"mind","actor":"jun","verb":"thinks","text":"준은 의심했다","quote":"y"},
{"kind":"speech","actor":"jun","verb":"says","text":"준이 여기 있다고 말했다","quote":"z"}]}`)}}
	r, err := ScoreCard(ctx, f, fleshScene(), Backend{Lang: "en", Sample: StyleSample{Flesh: "novel"}}, "prose", CardOpts{Tier2: "off"})
	if err != nil || !reflect.DeepEqual(r.Hallucinated, []string{"준은 알고 있었다", "준은 의심했다", "준이 여기 있다고 말했다"}) {
		t.Errorf("card hallucinated %v, %v", r.Hallucinated, err)
	}
}

// T3-R2: under flesh "novel" card plot additions are Hallucinated (fail + feedback); a speech addition is covered by a
// fact-less speech act only in FreeDialogue mode. Legacy keeps warnings only and coveredByIR as before.
func TestFleshCardFails(t *testing.T) {
	reply := `{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"}],"withheld":[],
"other":[{"kind":"speech","actor":"jun","verb":"says","targets":["min"],"text":"준이 여기 있다고 말했다","quote":"여기 있다."}]}`
	score := func(s Scene, flesh string) Report {
		t.Helper()
		f := &fakeCompleter{replies: []Result{tierReply(reply)}}
		r, err := ScoreCard(ctx, f, s, Backend{Lang: "en", Sample: StyleSample{Flesh: flesh}}, "prose", CardOpts{Tier2: "off"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	const line = "준이 여기 있다고 말했다"
	rep := score(fleshScene(), "novel")
	if !reflect.DeepEqual(rep.Hallucinated, []string{line}) || len(rep.Warnings) != 0 {
		t.Errorf("novel: hallucinated %v warnings %v", rep.Hallucinated, rep.Warnings)
	}
	if Passed(Render{Report: rep}) || !strings.Contains(feedback(fleshScene(), "en", rep), line) || !strings.Contains(patchFeedback(fleshScene(), "en", rep), line) {
		t.Error("novel speech addition must fail and be fed back")
	}
	talk := fleshScene() // jun has a fact-less speech act toward min
	talk.Beats[0].Acts = append(talk.Beats[0].Acts, Act{Actor: "jun", Verb: "says", Targets: []string{"min"}})
	if rep := score(talk, "novel"); !reflect.DeepEqual(rep.Hallucinated, []string{line}) {
		t.Errorf("novel, no free dialogue: fact-less act must not cover invented speech: %v", rep.Hallucinated)
	}
	free := talk
	free.FreeDialogue = true
	if rep := score(free, "novel"); len(rep.Hallucinated)+len(rep.Warnings) != 0 {
		t.Errorf("novel + free dialogue: covered, got %v / %v", rep.Hallucinated, rep.Warnings)
	}
	if rep := score(fleshScene(), ""); len(rep.Hallucinated) != 0 || !reflect.DeepEqual(rep.Warnings, []string{line}) {
		t.Errorf("legacy: %v / %v", rep.Hallucinated, rep.Warnings)
	}
	if rep := score(talk, ""); len(rep.Hallucinated)+len(rep.Warnings) != 0 {
		t.Errorf("legacy coveredByIR: %v / %v", rep.Hallucinated, rep.Warnings)
	}
}

// I1: under flesh "novel" a fact-less act covers only an event addition with the same action word (and speech
// only with FreeDialogue); plot kinds are never covered. Legacy keeps the actor + verb-class match.
func TestFleshCardCoverageKinds(t *testing.T) {
	s := fleshScene()
	s.Beats[0].Acts = append(s.Beats[0].Acts, Act{Actor: "min", Verb: "does", Action: "wait"})
	score := func(flesh, other string) Report {
		t.Helper()
		f := &fakeCompleter{replies: []Result{tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"}],"withheld":[],"other":[` + other + `]}`)}}
		r, err := ScoreCard(ctx, f, s, Backend{Lang: "en", Sample: StyleSample{Flesh: flesh}}, "prose", CardOpts{Tier2: "off"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, other := range []string{
		`{"kind":"possession","actor":"min","verb":"does","action":"take","text":"민이 열쇠를 집었다","quote":"x"}`,
		`{"kind":"life","actor":"min","verb":"does","action":"kill","text":"민이 경비를 죽였다","quote":"x"}`,
		`{"kind":"location","actor":"min","verb":"does","action":"leave","text":"민이 도시로 떠났다","quote":"x"}`,
		`{"kind":"event","actor":"min","verb":"does","action":"run","text":"민이 달렸다","quote":"x"}`,
		`{"kind":"","actor":"min","verb":"does","action":"take","text":"민이 열쇠를 집었다","quote":"x"}`,
	} {
		if rep := score("novel", other); Passed(Render{Report: rep}) || len(rep.Hallucinated) != 1 {
			t.Errorf("novel must fail %s: %v", other, rep.Hallucinated)
		}
		if rep := score("", other); len(rep.Hallucinated)+len(rep.Warnings) != 0 {
			t.Errorf("legacy coveredByIR changed for %s: %v / %v", other, rep.Hallucinated, rep.Warnings)
		}
	}
	if rep := score("novel", `{"kind":"event","actor":"min","verb":"does","action":"wait","text":"민이 기다렸다","quote":"x"}`); len(rep.Hallucinated)+len(rep.Warnings) != 0 {
		t.Errorf("novel event with the act's action word is covered: %v / %v", rep.Hallucinated, rep.Warnings)
	}
}

// The card path keeps the addition's kind for the verify log.
func TestFleshCardKeepsKind(t *testing.T) {
	s := fleshScene()
	f := &fakeCompleter{replies: []Result{tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"}],"withheld":[],"other":[{"kind":"possession","actor":"min","verb":"does","action":"take","text":"민이 열쇠를 집었다","quote":"x"}]}`)}}
	rep, err := ScoreCard(ctx, f, s, Backend{Lang: "en", Sample: StyleSample{Flesh: "novel"}}, "prose", CardOpts{Tier2: "off"})
	if err != nil || rep.Kinds["민이 열쇠를 집었다"] != "possession" {
		t.Fatalf("err=%v %+v", err, rep)
	}
	if got := factHits(s, "en", rep); len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), "added(possession): 민이 열쇠를 집었다") {
		t.Errorf("hits %v", got)
	}
}
