package write

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

var ctx = context.Background()

type fakeLLM struct {
	replies []string
	prompts []string
}

func (f *fakeLLM) Complete(_ context.Context, system, user string) (render.Result, error) {
	f.prompts = append(f.prompts, user)
	r := f.replies[0]
	if len(f.replies) > 1 {
		f.replies = f.replies[1:]
	}
	return render.Result{Text: r, StopReason: "end_turn", Usage: render.Usage{Input: 10, Output: 5}}, nil
}

const good = "=== FILE: cast.dieg.md ===\n# cast\n=== FILE: scenes/s1.dieg.md ===\n# s1\n" + End + "\n"

func checks(rounds ...[]Diag) (Checker, *int) {
	n := 0
	return func(_ context.Context, dir string) ([]Diag, error) {
		if _, err := os.Stat(filepath.Join(dir, "cast.dieg.md")); err != nil {
			return nil, err
		}
		d := rounds[min(n, len(rounds)-1)]
		n++
		return d, nil
	}, &n
}

func testRefs(t *testing.T) Refs {
	r, err := LoadRefs()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLoopCleanFirstRound(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{good}}
	chk, _ := checks([]Diag{{File: "cast.dieg.md", Line: 1, Col: 1, Severity: "info", Rule: "x", Msg: "fine"}})
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out})
	if err != nil || res.Rounds != 1 || !res.Clean || len(f.prompts) != 1 {
		t.Fatalf("%+v %v prompts=%d", res, err, len(f.prompts))
	}
	if _, err := os.Stat(filepath.Join(out, "story", "scenes", "s1.dieg.md")); err != nil {
		t.Error("story/ not written:", err)
	}
	if !strings.Contains(f.prompts[0], "idea") || !strings.Contains(f.prompts[0], "X.at") || !strings.Contains(f.prompts[0], End) {
		t.Error("code prompt lacks idea / .at convention / END contract")
	}
}

func TestLoopFeedsDiagnostics(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{good, good}}
	ds := []Diag{
		{File: "scenes/s1.dieg.md", Line: 3, Col: 5, Severity: "error", Rule: "parse", Msg: "unexpected token"},
		{File: "cast.dieg.md", Line: 7, Col: 1, Severity: "warn", Rule: "info-flow", Msg: "a says b without having learned it"},
	}
	chk, _ := checks(ds, nil)
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out})
	if err != nil || res.Rounds != 2 || !res.Clean {
		t.Fatalf("%+v %v", res, err)
	}
	p := f.prompts[1]
	for _, want := range []string{
		"scenes/s1.dieg.md:3:5: error parse: unexpected token",
		"cast.dieg.md:7:1: warn info-flow: a says b without having learned it",
		"=== FILE: cast.dieg.md ===\n# cast\n", "=== FILE: scenes/s1.dieg.md ===\n# s1\n",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("fix prompt lacks %q", want)
		}
	}
}

func TestLoopGivesUp(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{good}}
	stuck := []Diag{{File: "cast.dieg.md", Line: 2, Col: 1, Severity: "warn", Rule: "dead-actor", Msg: "still dead"}}
	chk, n := checks(stuck)
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out, Render: true})
	if err != nil || res.Rounds != 4 || res.Clean || *n != 4 {
		t.Fatalf("%+v %v checks=%d", res, err, *n)
	}
	if len(res.Diags) != 1 || res.Diags[0].Rule != "dead-actor" {
		t.Errorf("diags %+v", res.Diags)
	}
	if len(f.prompts) != 4 { // 4 code/fix calls, no render calls
		t.Errorf("calls %d, want 4", len(f.prompts))
	}
	b, _ := os.ReadFile(filepath.Join(out, "diags.txt"))
	if !strings.Contains(string(b), "dead-actor") {
		t.Error("diags.txt lacks remaining diagnostic")
	}
	if _, err := os.Stat(filepath.Join(out, "story", "cast.dieg.md")); err != nil {
		t.Error("last code not kept in story/")
	}
	if _, err := os.Stat(filepath.Join(out, "novel.md")); err == nil {
		t.Error("rendered although not clean")
	}
}

