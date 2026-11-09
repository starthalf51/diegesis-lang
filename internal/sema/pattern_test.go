package sema

import (
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/ir"
)

// `pattern <id> [at <scene>]`(최상위·장면 안)와 `fact … tag <name>`(R3)이 IR 행이 된다.
func TestPatternDeclAndFactTag(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
fact who "W" true tag identity
pattern chekhov-gun
pattern recognition at 1
#! allow pattern-missing: "fixture"
pattern red-herring
scene 1
  enter a
  pattern dramatic-irony
`)
	noErrors(t, diags)
	want := []ir.PatternDecl{
		{ID: "t/t.s#chekhov-gun", Pattern: "chekhov-gun"},
		{ID: "t/t.s/1#recognition", Pattern: "recognition", SceneID: "t/t.s/1"},
		{ID: "t/t.s#red-herring", Pattern: "red-herring"},
		{ID: "t/t.s/1#dramatic-irony", Pattern: "dramatic-irony", SceneID: "t/t.s/1"},
	}
	if len(w.PatternDecls) != len(want) {
		t.Fatalf("pattern decls = %+v", w.PatternDecls)
	}
	for i, d := range w.PatternDecls {
		d.SrcID = 0
		if d != want[i] {
			t.Errorf("decl %d = %+v, want %+v", i, d, want[i])
		}
	}
	if len(w.Waivers) != 1 || w.Waivers[0].TargetKind != "pattern" || w.Waivers[0].TargetID != "t/t.s#red-herring" {
		t.Errorf("waivers = %+v", w.Waivers)
	}
	for _, f := range w.Facts {
		if f.ID == "t/t.s/who" && f.Tag != "identity" {
			t.Errorf("fact who tag = %q", f.Tag)
		}
	}
	if _, diags := build(t, "module t.s\nimport t.cast\ncharacter x \"X\" tag identity\n"); len(diags) == 0 {
		t.Error("tag on a character should be an error")
	}
}
