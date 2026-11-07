package write

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diegesis/internal/render"
)

func scene3(t *testing.T) (string, map[string]string) {
	dir, files := story(t)
	for _, id := range []string{"s2", "s3"} {
		files["ep01/"+id+".dieg.md"] = strings.Replace(oldScene, `"s1"`, `"`+id+`"`, 1)
	}
	if err := WriteFiles(dir, files); err != nil {
		t.Fatal(err)
	}
	return dir, files
}

func TestChangedScenes(t *testing.T) {
	c, a, r := ChangedScenes(map[string]string{"s1": "h1", "s2": "h2", "s3": "h3"}, map[string]string{"s1": "h1", "s2": "x", "s4": "h4"})
	if !reflect.DeepEqual(c, []string{"s2"}) || !reflect.DeepEqual(a, []string{"s4"}) || !reflect.DeepEqual(r, []string{"s3"}) {
		t.Errorf("%v %v %v", c, a, r)
	}
}

func TestReviseKeepsUntouchedFiles(t *testing.T) {
	dir, files := scene3(t)
	s2 := "---\nmodule: t.scenes\nimport: [t.cast]\nscene: \"s2\"\n---\n\n```sl\nenter a\nNEW-S2\n```\n"
	f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\n" + s2 + End}}
	var seen string
	chk := func(_ context.Context, d string) ([]Diag, error) {
		b, _ := os.ReadFile(filepath.Join(d, "ep01", "s3.dieg.md"))
		seen = string(b)
		return nil, nil
	}
	res, err := ReviseLoop(ctx, f, chk, dir, "범인을 바꿔라", testRefs(t), Opts{Rounds: 2, Out: filepath.Dir(dir)})
	if err != nil || !res.Clean || res.Rounds != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if seen != files["ep01/s3.dieg.md"] {
		t.Error("checker did not see untouched s3")
	}
	for _, p := range []string{"cast.dieg.md", "ep01/s1.dieg.md", "ep01/s3.dieg.md"} {
		got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if string(got) != files[p] {
			t.Errorf("%s changed", p)
		}
	}
	got, _ := os.ReadFile(filepath.Join(dir, "ep01", "s2.dieg.md"))
	if !strings.Contains(string(got), "NEW-S2") {
		t.Error("s2 not replaced")
	}
	if p := f.prompts[0]; !strings.Contains(p, "범인을 바꿔라") || !strings.Contains(p, "=== FILE: ep01/s3.dieg.md ===") {
		t.Error("prompt lacks request / whole bundle")
	}
}

func TestReviseDelete(t *testing.T) {
	dir, files := scene3(t)
	f := &fakeLLM{replies: []string{
		"=== DELETE: ../x ===\n" + End,
		"=== DELETE: ep01/s3.dieg.md ===\n" + End,
	}}
	chk, _ := checks(nil)
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 3, Out: filepath.Dir(dir)})
	if err != nil || !res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ep01", "s3.dieg.md")); err == nil {
		t.Error("s3 not deleted")
	}
	delete(files, "ep01/s3.dieg.md")
	unchanged(t, dir, files)
}

func TestReviseUncleanLeavesStory(t *testing.T) {
	dir, files := scene3(t)
	f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\nBROKEN\n" + End}}
	chk, _ := checks([]Diag{{File: "ep01/s2.dieg.md", Severity: "error", Rule: "x", Msg: "bad"}})
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 2, Out: filepath.Dir(dir)})
	if err != nil || res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	unchanged(t, dir, files)
	if !strings.Contains(f.prompts[1], "ep01/s2.dieg.md:0:0: error x: bad") || !strings.Contains(f.prompts[1], "BROKEN") {
		t.Error("fix prompt lacks diag / candidate")
	}
}

// Review Focus 2: only changed and added scenes are rendered; removed scenes lose their files.
func TestReviseRendersOnlyChanged(t *testing.T) {
	s, err := render.LoadScene("../render/testdata/scenes/chibi-jianggan.json")
	if err != nil {
		t.Fatal(err)
	}
	bs, err := render.LoadBackends("../../styles")
	if err != nil || len(bs) == 0 {
		t.Fatal(err)
	}
	b := bs[0]
	mk := func(id string, beat string) render.Scene {
		x := s
		x.ID = id
		x.Title = id
		x.Place += beat
		return x
	}
	s1, s2, s2b, s4 := mk("s1", ""), mk("s2", ""), mk("s2", "CHANGED"), mk("s4", "")
	before, err := SceneHashes([]render.Scene{s1, s2, mk("s3", "")}, b)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	os.MkdirAll(filepath.Join(out, "novel"), 0o755)
	for _, n := range []string{"s1", "s2", "s3"} {
		os.WriteFile(filepath.Join(out, "novel", n+"."+b.ID+".md"), []byte("old "+n), 0o644)
		os.WriteFile(filepath.Join(out, "novel", n+"."+b.ID+".report.json"), []byte("{}"), 0o644)
	}
	all := []render.Scene{s1, s2b, s4}
	rc := &fakeLLM{replies: []string{"산문"}}
	xc := &fakeLLM{replies: []string{"{}"}}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", all, before, b, out, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rr.Changed, []string{"s2"}) || !reflect.DeepEqual(rr.Added, []string{"s4"}) || !reflect.DeepEqual(rr.Removed, []string{"s3"}) {
		t.Fatalf("%+v", rr)
	}
	want := map[string]bool{}
	for _, x := range []render.Scene{s2b, s4} {
		_, u, _ := b.Prompt(x)
		want[u] = true
	}
	_, u1, _ := b.Prompt(s1)
	seen := map[string]bool{}
	for _, p := range rc.prompts {
		seen[p] = true
		if p == u1 {
			t.Error("unchanged s1 was re-rendered")
		}
	}
	for p := range want {
		if !seen[p] {
			t.Error("a changed/added scene was not rendered")
		}
	}
	for _, n := range []string{"s3." + b.ID + ".md", "s3." + b.ID + ".report.json"} {
		if _, err := os.Stat(filepath.Join(out, "novel", n)); err == nil {
			t.Error(n, "left behind")
		}
	}
	if got, _ := os.ReadFile(filepath.Join(out, "novel", "s1."+b.ID+".md")); string(got) != "old s1" {
		t.Error("s1 prose touched")
	}
	if nv, _ := os.ReadFile(filepath.Join(out, "novel.md")); strings.Contains(string(nv), "old s3") || !strings.Contains(string(nv), "old s1") {
		t.Errorf("novel.md %q", nv)
	}
}
