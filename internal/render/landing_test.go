package render

import (
	"strings"
	"testing"
)

const landing = "End the scene on its last act, told as the point-of-view character's decision or action; add no action, movement or speech after it."

func TestPromptLandingRule(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	plain, _, _ := BuildPrompt(s, VariantIROnly, "en", nil)
	if strings.Contains(plain, landing) {
		t.Error("no design: landing rule must be absent")
	}
	for _, f := range []func(*Scene){
		func(s *Scene) { s.Goal = Goal{Who: s.Cast[0].ID, Text: "x"} },
		func(s *Scene) { s.Turn = Turn{From: "a", To: "b"} },
		func(s *Scene) { s.Tempo = "scene" },
	} {
		c := s
		f(&c)
		sys, _, _ := BuildPrompt(c, VariantIROnly, "en", nil)
		if !strings.Contains(sys, landing) {
			t.Error("design: landing rule missing")
		}
		if strings.Replace(sys, "- "+landing+"\n", "", 1) != plain {
			t.Error("design adds only the landing line")
		}
	}
}
