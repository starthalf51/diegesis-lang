package main

import (
	"context"
	"github.com/starthalf51/diegesis-lang/internal/render"
	"github.com/starthalf51/diegesis-lang/internal/write"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWriteCheckerRejectsCorpusID(t *testing.T) {
	ds, err := writeChecker([]string{"macbeth"})(context.Background(), "../../corpus/macbeth")
	if err != nil || len(ds) == 0 || ds[len(ds)-1].Rule != "bundle" {
		t.Fatalf("%v %v", ds, err)
	}
}

func TestWriteCheckerRelativePaths(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "scenes"), 0o755)
	os.WriteFile(filepath.Join(dir, "scenes", "s1.dieg.md"), []byte("---\nmodule: x.scenes\n---\n\n```sl\nnonsense here\n```\n"), 0o644)
	ds, err := writeChecker(nil)(context.Background(), dir)
	if err != nil || len(ds) == 0 {
		t.Fatalf("%v %v", ds, err)
	}
	if ds[0].File != "scenes/s1.dieg.md" {
		t.Errorf("file %q, want bundle-relative", ds[0].File)
	}
}

func TestNotUnderCorpus(t *testing.T) {
	root := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	for out, bad := range map[string]bool{"corpus": true, "corpus/x/y": true, "out/corpus": false, "corpusx": false, "out/s": false} {
		if err := notUnderCorpus(out); (err != nil) != bad {
			t.Errorf("%s: err=%v want bad=%t", out, err, bad)
		}
	}
}

func TestBadCodeEffortExits2(t *testing.T) {
	if c := cmdWrite([]string{"idea", "-o", t.TempDir(), "--code-effort", "max"}); c != 2 {
		t.Errorf("write exit %d", c)
	}
	if c := cmdContinue([]string{t.TempDir(), "--ep", "2", "--code-effort", "extreme"}); c != 2 {
		t.Errorf("continue exit %d", c)
	}
}

func TestBadRenderEffortExits2(t *testing.T) {
	for name, c := range map[string]int{
		"write":    cmdWrite([]string{"idea", "-o", t.TempDir(), "--render-effort", "max"}),
		"continue": cmdContinue([]string{t.TempDir(), "--ep", "2", "--render-effort", "max"}),
		"revise":   cmdRevise([]string{t.TempDir(), "x", "--render-effort", "max"}),
		"render":   cmdRender([]string{"w", "s", "--backend", "b", "--render-effort", "max"}),
	} {
		if c != 2 {
			t.Errorf("%s exit %d", name, c)
		}
	}
	o, st, lim := write.DefaultOpts(), "styles", 0
	if _, ok, code := commonFlag([]string{"--render-effort", "low"}, 0, &o, &st, &lim); !ok || code != 0 || o.RenderEffort != "low" {
		t.Errorf("ok=%v code=%d effort=%q", ok, code, o.RenderEffort)
	}
}

func TestWriteRejectsUnknownModel(t *testing.T) {
	if c := cmdWrite([]string{"idea", "-o", t.TempDir(), "--render-model", "nope"}); c != 2 {
		t.Fatalf("exit %d", c)
	}
}

// Fix 1: write -o on an existing serial dir is refused unless forced; nothing is touched.
func TestWriteRefusesExistingStory(t *testing.T) {
	out := t.TempDir()
	ep2 := filepath.Join(out, "story", "ep02", "s1.dieg.md")
	os.MkdirAll(filepath.Dir(ep2), 0o755)
	os.WriteFile(ep2, []byte("x"), 0o644)
	os.WriteFile(filepath.Join(out, "calls.jsonl"), []byte("log"), 0o644)
	o := write.DefaultOpts()
	o.Out = out
	_, err := runWrite("idea", "../../styles", 0, o, false)
	if err == nil || !strings.Contains(err.Error(), "dieg continue") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err=%v", err)
	}
	if b, _ := os.ReadFile(ep2); string(b) != "x" {
		t.Error("story touched")
	}
	if b, _ := os.ReadFile(filepath.Join(out, "calls.jsonl")); string(b) != "log" {
		t.Error("calls.jsonl touched")
	}
}

