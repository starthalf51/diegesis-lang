package sema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"diegesis/internal/ir"
	"diegesis/internal/syntax"
)

const goldenDir = "../../testdata/golden"

const cast = `module t.cast
work t "T"
  start 1000-01-01
character a "A"
character b "B"
character c "C"
character w1 "W1"
character w2 "W2"
group ws "WS"
  members w1, w2
place room "Room"
fact p "P" true
`

func parse(t *testing.T, srcs map[string]string) []*syntax.File {
	t.Helper()
	var files []*syntax.File
	names := make([]string, 0, len(srcs))
	for n := range srcs {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		f, diags := syntax.ParseFile(n, []byte(srcs[n]))
		for _, d := range diags {
			t.Fatalf("parse: %s", d)
		}
		files = append(files, f)
	}
	return files
}

func build(t *testing.T, scene string) (*ir.Work, []syntax.Diag) {
	t.Helper()
	w, diags := Build(parse(t, map[string]string{"cast.dieg": cast, "s.dieg": scene}))
	if w == nil {
		t.Fatal("Build returned nil IR")
	}
	return w, diags
}

func noErrors(t *testing.T, diags []syntax.Diag) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Errorf("unexpected: %s", d)
		}
	}
}

// SL_UPDATE_GOLDEN=1 rewrites macbeth.ir.json.
func TestBuildMacbethGolden(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join(goldenDir, "macbeth-*.dieg.md"))
	if len(paths) != 4 {
		t.Fatalf("want 4 macbeth inputs (3 scenes + cast), got %v", paths)
	}
	var files []*syntax.File
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, diags := syntax.ParseFile(filepath.Base(p), src)
		for _, d := range diags {
			t.Fatalf("parse: %s", d)
		}
		files = append(files, f)
	}
	w, diags := Build(files)
	for _, d := range diags {
		t.Errorf("unexpected diag: %s", d)
	}
	got, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	want := filepath.Join(goldenDir, "macbeth.ir.json")
	if os.Getenv("SL_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(want, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	exp, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("%v (run with SL_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(exp, []byte("\r\n"), []byte("\n")), got) {
		t.Errorf("IR differs from %s (SL_UPDATE_GOLDEN=1 to regenerate)", want)
	}
}

func audience(w *ir.Work, act string) []string {
	var out []string
	for _, a := range w.Audience {
		if a.ActID == act {
			out = append(out, strings.TrimPrefix(a.EntityID, "t/t.cast/"))
		}
	}
	return out
}

func TestAudienceWhisperAside(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
scene 1 at room
  enter a, b, c, ws
  a whispers b: p
  a aside: "x"
  a thinks: "y"
  a says: p
  w1 tells a: p
  exit c
  a tells b: p
  a sees p
`)
	noErrors(t, diags)
	s := "t/t.s/1#a"
	cases := map[string][]string{
		s + "1": {"b"},
		s + "2": nil,
		s + "3": nil,
		s + "4": {"b", "c", "ws", "w1", "w2"},
		s + "5": {"a", "b", "c", "w2"}, // the speaker's group is not its own audience
		s + "6": {"b", "ws", "w1", "w2"},
		s + "7": nil,
	}
	for act, want := range cases {
		if got := audience(w, act); !slices.Equal(got, want) {
			t.Errorf("%s audience = %v, want %v", act, got, want)
		}
	}
}

func TestOffstageSpeakerWarnWaivable(t *testing.T) {
	_, diags := build(t, `module t.s
import t.cast
scene 1 at room
  enter a
  b says: p
  #! allow offstage-speaker: "voice from off"
  c says: p
`)
	var got []string
	for _, d := range diags {
		got = append(got, d.Rule+"/"+d.Severity+"/"+strings.Repeat("w", btoi(d.Waived)))
	}
	want := []string{"offstage-speaker/warn/", "offstage-speaker/warn/w"}
	if !slices.Equal(got, want) {
		t.Errorf("diags = %v, want %v", got, want)
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestUndefinedRef(t *testing.T) {
	_, diags := build(t, `module t.s
import t.cast
scene 1 at room
  enter a, nobody
  a tells b: q
  when after 9.9
`)
	type pos struct {
		Rule      string
		Line, Col int
	}
	var got []pos
	for _, d := range diags {
		if d.File != "s.dieg" || d.Severity != "error" {
			t.Errorf("want error in s.dieg: %s", d)
		}
		got = append(got, pos{d.Rule, d.Line, d.Col})
	}
	want := []pos{{"undefined-ref", 4, 12}, {"undefined-ref", 5, 14}, {"undefined-ref", 6, 14}}
	if !slices.Equal(got, want) {
		t.Errorf("diags = %v, want %v", got, want)
	}
}

func TestErrorsKindsAndImports(t *testing.T) {
	w, diags := Build(parse(t, map[string]string{
		"cast.dieg": cast,
		"x.dieg":    "module t.x\nimport t.y\n",
		"y.dieg":    "module t.y\nimport t.x\n",
		"s.dieg": `module t.s
import t.cast
scene 1 at a
  enter room
  a tells b: holds(p, a)
  a reads p
`}))
	var got []string
	for _, d := range diags {
		if d.Severity == "error" {
			got = append(got, d.File+":"+d.Rule)
		}
	}
	want := []string{"s.dieg:arg-kind", "s.dieg:arg-kind", "s.dieg:role-unknown", "s.dieg:arg-kind", "y.dieg:import-cycle"}
	if !slices.Equal(got, want) {
		t.Errorf("diags = %v, want %v", got, want)
	}
	// modules with errors emit nothing; clean modules still do
	if len(w.Scenes) != 0 || len(w.Entities) == 0 || len(w.Modules) != 2 {
		t.Errorf("scenes=%d entities=%d modules=%v", len(w.Scenes), len(w.Entities), w.Modules)
	}
}

func TestWaiverNeedsReason(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
scene 1 at room
  enter a
  #! allow belief-contradiction: ""
  a says: p
  #! allow belief-contradiction: "ok"
  a says: p
`)
	if len(diags) != 1 || diags[0].Rule != "waiver-reason" || diags[0].Severity != "error" || diags[0].Line != 5 {
		t.Fatalf("diags = %v, want one waiver-reason error at line 5", diags)
	}
	_ = w
	w, diags = build(t, `module t.s
import t.cast
scene 1 at room
  enter a
  #! allow belief-contradiction: "ok"
  a says: p
`)
	noErrors(t, diags)
	want := []ir.Waiver{{Rule: "belief-contradiction", TargetKind: "act", TargetID: "t/t.s/1#a1", Reason: "ok"}}
	for i := range w.Waivers {
		w.Waivers[i].SrcID = 0
	}
	if !slices.Equal(w.Waivers, want) {
		t.Errorf("waivers = %+v, want %+v", w.Waivers, want)
	}
}

