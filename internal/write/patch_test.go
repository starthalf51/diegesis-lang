package write

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

func TestRerenderPatchesOnlyWithOld(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	bare := func(id, v string) render.Scene { s := mk(id, v); s.Beats, s.Withheld = nil, nil; return s }
	s2, s2b, s4 := bare("s2", ""), bare("s2", "CHANGED"), bare("s4", "")
	seedNovel(t, out, b, s2)
	rc := &fakeLLM{replies: []string{`{"edits":[{"old":"old s2","new":"new s2"}]}`, "전체 산문"}}
	xc := &fakeLLM{replies: []string{`{"items":[],"withheld":[],"other":[]}`}}
	o := Opts{Verifier: "card", Digest: digestOf("## DIGEST-TEXT\n")}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s2b, s4}, b, out, o, 0, map[string]render.Scene{"s2": s2})
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Scenes) != 2 || !rr.Scenes[0].Pass || !rr.Scenes[1].Pass {
		t.Fatalf("%+v", rr.Scenes)
	}
	if len(rc.prompts) != 2 || !strings.Contains(rc.prompts[0], "변경 내역") || !strings.Contains(rc.prompts[0], "DIGEST-TEXT") {
		t.Fatalf("scene 1 must be patched with diff+digest: %q", rc.prompts)
	}
	if strings.Contains(rc.prompts[1], "변경 내역") {
		t.Errorf("new scene must be fully rendered")
	}
	if p, _ := os.ReadFile(filepath.Join(out, "novel", "s2."+b.ID+".md")); strings.TrimSpace(string(p)) != "new s2" {
		t.Errorf("patched prose %q", p)
	}
	// without old scenes: full render for the changed one
	rc2 := &fakeLLM{replies: []string{"전체"}}
	seedNovel(t, out, b, s2)
	if _, err := Rerender(ctx, render.Completers{Render: rc2, Extract: xc}, "w", []render.Scene{s2b}, b, out, o, 0, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rc2.prompts[0], "변경 내역") {
		t.Errorf("nil old must full-render")
	}
}

// Disk prose that does not correspond to old (stale hash) must be rendered in full, not patched.
func TestRerenderStaleProseNotPatched(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	bare := func(id, v string) render.Scene { s := mk(id, v); s.Beats, s.Withheld = nil, nil; return s }
	older, s2, s2b := bare("s2", "OLDER"), bare("s2", ""), bare("s2", "CHANGED")
	seedNovel(t, out, b, older) // prose on disk is from `older`, not from old=s2
	rc := &fakeLLM{replies: []string{"전체"}}
	xc := &fakeLLM{replies: []string{`{"items":[],"withheld":[],"other":[]}`}}
	o := Opts{Verifier: "card"}
	if _, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s2b}, b, out, o, 0, map[string]render.Scene{"s2": s2}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rc.prompts[0], "변경 내역") {
		t.Errorf("stale prose was patched")
	}
}

func TestRerenderMarksPatched(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	bare := func(v string) render.Scene { s := mk("s2", v); s.Beats, s.Withheld = nil, nil; return s }
	s2, s2b := bare(""), bare("CHANGED")
	seedNovel(t, out, b, s2)
	rc := &fakeLLM{replies: []string{`{"edits":[{"old":"old s2","new":"new s2"}]}`}}
	xc := &fakeLLM{replies: []string{`{"items":[],"withheld":[],"other":[]}`}}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s2b}, b, out, Opts{Verifier: "card"}, 0, map[string]render.Scene{"s2": s2})
	if err != nil || !rr.Scenes[0].Patched {
		t.Fatalf("%v %+v", err, rr.Scenes)
	}
	if rep, _ := os.ReadFile(filepath.Join(out, "novel", "s2."+b.ID+".report.json")); !strings.Contains(string(rep), `"patched": true`) {
		t.Errorf("report.json lacks patched: %s", rep)
	}
}
