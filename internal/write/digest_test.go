package write

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diegesis/internal/render"
)

func digestOf(s string) func(context.Context, string) (string, string, error) {
	return func(context.Context, string) (string, string, error) { return s, s, nil }
}

func hashFile(t *testing.T, out string, b render.Backend, id string) string {
	t.Helper()
	h, err := os.ReadFile(filepath.Join(out, "novel", id+"."+b.ID+".hash"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(h))
}

func TestRenderAppendsDigest(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s := mk("s1", "")
	_, plain, err := b.Prompt(s)
	if err != nil {
		t.Fatal(err)
	}
	run := func(o Opts) string {
		rc := &fakeLLM{replies: []string{"산문"}}
		cs := render.Completers{Render: rc, Extract: &fakeLLM{replies: []string{"{}"}}}
		if _, err := Novel(ctx, cs, "w", []render.Scene{s}, b, out, o); err != nil {
			t.Fatal(err)
		}
		return rc.prompts[0]
	}
	if got := run(Opts{}); got != plain {
		t.Errorf("nil Digest changed the prompt")
	}
	if got := run(Opts{Digest: digestOf("## DIGEST-TEXT\n")}); !strings.HasSuffix(got, "## DIGEST-TEXT\n") || !strings.HasPrefix(got, plain) {
		t.Errorf("digest not appended at the end: ...%q", got[max(0, len(got)-80):])
	}
}

func TestDigestChangeMakesStale(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	now := []render.Scene{mk("s1", "")}
	run := func(d string) RerenderResult {
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{Digest: digestOf(d)}, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	if rr := run("A"); len(rr.Added) != 1 {
		t.Fatalf("first %+v", rr)
	}
	if rr := run("A"); len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("same digest %+v", rr)
	}
	if rr := run("B"); len(rr.Changed) != 1 || len(rr.Scenes) != 1 {
		t.Fatalf("new digest %+v", rr)
	}
}

func TestLegacyHashNotStale(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	seedNovel(t, out, b, mk("s1", ""))
	run := func(s render.Scene) RerenderResult {
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", []render.Scene{s}, b, out, Opts{Digest: digestOf("A")}, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	if rr := run(mk("s1", "")); len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("legacy hash went stale %+v", rr)
	}
	if h := hashFile(t, out, b, "s1"); strings.Contains(h, " ") {
		t.Fatalf("untouched legacy hash rewritten: %q", h)
	}
	if rr := run(mk("s1", "X")); len(rr.Changed) != 1 {
		t.Fatalf("ir change %+v", rr)
	}
	if h := hashFile(t, out, b, "s1"); len(strings.Fields(h)) != 2 {
		t.Errorf("hash after re-render = %q, want two columns", h)
	}
}

// Only the state part of the digest decides staleness; the text (acts, plants) may change freely.
func TestStaleOnlyOnStatePart(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	now := []render.Scene{mk("s1", "")}
	run := func(text, state string) RerenderResult {
		d := func(context.Context, string) (string, string, error) { return text, state, nil }
		cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{Digest: d}, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rr
	}
	run("acts v1", "STATE")
	if rr := run("acts v2", "STATE"); len(rr.Changed)+len(rr.Added) != 0 {
		t.Fatalf("text-only change went stale %+v", rr)
	}
	if rr := run("acts v2", "STATE2"); len(rr.Changed) != 1 {
		t.Fatalf("state change not stale %+v", rr)
	}
}

// A legacy hash is stamped with the state digest; a later digest change is then state-stale (listed in StateStale),
// and a scene in Opts.Skip is never rendered.
func TestStampThenStateStaleAndSkip(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s1, s2 := mk("s1", ""), mk("s2", "")
	seedNovel(t, out, b, s1)
	seedNovel(t, out, b, s2)
	if n, _, err := Stamp(ctx, []render.Scene{s1, s2}, b, out, Opts{Digest: digestOf("A")}); err != nil || n != 2 {
		t.Fatalf("stamp %d %v", n, err)
	}
	cs := render.Completers{Render: &fakeLLM{replies: []string{"산문"}}, Extract: &fakeLLM{replies: []string{"{}"}}}
	rr, err := Rerender(ctx, cs, "w", []render.Scene{s1, s2}, b, out, Opts{Digest: digestOf("B"), Skip: []string{"s2"}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.StateStale) != 2 || len(rr.Changed) != 1 || rr.Changed[0] != "s1" || len(rr.Skipped) != 1 || len(rr.Scenes) != 1 {
		t.Fatalf("%+v", rr)
	}
}

func stateStaleSetup(t *testing.T, a string) (render.Backend, render.Scene, string) {
	b, mk, out := rerenderFixture(t)
	s := mk("s1", "")
	s.Beats, s.Withheld = nil, nil
	seedNovel(t, out, b, s)
	if n, _, err := Stamp(ctx, []render.Scene{s}, b, out, Opts{Digest: digestOf(a)}); err != nil || n != 1 {
		t.Fatalf("stamp %d %v", n, err)
	}
	return b, s, out
}

const (
	stateA = "### 상태\n- 상자 at = 홀\n- 문 = 닫힘\n"
	stateB = "### 상태\n- 상자 at = 마당\n- 문 = 닫힘\n"
)

// Only the PG state changed (old IR == new IR): the patch prompt must name the changed state lines, not an empty diff.
func TestStateStalePatchCarriesStateLines(t *testing.T) {
	b, s, out := stateStaleSetup(t, stateA)
	rc := &fakeLLM{replies: []string{`{"edits":[{"old":"old s1","new":"new s1"}]}`}}
	xc := &fakeLLM{replies: []string{`{"items":[],"withheld":[],"other":[]}`}}
	o := Opts{Verifier: "card", Digest: digestOf(stateB)}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s}, b, out, o, 0, map[string]render.Scene{"s1": s})
	if err != nil || len(rr.StateStale) != 1 || len(rr.Scenes) != 1 || !rr.Scenes[0].Patched {
		t.Fatalf("%+v %v", rr, err)
	}
	_, diff, ok := strings.Cut(rc.prompts[0], "## 변경 내역")
	if !ok || !strings.Contains(diff, "상자 at = 홀") || !strings.Contains(diff, "상자 at = 마당") {
		t.Fatalf("diff lacks the changed state lines: %q", diff)
	}
	if strings.Contains(diff, "문 = 닫힘") {
		t.Errorf("unchanged state line in the diff: %q", diff)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "novel", "s1."+b.ID+".state.txt")); string(got) != stateB {
		t.Errorf("state file after patch = %q", got)
	}
}