type edge struct {
	From, To string
	Min, Max int64 // -1 = nil
	Waived   bool
}

func edges(w *ir.Work) []edge {
	var out []edge
	n := func(p *int64) int64 {
		if p == nil {
			return -1
		}
		return *p
	}
	for _, e := range w.TimeEdges {
		if e.From == "t/start" && e.Min != nil && *e.Min == 0 && e.Max == nil {
			continue // every point is after the origin
		}
		out = append(out, edge{strings.TrimPrefix(e.From, "t/t.s/"), strings.TrimPrefix(e.To, "t/t.s/"), n(e.Min), n(e.Max), e.Waived})
	}
	return out
}

func TestTimeEdges(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
scene X at room
  enter a
  a says: p
scene Y at room
  when after X by 0d..1d
  enter a
  beat b1:
    a says: p
scene Z at room
  #! allow time-cycle: "two clocks"
  when before Y
  when at 1000-02
  when during X
born a when after X
`)
	noErrors(t, diags)
	got := edges(w)
	want := []edge{
		{"X.start", "X#a1", 0, -1, false},
		{"X#a1", "X.end", 0, -1, false},
		{"X.end", "Y.start", 0, 86400, false},
		{"Y.start", "Y#b1", 0, -1, false},
		{"Y#b1", "Y#a1", 0, -1, false},
		{"Y#a1", "Y.end", 0, -1, false},
		{"Z.end", "Y.start", 0, -1, true},
		{"t/start", "Z.start", 31 * 86400, 59*86400 - 1, false},
		{"X.start", "Z.start", 0, -1, false},
		{"Z.end", "X.end", 0, -1, false},
		{"Z.start", "Z.end", 0, -1, false},
		{"X.end", "t/t.cast/a.born", 0, -1, false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("edges:\n got %v\nwant %v", got, want)
	}
	for _, e := range w.TimeEdges {
		if e.Waived && (e.WaiverRule != "time-cycle" || e.WaiverReason != "two clocks") {
			t.Errorf("waived edge without rule/reason: %+v", e)
		}
	}
}

func TestEventAccountPayoffRole(t *testing.T) {
	w, diags := Build(parse(t, map[string]string{
		"cast.dieg": cast + "role chief \"Chief\"\n  held a from start\nartifact letter \"L\"\n  contains p\n",
		"s.dieg": `module t.s
import t.cast
event e "off stage"
  when after 1
  b writes letter
account acc of e by c
  b tells a: p
scene 1 at room
  enter a, b
  setup s1 promise: p
  role:chief says: p
scene 2 at room
  discourse after 1
  enter b
  b reads letter
  payoff t.s/s1
dies a when at 1000-03
`}))
	noErrors(t, diags)
	if len(w.Accounts) != 1 || w.Accounts[0].EventID != "t/t.s/e" || w.Accounts[0].SourceID != "t/t.cast/c" {
		t.Errorf("accounts = %+v", w.Accounts)
	}
	if len(w.Payoffs) != 1 || w.Payoffs[0].SetupID != "t/t.s/s1" || w.Payoffs[0].AtPt != "t/t.s/2#a1" {
		t.Errorf("payoffs = %+v", w.Payoffs)
	}
	if got := audience(w, "t/t.s/acc#a1"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("off-stage tells audience = %v, want named recipient", got)
	}
	var role *ir.Act
	for i := range w.Acts {
		if w.Acts[i].ActorIsRole {
			role = &w.Acts[i]
		}
	}
	if role == nil || role.ActorID != "t/t.cast/chief" {
		t.Errorf("role actor = %+v", role)
	}
	if w.Scenes[3].DiscourseAfter != "t/t.s/1" || w.Scenes[2].DiscourseOrd != 1 {
		t.Errorf("scenes = %+v", w.Scenes)
	}
	if len(w.RoleHolders) != 1 || w.RoleHolders[0].FromPt != "t/start" {
		t.Errorf("holders = %+v", w.RoleHolders)
	}
}
