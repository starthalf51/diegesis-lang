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
	chk, _ := checks(nil, []Diag{{File: "ep01/s2.dieg.md", Severity: "error", Rule: "x", Msg: "bad"}})
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 2, Out: filepath.Dir(dir)})
	if err != nil || res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	unchanged(t, dir, files)
	if !strings.Contains(f.prompts[1], "ep01/s2.dieg.md:0:0: error x: bad") || !strings.Contains(f.prompts[1], "BROKEN") {
		t.Error("fix prompt lacks diag / candidate")
	}
}

// rerenderFixture: a backend, a scene maker (variant != "" changes the render request) and a novel dir whose prose
// and hashes were made for the given scenes.
func rerenderFixture(t *testing.T) (render.Backend, func(id, variant string) render.Scene, string) {
	s, err := render.LoadScene("../render/testdata/scenes/chibi-jianggan.json")
	if err != nil {
		t.Fatal(err)
	}
	bs, err := render.LoadBackends("../../styles")
	if err != nil || len(bs) == 0 {
		t.Fatal(err)
	}
	b := bs[0]
	mk := func(id, variant string) render.Scene {
		x := s
		x.ID, x.Title = id, id
		x.Place += variant
		return x
	}
	return b, mk, t.TempDir()
}

func seedNovel(t *testing.T, out string, b render.Backend, old ...render.Scene) {
	hs, err := SceneHashes(old, b)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(out, "novel"), 0o755)
	for _, s := range old {
		base := filepath.Join(out, "novel", s.ID+"."+b.ID)
		os.WriteFile(base+".md", []byte("old "+s.ID), 0o644)
		os.WriteFile(base+".report.json", []byte("{}"), 0o644)
		os.WriteFile(base+".hash", []byte(hs[s.ID]+"\n"), 0o644)
	}
}

// Review Focus 2: only changed and added scenes are rendered; removed scenes lose their files.
func TestReviseRendersOnlyChanged(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s1, s2, s2b, s4 := mk("s1", ""), mk("s2", ""), mk("s2", "CHANGED"), mk("s4", "")
	seedNovel(t, out, b, s1, s2, mk("s3", ""))
	rc := &fakeLLM{replies: []string{"산문"}}
	xc := &fakeLLM{replies: []string{"{}"}}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s1, s2b, s4}, b, out, Opts{}, 0, nil)
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
	for _, e := range []string{".md", ".report.json", ".hash"} {
		if _, err := os.Stat(filepath.Join(out, "novel", "s3."+b.ID+e)); err == nil {
			t.Error("s3", e, "left behind")
		}
	}
	if got, _ := os.ReadFile(filepath.Join(out, "novel", "s1."+b.ID+".md")); string(got) != "old s1" {
		t.Error("s1 prose touched")
	}
	if nv, _ := os.ReadFile(filepath.Join(out, "novel.md")); strings.Contains(string(nv), "old s3") || !strings.Contains(string(nv), "old s1") {
		t.Errorf("novel.md %q", nv)
	}
}

// A --scenes cap (or a --no-render revise) leaves stale prose; the next run renders the backlog, then nothing.
func TestRerenderPicksUpBacklog(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	seedNovel(t, out, b, mk("s1", ""), mk("s2", ""), mk("s3", ""))
	now := []render.Scene{mk("s1", "X"), mk("s2", "X"), mk("s3", "X")}
	run := func(limit int) RerenderResult {
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{}, limit, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	if rr := run(1); len(rr.Changed) != 3 || len(rr.Scenes) != 1 || rr.Scenes[0].ID != "s1" {
		t.Fatalf("first run %+v", rr)
	}
	if rr := run(0); len(rr.Changed) != 2 || len(rr.Scenes) != 2 {
		t.Fatalf("second run %+v", rr)
	}
	if rr := run(0); len(rr.Changed)+len(rr.Added) != 0 || len(rr.Scenes) != 0 {
		t.Fatalf("third run %+v", rr)
	}
}

func TestSwapDir(t *testing.T) {
	dir, files := scene3(t)
	if err := swapDir(dir, map[string]string{"cast.dieg.md": "NEW\n"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "cast.dieg.md")); string(got) != "NEW\n" {
		t.Error("not swapped")
	}
	for _, x := range []string{".new", ".old"} {
		if _, err := os.Stat(dir + x); err == nil {
			t.Error(x, "left behind")
		}
	}
	// second rename fails: the old dir comes back
	dir, files = scene3(t)
	n := 0
	rename = func(a, b string) error {
		if n++; n == 2 {
			return os.ErrPermission
		}
		return os.Rename(a, b)
	}
	defer func() { rename = os.Rename }()
	if err := swapDir(dir, map[string]string{"cast.dieg.md": "NEW\n"}); err == nil {
		t.Fatal("no error")
	}
	unchanged(t, dir, files)
	if _, err := os.Stat(dir + ".new"); err == nil {
		t.Error(".new left behind")
	}
}

// An error after a check ran leaves PG on a rejected candidate; ReviseLoop re-checks story/ itself.
func TestReviseErrorReloadsStory(t *testing.T) {
	dir, files := scene3(t)
	f := &errAfter{ok: "=== FILE: ep01/s2.dieg.md ===\nBROKEN\n" + End}
	var dirs []string
	chk := func(_ context.Context, d string) ([]Diag, error) {
		dirs = append(dirs, d)
		if len(dirs) == 1 { // baseline: the untouched story is clean
			return nil, nil
		}
		return []Diag{{Severity: "error", Rule: "x", Msg: "bad"}}, nil
	}
	_, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 3, Out: filepath.Dir(dir)})
	if err == nil || len(dirs) != 3 || dirs[2] != dir {
		t.Fatalf("err=%v checks=%v", err, dirs)
	}
	unchanged(t, dir, files)
}