func TestLoopRejectsBadBundle(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{"Sure! Here is the story.", "=== FILE: ../esc.dieg.md ===\nx\n", good}}
	chk, n := checks(nil)
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out})
	if err != nil || res.Rounds != 3 || !res.Clean || *n != 1 {
		t.Fatalf("%+v %v checks=%d", res, err, *n)
	}
	if !strings.Contains(f.prompts[1], "-:0:0: error bundle: ") || !strings.Contains(f.prompts[2], "error bundle: bad file path") {
		t.Errorf("bundle diag not fed back:\n%s", f.prompts[1][strings.LastIndex(f.prompts[1], "# 도구 진단"):])
	}
	if _, err := os.Stat(filepath.Join(out, "rounds", "r2")); err == nil {
		t.Error("bad bundle written")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(out), "esc.dieg.md")); err == nil {
		t.Error("file escaped")
	}
}

func TestLoopMeterLabels(t *testing.T) {
	var buf bytes.Buffer
	m := render.NewMeter(&fakeLLM{replies: []string{"junk", good}}, &buf)
	chk, _ := checks(nil)
	if _, err := Loop(ctx, m, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if strings.Count(s, "\n") != 2 || !strings.Contains(s, `"label":"code-r1"`) || !strings.Contains(s, `"label":"fix-r2"`) {
		t.Errorf("calls.jsonl:\n%s", s)
	}
}

func TestNovelLabelsAndFiles(t *testing.T) {
	s, err := render.LoadScene("../render/testdata/scenes/chibi-jianggan.json")
	if err != nil {
		t.Fatal(err)
	}
	bs, err := render.LoadBackends("../../styles")
	if err != nil || len(bs) == 0 {
		t.Fatal(bs, err)
	}
	var buf bytes.Buffer
	out := t.TempDir()
	// Every extraction is unparseable, so verification never passes: 3 renders + 3 extractions, then give up.
	m := render.NewMeter(&fakeLLM{replies: []string{"산문1", "not json", "산문2", "not json", "산문3", "not json"}}, &buf)
	rs, err := Novel(ctx, render.Completers{Render: m, Extract: m}, "w", []render.Scene{s}, bs[0], out, Opts{})
	if err != nil || len(rs) != 1 || rs[0].Pass || rs[0].Attempts != 3 {
		t.Fatalf("%+v %v", rs, err)
	}
	for _, l := range []string{"render-" + s.ID + "-a1", "extract-" + s.ID + "-a1", "render-" + s.ID + "-a3", "extract-" + s.ID + "-a3"} {
		if !strings.Contains(buf.String(), `"label":"`+l+`"`) {
			t.Errorf("no %s in\n%s", l, buf.String())
		}
	}
	if strings.Count(buf.String(), "\n") != 6 {
		t.Errorf("want 6 calls:\n%s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(out, "novel", s.ID+"."+bs[0].ID+".report.json")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(out, "novel", s.ID+"."+bs[0].ID+".md")); err != nil {
		t.Error(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "novel.md")); !strings.Contains(string(b), "산문3") {
		t.Error("novel.md lacks prose")
	}
}

func TestRefsInSync(t *testing.T) {
	for copy, orig := range map[string]string{
		"refs/grammar.ebnf": "../../spec/grammar.ebnf",
		"refs/README.md":    "../../README.ko.md",
		"refs/cast.dieg.md": "../../corpus/macbeth/cast.dieg.md",
		"refs/1.3.dieg.md":  "../../corpus/macbeth/act1/1.3.dieg.md",
	} {
		a, err1 := refsFS.ReadFile(copy)
		b, err2 := os.ReadFile(orig)
		if err1 != nil || err2 != nil || !bytes.Equal(a, b) {
			t.Errorf("%s out of sync with %s (%v %v); copy it again", copy, orig, err1, err2)
		}
	}
	r := testRefs(t)
	if !strings.Contains(r.Keywords, "X.at") || !strings.Contains(r.Keywords, "object-location") || strings.Contains(r.Keywords, "## 테스트") {
		t.Errorf("keywords section:\n%s", r.Keywords)
	}
}

func TestLoopDiagsTxtDescribesStory(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{good, "no header"}}
	chk, _ := checks([]Diag{{File: "cast.dieg.md", Line: 1, Col: 1, Severity: "warn", Rule: "dead-actor", Msg: "m"}})
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out})
	if err != nil || res.Clean || res.Diags[0].Rule != "dead-actor" {
		t.Fatalf("%+v %v", res, err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "diags.txt"))
	if !strings.Contains(string(b), "dead-actor") || strings.Contains(string(b), "bundle") {
		t.Errorf("diags.txt should describe story/ (round 1), got:\n%s", b)
	}
}

