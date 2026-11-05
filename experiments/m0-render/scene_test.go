package main

import (
	"strings"
	"testing"
)

func TestLoadSceneMinimal(t *testing.T) {
	s, err := LoadScene("testdata/scenes/min.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Beats[0].Acts) != 1 || s.Beats[0].Acts[0].Verb != "tells" {
		t.Fatalf("%+v", s.Beats)
	}
	if _, ok := s.Entity("ross"); !ok {
		t.Error("Entity(ross) not found")
	}
	if f, ok := s.FactByID("f1"); !ok || !f.True {
		t.Error("FactByID(f1) wrong")
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]string{
		"bad/speaker-offstage.json": `beats[0].acts[1].actor "ross": not on stage`,
		"bad/unknown-fact.json":     `beats[0].acts[0].fact "nope": unknown fact`,
		"bad/unknown-withheld.json": `withheld[0] "nope": unknown fact`,
		"bad/unknown-verb.json":     `beats[0].acts[0].verb "shouts": unknown verb`,
		"bad/unknown-field.json":    `unknown field`,
	}
	for f, want := range cases {
		_, err := LoadScene("testdata/" + f)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", f, err, want)
		}
	}
}

func TestExitThenSpeakIsOffstage(t *testing.T) {
	_, err := LoadScene("testdata/bad/exit-then-speak.json")
	if err == nil || !strings.Contains(err.Error(), `beats[0].acts[1].actor "ross": not on stage`) {
		t.Fatalf("got %v", err)
	}
}
