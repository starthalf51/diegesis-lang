package main

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
	halluc.NewEvents = []string{"주유가 장간을 죽인다"}
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
		NewEvents:   []string{"x", "x"},
	}
	r := Diff(s, e)
	if want := []string{"unknown-fact:yy", "unknown-fact:zz", "x"}; !reflect.DeepEqual(r.Hallucinated, want) {
		t.Fatalf("halluc %v", r.Hallucinated)
	}
	if !reflect.DeepEqual(r.Leaked, []string{"w"}) || len(r.Missing) != 0 {
		t.Fatalf("%+v", r)
	}
}
