package write

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"diegesis/internal/render"
)

const oldScene = "---\nmodule: t.scenes\nimport: [t.cast]\nscene: \"s1\"\n---\n\n```sl\nenter a\nOLD-SCENE-BODY\n```\n"

// story writes a one-episode story dir and returns it with its files.
func story(t *testing.T) (string, map[string]string) {
	dir := filepath.Join(t.TempDir(), "story")
	files := map[string]string{"cast.dieg.md": "---\nmodule: t.cast\n---\n\n```sl\nwork t \"T\"\ncharacter a \"A\"\n```\n", "ep01/s1.dieg.md": oldScene}
	if err := WriteFiles(dir, files); err != nil {
		t.Fatal(err)
	}
	return dir, files
}

func unchanged(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil || string(got) != want {
			t.Errorf("%s changed: %q %v", p, got, err)
		}
	}
}

// Review Focus 1: a bundle touching an existing file or a path outside ep02/ is a bundle diagnostic; nothing is written.
func TestContinueRejectsPastEdits(t *testing.T) {
	dir, files := story(t)
	f := &fakeLLM{replies: []string{
		"=== FILE: ep01/s1.dieg.md ===\n# rewritten past\n" + End,
		"=== FILE: cast.dieg.md ===\n# new cast\n" + End,
		"=== FILE: ep03/s2.dieg.md ===\n# wrong episode\n" + End,
	}}
	chk, n := checks(nil)
	o := Opts{Rounds: 3, Out: filepath.Dir(dir), Digest: "DIGEST"}
	res, err := ContinueLoop(ctx, f, chk, dir, 2, "brief", testRefs(t), o)
	if err != nil || res.Clean || res.Rounds != 3 || *n != 0 {
		t.Fatalf("%+v %v checks=%d", res, err, *n)
	}
	for i, p := range []string{"ep01/s1.dieg.md", "cast.dieg.md", "ep03/s2.dieg.md"} {
		if i+1 < len(f.prompts) && !strings.Contains(f.prompts[i+1], p) {
			t.Errorf("round %d prompt does not name %s", i+2, p)
		}
	}
	if len(res.Diags) != 1 || res.Diags[0].Rule != "bundle" {
		t.Errorf("diags %v", res.Diags)
	}
	unchanged(t, dir, files)
	if _, err := os.Stat(filepath.Join(dir, "ep02")); err == nil {
		t.Error("ep02 left behind")
	}
}

func TestContinueOnlyNewScenesRendered(t *testing.T) {
	dir, files := story(t)
	ep := "=== FILE: ep02/s2.dieg.md ===\n---\nmodule: t.scenes\nimport: [t.cast]\nscene: \"s2\"\n---\n\n```sl\nenter a\n```\n" + End
	f := &fakeLLM{replies: []string{ep, ep}}
	var seen []bool
	chk := func(_ context.Context, d string) ([]Diag, error) {
		_, e1 := os.Stat(filepath.Join(d, "ep01", "s1.dieg.md"))
		_, e2 := os.Stat(filepath.Join(d, "ep02", "s2.dieg.md"))
		seen = append(seen, e1 == nil && e2 == nil)
		if len(seen) == 1 {
			return []Diag{{File: "ep02/s2.dieg.md", Line: 1, Severity: "error", Rule: "x", Msg: "bad"}}, nil
		}
		return nil, nil
	}
	o := Opts{Rounds: 3, Out: filepath.Dir(dir), Digest: "DIGEST-MARK"}
	res, err := ContinueLoop(ctx, f, chk, dir, 2, "서진이 종탑에 오른다", testRefs(t), o)
	if err != nil || !res.Clean || res.Rounds != 2 || !slices.Equal(seen, []bool{true, true}) {
		t.Fatalf("%+v %v seen=%v", res, err, seen)
	}
	p := f.prompts[0]
	if !strings.Contains(p, "DIGEST-MARK") || !strings.Contains(p, "work t") || !strings.Contains(p, "서진이 종탑에 오른다") || !strings.Contains(p, "ep02/") {
		t.Error("prompt lacks digest / cast / brief / ep02")
	}
	if strings.Contains(p, "OLD-SCENE-BODY") {
		t.Error("prompt carries a past scene's code")
	}
	if !strings.Contains(f.prompts[1], "ep02/s2.dieg.md:1:0: error x: bad") {
		t.Error("fix prompt lacks the diagnostic")
	}
	unchanged(t, dir, files)
	all := []render.Scene{{ID: "s1"}, {ID: "s2"}}
	if got := Only(all, res.Scenes); len(got) != 1 || got[0].ID != "s2" {
		t.Errorf("render targets %v, want [s2]", got)
	}
}

func TestDefaultCodeEffort(t *testing.T) {
	if e := DefaultOpts().CodeEffort; e != "medium" {
		t.Errorf("CodeEffort %q, want medium", e)
	}
}

// ponytail cap: tail lines go from the end first; head survives while it fits.
func TestCapDigest(t *testing.T) {
	head := []string{"# head", "- x [독자 비공개]"}
	var tail []string
	for i := 0; i < 400; i++ {
		tail = append(tail, strings.Repeat("가", 30)+string(rune('a'+i%26)))
	}
	s := capDigest(head, tail)
	if n := len([]rune(s)); n > DigestCap || !strings.HasPrefix(s, "# head\n- x [독자 비공개]\n") || !strings.Contains(s, "생략") {
		t.Fatalf("len %d, head kept=%t", n, strings.HasPrefix(s, "# head"))
	}
	if strings.Contains(s, tail[len(tail)-1]+"\n") && !strings.Contains(s, tail[0]) {
		t.Error("trimmed from the front")
	}
	if s := capDigest(head, tail[:2]); strings.Contains(s, "생략") {
		t.Error("cut without need")
	}
}

func TestDefaultRenderEffortLow(t *testing.T) {
	if e := DefaultOpts().RenderEffort; e != "low" {
		t.Errorf("RenderEffort %q, want low", e)
	}
}