type errAfter struct {
	ok string
	n  int
}

func (e *errAfter) Complete(context.Context, string, string) (render.Result, error) {
	if e.n++; e.n > 1 {
		return render.Result{}, os.ErrClosed
	}
	return render.Result{Text: e.ok, StopReason: "end_turn"}, nil
}

// Fix 2: files that are not .dieg.md survive a revise untouched and are not shown to the model.
func TestReviseKeepsNonDiegFiles(t *testing.T) {
	dir, _ := scene3(t)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("keep me\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ep01", "meta.yaml"), []byte("a: 1\n"), 0o644)
	s2 := "---\nmodule: t.scenes\nimport: [t.cast]\nscene: \"s2\"\n---\n\n```sl\nenter a\nNEW\n```\n"
	f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\n" + s2 + "=== FILE: notes.md ===\nOVERWRITE\n" + End, "=== FILE: ep01/s2.dieg.md ===\n" + s2 + End}}
	chk := func(context.Context, string) ([]Diag, error) { return nil, nil }
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 3, Out: filepath.Dir(dir)})
	if err != nil || !res.Clean {
		t.Fatalf("%+v %v", res, err)
	}
	for p, want := range map[string]string{"notes.md": "keep me\n", "ep01/meta.yaml": "a: 1\n"} {
		if got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))); string(got) != want {
			t.Errorf("%s = %q", p, got)
		}
	}
	if strings.Contains(f.prompts[0], "keep me") || strings.Contains(f.prompts[0], "a: 1") {
		t.Error("non-dieg file shown to the model")
	}
	if res.Rounds != 2 {
		t.Errorf("model overwrite of notes.md should be a bundle error; rounds=%d", res.Rounds)
	}
}

// Minor: a new path equal to an existing one up to case is a bundle error.
func TestReviseRejectsCaseCollision(t *testing.T) {
	dir, files := scene3(t)
	f := &fakeLLM{replies: []string{"=== FILE: EP01/S2.dieg.md ===\nx\n" + End}}
	chk := func(context.Context, string) ([]Diag, error) { return nil, nil }
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 1, Out: filepath.Dir(dir)})
	if err != nil || res.Clean {
		t.Fatalf("%+v %v", res, err)
	}
	unchanged(t, dir, files)
}

// Minor: revision k never overwrites revision k-1's rounds; labels carry the ordinal.
func TestReviseRevOrdinal(t *testing.T) {
	dir, _ := scene3(t)
	out := filepath.Dir(dir)
	chk := func(context.Context, string) ([]Diag, error) { return nil, nil }
	for i := 1; i <= 2; i++ {
		s2 := "---\nmodule: t.scenes\nimport: [t.cast]\nscene: \"s2\"\n---\n\n```sl\nenter a\nV" + string(rune('0'+i)) + "\n```\n"
		f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\n" + s2 + End}}
		if _, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 1, Out: out}); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"rev1-r1.raw.txt", "rev2-r1.raw.txt"} {
		if _, err := os.Stat(filepath.Join(out, "rounds", p)); err != nil {
			t.Error(err)
		}
	}
}