// Fix 3: after continue, the new episode's scenes and any stale backlog are rendered under one --scenes cap.
func TestRenderContinuedPicksUpBacklog(t *testing.T) {
	s, err := render.LoadScene("../../internal/render/testdata/scenes/chibi-jianggan.json")
	if err != nil {
		t.Fatal(err)
	}
	bs, err := render.LoadBackends("../../styles")
	if err != nil || len(bs) == 0 {
		t.Fatal(err)
	}
	b, out := bs[0], t.TempDir()
	mk := func(id, v string) render.Scene { x := s; x.ID, x.Title, x.Place = id, id, x.Place+v; return x }
	old := []render.Scene{mk("s1", ""), mk("s2", "")}
	hs, _ := write.SceneHashes(old, b)
	os.MkdirAll(filepath.Join(out, "novel"), 0o755)
	for _, x := range old {
		base := filepath.Join(out, "novel", x.ID+"."+b.ID)
		os.WriteFile(base+".md", []byte("old"), 0o644)
		os.WriteFile(base+".hash", []byte(hs[x.ID]+"\n"), 0o644)
	}
	all := []render.Scene{mk("s1", ""), mk("s2", "STALE"), mk("s3", "")} // s2 backlog, s3 new
	cs := render.Completers{Render: fakeC{"산문"}, Extract: fakeC{"{}"}}
	var rr write.RerenderResult
	o := write.DefaultOpts()
	o.Out = out
	o.Skeleton = false
	code, err := renderContinued(context.Background(), cs, "w", all, b, o, write.Result{Scenes: []string{"s3"}}, 0,
		func(_ write.Result, r write.RerenderResult, backlog int, _ []string) {
			rr = r
			if backlog != 1 {
				t.Errorf("backlog %d", backlog)
			}
		})
	if err != nil {
		t.Fatal(code, err)
	}
	if len(rr.Scenes) != 2 {
		t.Fatalf("rendered %d, want s2+s3: %+v", len(rr.Scenes), rr)
	}
}

type fakeC struct{ s string }

func (f fakeC) Complete(context.Context, string, string) (render.Result, error) {
	return render.Result{Text: f.s, StopReason: "end_turn"}, nil
}

// --renderer and --beat-retries were removed with the layered renderer: unknown flags exit 2 before any call.
func TestRendererFlagGone(t *testing.T) {
	// 0 calls: every run opens <dir>/calls.jsonl before its first call, so the dir must stay empty.
	for _, bad := range [][]string{{"--renderer", "single"}, {"--renderer", "layered"}, {"--beat-retries", "1"}} {
		for name, run := range map[string]func(dir string) int{
			"write":    func(d string) int { return cmdWrite(append([]string{"idea", "-o", d}, bad...)) },
			"continue": func(d string) int { return cmdContinue(append([]string{d, "--ep", "2"}, bad...)) },
			"revise":   func(d string) int { return cmdRevise(append([]string{d, "요청"}, bad...)) },
		} {
			dir := t.TempDir()
			if c := run(dir); c != 2 {
				t.Errorf("%s %v exit %d", name, bad, c)
			}
			if es, _ := os.ReadDir(dir); len(es) != 0 {
				t.Errorf("%s %v wrote %v", name, bad, es)
			}
		}
	}
}

func TestVerifierFlag(t *testing.T) {
	if write.DefaultOpts().Verifier != "card" {
		t.Fatal("default verifier must be card")
	}
	for name, run := range map[string]func(dir string) int{
		"write":    func(d string) int { return cmdWrite([]string{"idea", "-o", d, "--verifier", "bogus"}) },
		"continue": func(d string) int { return cmdContinue([]string{d, "--ep", "2", "--verifier", "bogus"}) },
		"revise":   func(d string) int { return cmdRevise([]string{d, "요청", "--verifier", "bogus"}) },
		"render": func(d string) int {
			return cmdRender([]string{"w", "s", "--backend", "b", "--verify", "--verifier", "bogus"})
		},
	} {
		dir := t.TempDir()
		if c := run(dir); c != 2 {
			t.Errorf("%s exit %d", name, c)
		}
		if es, _ := os.ReadDir(dir); len(es) != 0 {
			t.Errorf("%s wrote %v", name, es)
		}
	}
	o, styles, limit := write.DefaultOpts(), "", 0
	if _, ok, code := commonFlag([]string{"--verifier", "open"}, 0, &o, &styles, &limit); !ok || code != 0 || o.Verifier != "open" {
		t.Fatalf("%v %d %q", ok, code, o.Verifier)
	}
}

func TestRenderVerifierNeedsVerify(t *testing.T) {
	if c := cmdRender([]string{"w", "s", "--backend", "b", "--verifier", "open"}); c != 2 {
		t.Errorf("exit %d", c)
	}
}