func TestFileIDAndPromptRules(t *testing.T) {
	if got := fileID(`a/b?c`); got != "a_b_c" {
		t.Errorf("fileID %q", got)
	}
	p := CodePrompt("idea", testRefs(t), nil, false)
	for _, w := range []string{"8–12", "exit all", "road_cut"} {
		if !strings.Contains(p, w) {
			t.Errorf("prompt lacks %q", w)
		}
	}
}

func TestSkeletonCodePrompt(t *testing.T) {
	r := testRefs(t)
	on := CodePrompt("idea", r, nil, true)
	off := CodePrompt("idea", r, nil, false)
	if !strings.Contains(on, "인용 문자열을 쓰지 않는다") || strings.Contains(off, "인용 문자열을 쓰지 않는다") {
		t.Error("skeleton instruction only in skeleton mode")
	}
	if !strings.Contains(off, "인용 문자열(대사·생각)이 선언된 사실을 말하면") {
		t.Error("non-skeleton keeps the current quote rule")
	}
	if !strings.Contains(on, "8–12") || !strings.Contains(on, "exit all") {
		t.Error("skeleton keeps the other rules")
	}
}

func rerr(rule string) Diag  { return Diag{File: "cast.dieg.md", Line: 1, Col: 1, Severity: "error", Rule: rule, Msg: "m"} }
func rwarn(rule string) Diag { return Diag{File: "cast.dieg.md", Line: 1, Col: 1, Severity: "warn", Rule: rule, Msg: "m"} }

func goodWith(body string) string {
	return "=== FILE: cast.dieg.md ===\n# cast\n=== FILE: scenes/s1.dieg.md ===\n# " + body + "\n" + End + "\n"
}

func storyHas(t *testing.T, out, want string) {
	t.Helper()
	if b, _ := os.ReadFile(filepath.Join(out, "story", "scenes", "s1.dieg.md")); !strings.Contains(string(b), want) {
		t.Errorf("story/ lacks %q: %q", want, b)
	}
}

func TestLoopKeepsBestRound(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{goodWith("one"), goodWith("two")}}
	chk, _ := checks([]Diag{rwarn("w1")}, []Diag{rerr("e1"), rerr("e2")})
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out})
	if err != nil || res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(res.Diags) != 1 || res.Diags[0].Rule != "w1" {
		t.Errorf("Result.Diags not round 1: %+v", res.Diags)
	}
	storyHas(t, out, "one")
	if b, _ := os.ReadFile(filepath.Join(out, "diags.txt")); !strings.Contains(string(b), "w1") || strings.Contains(string(b), "e1") {
		t.Errorf("diags.txt not round 1: %q", b)
	}

	// a tie keeps the earlier round
	out = t.TempDir()
	f = &fakeLLM{replies: []string{goodWith("one"), goodWith("two")}}
	chk, _ = checks([]Diag{rwarn("w1")}, []Diag{rwarn("w2")})
	res, err = Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out})
	if err != nil || res.Diags[0].Rule != "w1" {
		t.Fatalf("tie: %+v %v", res, err)
	}
	storyHas(t, out, "one")

	// a later strictly better round wins; a malformed round is never a candidate
	out = t.TempDir()
	f = &fakeLLM{replies: []string{goodWith("one"), "no header", goodWith("three")}}
	chk, _ = checks([]Diag{rerr("e1")}, []Diag{rwarn("w3")})
	res, err = Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 3, Out: out})
	if err != nil || res.Diags[0].Rule != "w3" {
		t.Fatalf("better: %+v %v", res, err)
	}
	storyHas(t, out, "three")
}

func TestLoopAllowWarn(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{good}}
	chk, _ := checks([]Diag{rwarn("w1")})
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out, AllowWarn: true})
	if err != nil || !res.Clean || res.Rounds != 1 || len(f.prompts) != 1 {
		t.Fatalf("%+v %v prompts=%d", res, err, len(f.prompts))
	}

	// errors still block, and only they are sent back
	out = t.TempDir()
	f = &fakeLLM{replies: []string{good, good}}
	chk, _ = checks([]Diag{rwarn("w1"), rerr("e1")}, nil)
	res, err = Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 4, Out: out, AllowWarn: true})
	if err != nil || !res.Clean || res.Rounds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if !strings.Contains(f.prompts[1], "e1") || strings.Contains(f.prompts[1], "w1") {
		t.Error("fix prompt should carry the error only")
	}

	// an error-severity lint rule (reader-unknown-term) is not passed by AllowWarn either
	out = t.TempDir()
	f = &fakeLLM{replies: []string{good}}
	chk, _ = checks([]Diag{rerr("reader-unknown-term")})
	res, err = Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 1, Out: out, AllowWarn: true})
	if err != nil || res.Clean || len(res.Diags) != 1 || res.Diags[0].Rule != "reader-unknown-term" {
		t.Fatalf("lint error passed: %+v %v", res, err)
	}
}