// A scene stamped without a stored state text (legacy) cannot be diffed: it is rendered in full.
func TestStateStaleLegacyFullRender(t *testing.T) {
	b, s, out := stateStaleSetup(t, stateA)
	sf := filepath.Join(out, "novel", "s1."+b.ID+".state.txt")
	if _, err := os.Stat(sf); err != nil {
		t.Fatalf("stamp wrote no state file: %v", err)
	}
	os.Remove(sf)
	rc := &fakeLLM{replies: []string{"전체 산문"}}
	xc := &fakeLLM{replies: []string{`{"items":[],"withheld":[],"other":[]}`}}
	o := Opts{Verifier: "card", Digest: digestOf(stateB)}
	rr, err := Rerender(ctx, render.Completers{Render: rc, Extract: xc}, "w", []render.Scene{s}, b, out, o, 0, map[string]render.Scene{"s1": s})
	if err != nil || len(rr.StateStale) != 1 || len(rr.Scenes) != 1 || rr.Scenes[0].Patched {
		t.Fatalf("%+v %v", rr, err)
	}
	if strings.Contains(rc.prompts[0], "변경 내역") {
		t.Errorf("legacy state-stale scene was patched")
	}
}

// Stamp rewrites only hashes it can identify as this scene's (generation or legacy full hash); others are skipped.
func TestStampSkipsUnknownHash(t *testing.T) {
	b, mk, out := rerenderFixture(t)
	s1, s2, s3 := mk("s1", ""), mk("s2", ""), mk("s3", "")
	seedNovel(t, out, b, s1, s2, s3)
	p2 := filepath.Join(out, "novel", "s2."+b.ID+".hash")
	os.WriteFile(p2, []byte("deadbeef\n"), 0o644)
	full, _ := render.RenderHash(s3, b) // legacy full hash is accepted
	os.WriteFile(filepath.Join(out, "novel", "s3."+b.ID+".hash"), []byte(full+"\n"), 0o644)
	n, skipped, err := Stamp(ctx, []render.Scene{s1, s2, s3}, b, out, Opts{Digest: digestOf("A")})
	if err != nil || n != 2 || skipped != 1 {
		t.Fatalf("stamped=%d skipped=%d %v", n, skipped, err)
	}
	if h, _ := os.ReadFile(p2); strings.TrimSpace(string(h)) != "deadbeef" {
		t.Errorf("mismatched hash rewritten: %q", h)
	}
}
