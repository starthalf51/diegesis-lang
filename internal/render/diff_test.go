package render

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	perfect := extractedFromScene(s)
	r := Diff(s, perfect)
	if len(r.Missing)+len(r.Hallucinated)+len(r.Leaked) != 0 {
		t.Fatalf("%+v", r)
	}
	if r.Missing == nil || r.Hallucinated == nil || r.Leaked == nil {
		t.Fatalf("slices must be non-nil: %+v", r)
	}
	lossy := perfect
	lossy.Acts = lossy.Acts[1:]
	if r := Diff(s, lossy); len(r.Missing) != 1 {
		t.Fatalf("%+v", r)
	}
	leaky := perfect
	leaky.FactsStated = append(append([]string{}, leaky.FactsStated...), "letter_forged")
	if r := Diff(s, leaky); !reflect.DeepEqual(r.Leaked, []string{"letter_forged"}) {
		t.Fatalf("%+v", r)
	}
	halluc := perfect
	halluc.NewEvents = []NewEvent{{Text: "주유가 장간을 죽인다"}}
	if r := Diff(s, halluc); len(r.Hallucinated) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffIgnoresVerbSynonyms(t *testing.T) {
	s := Scene{
		Facts: []Fact{{ID: "f1"}},
		Beats: []Beat{{Acts: []Act{{Actor: "a", Verb: "hails", Targets: []string{"b"}, Fact: "f1"}}}},
	}
	e := Extracted{Acts: []ExtractedAct{{Actor: "a", Verb: "says", Fact: "f1"}}, FactsStated: []string{"f1"}}
	if r := Diff(s, e); len(r.Missing) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffKeysAndFailClosed(t *testing.T) {
	s := Scene{
		Facts:    []Fact{{ID: "f1"}, {ID: "w"}},
		Withheld: []string{"w"},
		Beats: []Beat{{Acts: []Act{
			{Actor: "a", Verb: "reads", Fact: "f1"},
			{Verb: "enter", Targets: []string{"a"}},
		}}},
	}
	// wrong class and unknown verb both stay Missing
	for _, verb := range []string{"says", "zorp"} {
		e := Extracted{Acts: []ExtractedAct{{Actor: "a", Verb: verb, Fact: "f1"}}, FactsStated: []string{"f1"}}
		r := Diff(s, e)
		if !reflect.DeepEqual(r.Missing, []string{"a|read|f1"}) {
			t.Fatalf("%s: %+v", verb, r)
		}
	}
	e := Extracted{
		Acts:        []ExtractedAct{{Actor: "a", Verb: "reads", Fact: "f1"}, {Actor: "a", Verb: "says", Fact: "zz"}, {Actor: "a", Verb: "says", Fact: "w"}},
		FactsStated: []string{"f1", "zz", "yy", "w", "w"},
		NewEvents:   []NewEvent{{Text: "x"}, {Text: "x"}},
	}
	r := Diff(s, e)
	// a says w is a fact act not in the original, so it is a leak and an excess act (R4).
	if want := []string{"extra-act:a|assert|w", "unknown-fact:yy", "unknown-fact:zz", "x"}; !reflect.DeepEqual(r.Hallucinated, want) {
		t.Fatalf("halluc %v", r.Hallucinated)
	}
	if !reflect.DeepEqual(r.Leaked, []string{"w"}) || len(r.Missing) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffMissingIsMultiset(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	e := extractedFromScene(s)
	var acts []ExtractedAct
	for _, a := range e.Acts {
		if !(a.Actor == "jiang_gan" && a.Verb == "tells" && a.Fact == "commanders_traitors") {
			acts = append(acts, a)
		}
	}
	e.Acts = acts // the aside with the same key remains
	r := Diff(s, e)
	if !reflect.DeepEqual(r.Missing, []string{"jiang_gan|assert|commanders_traitors"}) {
		t.Fatalf("%+v", r)
	}
}

func factlessScene() Scene {
	return Scene{
		Cast: []Entity{{ID: "seoyun", Names: map[string][]string{"ko": {"서윤"}}}, {ID: "haeun", Names: map[string][]string{"ko": {"하은"}}}, {ID: "ferry_ticket"}},
		Beats: []Beat{{ID: "b1", Acts: []Act{
			{Actor: "haeun", Verb: "does", Action: "hide", Targets: []string{"ferry_ticket"}},
			{Verb: "exit", Targets: []string{"seoyun", "haeun"}},
		}}},
	}
}

func TestDiffDropsAuthoredFactlessEvent(t *testing.T) {
	e := Extracted{NewEvents: []NewEvent{{Actor: "하은", Verb: "does", Action: "Hide", Targets: []string{"ferry_ticket"}}}}
	if r := Diff(factlessScene(), e); len(r.Hallucinated) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffKeepsDifferentAction(t *testing.T) {
	e := Extracted{NewEvents: []NewEvent{{Actor: "haeun", Verb: "does", Action: "burn", Targets: []string{"ferry_ticket"}}}}
	if r := Diff(factlessScene(), e); len(r.Hallucinated) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffDropsExitAll(t *testing.T) {
	e := Extracted{NewEvents: []NewEvent{{Actor: "haeun", Verb: "exit"}}}
	if r := Diff(factlessScene(), e); len(r.Hallucinated) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffKeepsUnauthoredEvent(t *testing.T) {
	e := Extracted{NewEvents: []NewEvent{{Actor: "seoyun", Verb: "does", Action: "hide", Targets: []string{"ferry_ticket"}}}}
	if r := Diff(factlessScene(), e); len(r.Hallucinated) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffPresenceDirectionAndEmpty(t *testing.T) {
	s := Scene{Cast: []Entity{{ID: "x"}}, Beats: []Beat{{Acts: []Act{{Verb: "enter", Targets: []string{"x"}}}}}}
	if r := Diff(s, Extracted{NewEvents: []NewEvent{{Verb: "exit", Targets: []string{"x"}}}}); len(r.Hallucinated) != 1 {
		t.Fatalf("enter vs exit %+v", r)
	}
	s.Beats[0].Acts[0].Verb = "exit"
	if r := Diff(s, Extracted{NewEvents: []NewEvent{{Verb: "exit", Targets: []string{"x"}}}}); len(r.Hallucinated) != 0 {
		t.Fatalf("exit %+v", r)
	}
	if r := Diff(s, Extracted{NewEvents: []NewEvent{{Verb: "exit"}}}); len(r.Hallucinated) != 1 {
		t.Fatalf("empty %+v", r)
	}
}

func TestDiffOnlyActAndPresenceDrop(t *testing.T) {
	s := Scene{Cast: []Entity{{ID: "a"}, {ID: "sewing_box"}}, Beats: []Beat{{Acts: []Act{
		{Actor: "a", Verb: "says"}, {Actor: "a", Verb: "does", Action: "reach", Targets: []string{"sewing_box"}}}}}}
	for name, c := range map[string]struct {
		e    NewEvent
		want int
	}{
		"assert":      {NewEvent{Actor: "a", Verb: "says", Text: "t"}, 1},
		"bare does":   {NewEvent{Actor: "a", Verb: "does"}, 1},
		"target only": {NewEvent{Actor: "a", Verb: "does", Targets: []string{"sewing_box"}}, 0},
	} {
		if r := Diff(s, Extracted{NewEvents: []NewEvent{c.e}}); len(r.Hallucinated) != c.want {
			t.Errorf("%s %+v", name, r)
		}
	}
}

func TestDiffAmbiguousNameNoMatch(t *testing.T) {
	s := Scene{Cast: []Entity{{ID: "a", Names: map[string][]string{"ko": {"민"}}}, {ID: "b", Names: map[string][]string{"ko": {"민"}}}},
		Beats: []Beat{{Acts: []Act{{Actor: "a", Verb: "does", Action: "hide"}}}}}
	if r := Diff(s, Extracted{NewEvents: []NewEvent{{Actor: "민", Verb: "does", Action: "hide"}}}); len(r.Hallucinated) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDiffDoesActionNeedsTarget(t *testing.T) {
	s := Scene{Cast: []Entity{{ID: "mina"}, {ID: "letter"}, {ID: "body"}},
		Beats: []Beat{{Acts: []Act{{Actor: "mina", Verb: "does", Action: "hide", Targets: []string{"letter"}}}}}}
	for name, c := range map[string]struct {
		t    string
		want int
	}{"other target": {"body", 1}, "same target": {"letter", 0}} {
		e := Extracted{NewEvents: []NewEvent{{Actor: "mina", Verb: "does", Action: "hide", Targets: []string{c.t}}}}
		if r := Diff(s, e); len(r.Hallucinated) != c.want {
			t.Errorf("%s %+v", name, r)
		}
	}
}