func TestSwapDirDoubleFailureNamesOld(t *testing.T) {
	d := filepath.Join(t.TempDir(), "story")
	WriteFiles(d, map[string]string{"a.dieg.md": "x"})
	n := 0
	rename = func(a, b string) error {
		if n++; n > 1 {
			return os.ErrPermission
		}
		return os.Rename(a, b)
	}
	defer func() { rename = os.Rename }()
	err := swapDir(d, map[string]string{"b.dieg.md": "y"})
	if err == nil || !strings.Contains(err.Error(), "previous story kept at "+d+".old") {
		t.Fatalf("%v", err)
	}
}

// A scene rendered by the removed layered renderer carries a "layered:" hash; it never matches, so it is stale.
func TestLayeredHashIsStale(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	seedNovel(t, out, b, mk("s1", ""), mk("s2", ""))
	now := []render.Scene{mk("s1", ""), mk("s2", "")}
	run := func() RerenderResult {
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{}, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	if rr := run(); len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("fresh hashes %+v", rr)
	}
	for _, s := range now {
		p := filepath.Join(out, "novel", fileID(s.ID)+"."+b.ID+".hash")
		h, _ := os.ReadFile(p)
		if err := os.WriteFile(p, append([]byte("layered:"), h...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if rr := run(); len(rr.Changed) != 2 {
		t.Fatalf("layered hashes %+v", rr)
	}
	if rr := run(); len(rr.Changed) != 0 {
		t.Fatalf("after re-render %+v", rr)
	}
}

// A warn the untouched story already has does not block a revision and is not put into the fix prompt.
func TestReviseIgnoresPreexistingWarn(t *testing.T) {
	dir, _ := scene3(t)
	old := Diag{File: "cast.dieg.md", Line: 3, Severity: "warn", Rule: "relative-time-fact", Msg: "old fact"}
	moved := old
	moved.Line = 9 // same warn, line moved by the edit
	f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\nNEW\n" + End}}
	chk, _ := checks([]Diag{old}, []Diag{moved})
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 3, Out: filepath.Dir(dir)})
	if err != nil || !res.Clean || res.Rounds != 1 || len(f.prompts) != 1 {
		t.Fatalf("%+v %v prompts=%d", res, err, len(f.prompts))
	}
}

// A warn the revision introduces still blocks and is asked to be fixed; the old one is not.
func TestReviseBlocksOnNewWarn(t *testing.T) {
	dir, _ := scene3(t)
	old := Diag{File: "cast.dieg.md", Severity: "warn", Rule: "relative-time-fact", Msg: "old fact"}
	nw := Diag{File: "ep01/s2.dieg.md", Severity: "warn", Rule: "retell-known", Msg: "brand new"}
	f := &fakeLLM{replies: []string{"=== FILE: ep01/s2.dieg.md ===\nNEW\n" + End, "=== FILE: ep01/s2.dieg.md ===\nNEW2\n" + End}}
	chk, _ := checks([]Diag{old}, []Diag{old, nw}, []Diag{old})
	res, err := ReviseLoop(ctx, f, chk, dir, "req", testRefs(t), Opts{Rounds: 3, Out: filepath.Dir(dir)})
	if err != nil || !res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if !strings.Contains(f.prompts[1], "brand new") || strings.Contains(f.prompts[1], "old fact") {
		t.Error("fix prompt should carry only the new warn")
	}
}

// Only a change to a scene's own request stales it: a different Previously recap (an earlier scene's fact text) does not.
func TestStaleIgnoresPreviously(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s1, s2 := mk("s1", ""), mk("s2", "")
	s1.Previously, s2.Previously = "old recap", "old recap 2"
	seedNovel(t, out, b, s1, s2)
	run := func(a, c render.Scene) RerenderResult {
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", []render.Scene{a, c}, b, out, Opts{}, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	n1, n2 := s1, s2
	n2.Previously = "a different recap"
	if rr := run(n1, n2); len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("recap-only change staled %+v", rr)
	}
	n2 = mk("s2", "-own change")
	n2.Previously = s2.Previously
	if rr := run(n1, n2); len(rr.Changed) != 1 || rr.Changed[0] != "s2" {
		t.Fatalf("own change not stale: %+v", rr)
	}
}

// A legacy .hash (full RenderHash, with Previously) of an unchanged scene is not stale.
func TestLegacyFullHashNotStale(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s := mk("s1", "")
	s.Previously = "recap"
	seedNovel(t, out, b, s)
	full, _ := render.RenderHash(s, b)
	os.WriteFile(filepath.Join(out, "novel", "s1."+b.ID+".hash"), []byte(full+"\n"), 0o644)
	cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
	rr, err := Rerender(ctx, cs, "w", []render.Scene{s}, b, out, Opts{}, 0, nil)
	if err != nil || len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("%+v %v", rr, err)
	}
}