func TestNoDigestFlag(t *testing.T) {
	o := write.DefaultOpts()
	styles, limit := "styles", 0
	if _, ok, code := commonFlag([]string{"--no-digest"}, 0, &o, &styles, &limit); !ok || code != 0 || !o.NoDigest {
		t.Fatalf("ok=%t code=%d NoDigest=%t", ok, code, o.NoDigest)
	}
	got, closeFn, err := withDigest(context.Background(), "w", o)
	if err != nil || got.Digest != nil {
		t.Fatalf("Digest wired despite --no-digest: %v", err)
	}
	closeFn()
}

type okCompleter struct{}

func (okCompleter) Complete(context.Context, string, string) (render.Result, error) {
	return render.Result{Text: "x", StopReason: "end_turn"}, nil
}

// DIEG_CALL_CAP=N lets exactly N calls through, even from many goroutines, and a refused call is not counted.
func TestCallCapExact(t *testing.T) {
	t.Setenv("DIEG_CALL_CAP", "5")
	k := &counter{c: okCompleter{}, n: new(atomic.Int64)}
	var wg sync.WaitGroup
	var ok atomic.Int64
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := k.Complete(context.Background(), "", ""); err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 5 || k.n.Load() != 5 {
		t.Errorf("passed=%d counted=%d, want 5/5", ok.Load(), k.n.Load())
	}
}

func TestConvertRejectsRenderFlags(t *testing.T) {
	for _, f := range [][]string{{"--scenes", "3"}, {"--render-model", "claude-haiku-5-5"}, {"--extract-model", "claude-haiku-5-5"},
		{"--render-effort", "low"}, {"--no-digest"}, {"--verifier", "card"}, {"--backend", "b"}, {"--styles", "s"}} {
		if c := cmdConvert(append([]string{"story.txt", "-o", t.TempDir()}, f...)); c != 2 {
			t.Errorf("convert %v exit %d", f, c)
		}
	}
}

func TestFinishFlags(t *testing.T) {
	for name, c := range map[string]int{
		"write model":    cmdWrite([]string{"idea", "-o", t.TempDir(), "--finish-model", "nope"}),
		"convert model":  cmdConvert([]string{"f.txt", "-o", t.TempDir(), "--finish-model", "nope"}),
		"write rounds":   cmdWrite([]string{"idea", "-o", t.TempDir(), "--finish-rounds", "0"}),
		"convert rounds": cmdConvert([]string{"f.txt", "-o", t.TempDir(), "--finish-rounds", "x"}),
	} {
		if c != 2 {
			t.Errorf("%s: exit %d, want 2", name, c)
		}
	}
	o := write.DefaultOpts()
	if _, ok, c := commonFlag([]string{"--finish-model", "claude-sonnet-5-5"}, 0, &o, new(string), new(int)); !ok || c != 0 || o.FinishModel != "claude-sonnet-5-5" {
		t.Errorf("--finish-model not parsed: ok=%t c=%d %q", ok, c, o.FinishModel)
	}
	if renderOnly["--finish-model"] || renderOnly["--finish-rounds"] {
		t.Error("finish flags must work in convert")
	}
}

func TestAllowWarnOnlyWriteConvert(t *testing.T) {
	for _, f := range [][]string{{"--allow-warn"}, {"--finish-model", "claude-haiku-5-5"}, {"--finish-rounds", "2"}} {
		if c := cmdContinue(append([]string{t.TempDir(), "--ep", "2"}, f...)); c != 2 {
			t.Errorf("continue %v exit %d, want 2", f, c)
		}
		if c := cmdRevise(append([]string{t.TempDir(), "change"}, f...)); c != 2 {
			t.Errorf("revise %v exit %d, want 2", f, c)
		}
	}
	if c := cmdWrite([]string{"idea", "-o", t.TempDir(), "--finish-rounds", "2"}); c != 2 {
		t.Errorf("write --finish-rounds without --finish-model: exit %d, want 2", c)
	}
	if c := cmdConvert([]string{"f.txt", "-o", t.TempDir(), "--finish-rounds", "2"}); c != 2 {
		t.Errorf("convert --finish-rounds without --finish-model: exit %d, want 2", c)
	}
}

func TestCmdConvertGuards(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	short := filepath.Join(dir, "short.txt")
	os.WriteFile(short, []byte("A para.\n"), 0o644)
	os.MkdirAll(filepath.Join(out, "story"), 0o755)
	if c := cmdConvert([]string{short, "-o", out}); c != 1 {
		t.Errorf("existing story/ without --force: exit %d, want 1", c)
	}
	long := filepath.Join(dir, "long.txt")
	os.WriteFile(long, []byte(strings.Repeat("가", 12001)), 0o644)
	if c := cmdConvert([]string{long, "-o", t.TempDir()}); c != 1 {
		t.Errorf("too long: exit %d, want 1", c)
	}
}