func TestLoopCoveragePrefersFullRound(t *testing.T) {
	out := t.TempDir()
	f := &fakeLLM{replies: []string{goodWith("full"), goodWith("short")}}
	chk, _ := checks([]Diag{rerr("e1"), rerr("e2")}, []Diag{rerr("e3")})
	cov := func(dir string) int {
		if b, _ := PackFiles(dir); strings.Contains(b, "full") {
			return 10
		}
		return 1
	}
	res, err := Loop(ctx, f, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out, Coverage: cov})
	if err != nil || len(res.Diags) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	storyHas(t, out, "full") // fewer errors from dropping scenes does not win
}

func TestGroupDiags(t *testing.T) {
	var ds []Diag
	ds = append(ds, Diag{File: "ep01/s9.dieg.md", Line: 1, Col: 1, Severity: "warn", Rule: "other", Msg: "x"})
	for i := 0; i < 18; i++ {
		ds = append(ds, Diag{File: "ep01/s" + strconv.Itoa(i) + ".dieg.md", Line: 5 + i, Col: 3, Severity: "error", Rule: "no-name", Msg: "missing name"})
	}
	ds = append(ds, Diag{File: "ep01/s1.dieg.md", Line: 2, Col: 1, Severity: "error", Rule: "no-name", Msg: "different"})
	got := GroupDiags(ds)
	if len(got) != 3 || got[0].Rule != "other" || got[0].Msg != "x" || got[2].Msg != "different" {
		t.Fatalf("%+v", got)
	}
	g := got[1]
	if g.File != "ep01/s0.dieg.md" || g.Line != 5 || g.Col != 3 || g.Msg != "missing name (18 places; first at ep01/s0.dieg.md:5:3)" {
		t.Errorf("group %+v", g)
	}
}

func TestLoopFinish(t *testing.T) {
	out := t.TempDir()
	main := &fakeLLM{replies: []string{goodWith("main1"), goodWith("main2")}}
	fin := &fakeLLM{replies: []string{goodWith("fin1"), goodWith("fin2")}}
	chk, _ := checks([]Diag{rwarn("w1")}, []Diag{rerr("e1"), rerr("e2")}, []Diag{rerr("e3")}, nil)
	res, err := Loop(ctx, main, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out, Finish: fin})
	if err != nil || !res.Clean || res.Rounds != 4 || len(fin.prompts) != 2 || len(main.prompts) != 2 {
		t.Fatalf("%+v %v finish=%d", res, err, len(fin.prompts))
	}
	if !strings.Contains(fin.prompts[0], "main1") || strings.Contains(fin.prompts[0], "main2") {
		t.Error("first finish prompt should carry the best round's code, not the last")
	}
	storyHas(t, out, "fin2")

	// clean in the main rounds: the finisher is never called
	out = t.TempDir()
	main = &fakeLLM{replies: []string{good}}
	fin = &fakeLLM{replies: []string{good}}
	chk, _ = checks(nil)
	res, err = Loop(ctx, main, chk, "idea", testRefs(t), Opts{Rounds: 2, Out: out, Finish: fin})
	if err != nil || !res.Clean || res.Rounds != 1 || len(fin.prompts) != 0 {
		t.Fatalf("%+v %v finish=%d", res, err, len(fin.prompts))
	}

	// FinishRounds bounds the extra rounds
	out = t.TempDir()
	main = &fakeLLM{replies: []string{good}}
	fin = &fakeLLM{replies: []string{good}}
	chk, _ = checks([]Diag{rerr("e1")})
	res, err = Loop(ctx, main, chk, "idea", testRefs(t), Opts{Rounds: 1, Out: out, Finish: fin, FinishRounds: 1})
	if err != nil || res.Clean || res.Rounds != 2 || len(fin.prompts) != 1 {
		t.Fatalf("%+v %v finish=%d", res, err, len(fin.prompts))
	}
}
