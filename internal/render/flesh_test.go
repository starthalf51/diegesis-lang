package render

import (
	"reflect"
	"strings"
	"testing"
)

var fleshContract = []string{
	"After each key act (speech, discovery, decision, reveal), give the point-of-view character's reaction — body, feeling, or thought.",
	"sensory detail, gestures, small movements within the same place, the point-of-view character's inner thoughts, atmosphere.",
	"new events; any change of possession, location (leaving the place), life or death, or relationship; new named characters or places; anything that contradicts the acts or facts; another character's unspoken thoughts in first person.",
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
	for _, w := range []string{`"kind"`, "event, possession, location, life, relationship, name, contradiction", "Do not report sensory detail"} {
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
	if cL != cardSys1 || !strings.Contains(cN, `"kind"`) || !strings.Contains(cN, "Do not report sensory detail") {
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

// Schema pinned: new_events items carry "kind", one of event, possession, location, life, relationship, name, contradiction
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
	if !reflect.DeepEqual(r.Warnings, []string{"방을 나갔다"}) {
		t.Errorf("warnings %v", r.Warnings)
	}
}
