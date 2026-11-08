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
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{Digest: digestOf(d)}, 0)
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
		rr, err := Rerender(ctx, cs, "w", []render.Scene{s}, b, out, Opts{Digest: digestOf("A")}, 0)
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
		rr, err := Rerender(ctx, cs, "w", now, b, out, Opts{Digest: d}, 0)
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
