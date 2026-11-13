package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain doubles as the fake `claude`: when READCHECK_FAKE_RESP is set it
// answers the next canned response (JSON array of strings) and exits.
func TestMain(m *testing.M) {
	if rf := os.Getenv("READCHECK_FAKE_RESP"); rf != "" {
		in, _ := io.ReadAll(os.Stdin)
		if sf := os.Getenv("READCHECK_FAKE_STDIN"); sf != "" {
			os.WriteFile(sf, in, 0o644)
		}
		if cf := os.Getenv("READCHECK_FAKE_CWD"); cf != "" {
			wd, _ := os.Getwd()
			os.WriteFile(cf, []byte(wd), 0o644)
		}
		var resp []string
		b, _ := os.ReadFile(rf)
		json.Unmarshal(b, &resp)
		cf := rf + ".n"
		cb, _ := os.ReadFile(cf)
		n := len(cb) // one byte per call so far
		os.WriteFile(cf, append(cb, 'x'), 0o644)
		fmt.Fprintln(os.Stderr, "Permission allow rule foo is ignored")
		fmt.Println("Warning: this sign-in ends soon")
		if n < len(resp) {
			fmt.Println(resp[n])
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func cold(errs, term, vocab int, cont bool) string {
	cs := []map[string]string{}
	for i := 0; i < errs; i++ {
		cs = append(cs, map[string]string{"kind": "error", "text": "x"})
	}
	for i := 0; i < term; i++ {
		cs = append(cs, map[string]string{"kind": "term", "text": "x"})
	}
	for i := 0; i < vocab; i++ {
		cs = append(cs, map[string]string{"kind": "vocab", "text": "x"})
	}
	b, _ := json.Marshal(map[string]any{"summary": "s", "confusions": cs, "continue": cont})
	return "```json\n" + string(b) + "\n```"
}

// runFake runs the tool against canned responses; returns exit code and stdout.
func runFake(t *testing.T, resp []string, extra []string, eps ...string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	rf := filepath.Join(dir, "resp.json")
	b, _ := json.Marshal(resp)
	os.WriteFile(rf, b, 0o644)
	ref := filepath.Join(dir, "ref.txt")
	os.WriteFile(ref, []byte("ref text"), 0o644)
	exe, _ := os.Executable()
	t.Setenv("READCHECK_CLAUDE", exe)
	t.Setenv("READCHECK_FAKE_RESP", rf)
	args := append([]string{"--ref", ref}, extra...)
	for _, e := range eps {
		p := filepath.Join(dir, e)
		os.WriteFile(p, []byte("episode "+e), 0o644)
		args = append(args, p)
	}
	var out bytes.Buffer
	code := run(args, &out)
	return code, out.String()
}

func TestReadcheckVerdict(t *testing.T) {
	cases := []struct {
		name  string
		cold  string
		pair  []string // first round A=manuscript, second round B=manuscript
		want  string
		wins  string
		exit0 bool
	}{
		{"pass", cold(1, 0, 0, true), []string{"A\nwhy", "A\nwhy"}, "PASS", "1/2", true},
		{"error 2", cold(2, 0, 0, true), []string{"A", "B"}, "PASS", "2/2", true},
		{"error 3", cold(3, 0, 0, true), []string{"A", "B"}, "FAIL", "2/2", false},
		{"term", cold(0, 1, 0, true), []string{"A", "B"}, "FAIL", "2/2", false},
		{"pair0", cold(0, 0, 0, true), []string{"B", "A"}, "FAIL", "0/2", false},
		{"vocab ignored", cold(0, 0, 9, false), []string{"A", "B"}, "PASS", "2/2", true},
	}
	for _, c := range cases {
		code, out := runFake(t, append([]string{c.cold}, c.pair...), nil, "ep.md")
		if !strings.Contains(out, c.want) || !strings.Contains(out, "pair="+c.wins) || !strings.Contains(out, "calls=3/3") {
			t.Errorf("%s: output %q", c.name, out)
		}
		if (code == 0) != c.exit0 {
			t.Errorf("%s: exit %d", c.name, code)
		}
	}
}

func TestReadcheckBadJSON(t *testing.T) {
	code, out := runFake(t, []string{"not json at all", cold(0, 0, 0, true), "A", "B"}, []string{"--budget", "6"}, "ep.md")
	if code != 0 || !strings.Contains(out, "PASS") || !strings.Contains(out, "calls=4/6") {
		t.Errorf("retry: exit %d out %q", code, out)
	}
	code, out = runFake(t, []string{"bad", "still bad", "A", "B"}, []string{"--budget", "9"}, "ep.md")
	if code != 1 || !strings.Contains(out, "UNDECIDED") {
		t.Errorf("both bad: exit %d out %q", code, out)
	}
}

func TestReadcheckBudget(t *testing.T) {
	code, out := runFake(t, []string{cold(0, 0, 0, true), "A", "B"}, []string{"--budget", "4"}, "a.md", "b.md")
	if code != 1 || !strings.Contains(out, "b.md SKIPPED(budget)") || !strings.Contains(out, "calls=3/4") {
		t.Errorf("exit %d out %q", code, out)
	}
}

func TestReadcheckStrictParse(t *testing.T) {
	// summary only -> unparsable -> retry
	code, out := runFake(t, []string{`{"summary":"s","continue":true}`, cold(0, 0, 0, true), "A", "B"}, []string{"--budget", "6"}, "ep.md")
	if code != 0 || !strings.Contains(out, "calls=4/6") {
		t.Errorf("summary-only: exit %d out %q", code, out)
	}
	// "Structural " is normalised
	r := `{"summary":"s","confusions":[{"kind":"Error ","text":"x"},{"kind":" ERROR","text":"y"},{"kind":"error","text":"z"}],"continue":true}`
	_, out = runFake(t, []string{r, "A", "B"}, nil, "ep.md")
	if !strings.Contains(out, "error=3") || !strings.Contains(out, "FAIL") {
		t.Errorf("normalise: %q", out)
	}
	// unknown kind -> unparsable both times -> UNDECIDED
	bad := `{"summary":"s","confusions":[{"kind":"plot","text":"x"}],"continue":true}`
	code, out = runFake(t, []string{bad, bad}, []string{"--budget", "6"}, "ep.md")
	if code != 1 || !strings.Contains(out, "UNDECIDED") {
		t.Errorf("plot kind: exit %d out %q", code, out)
	}
}

func TestReadcheckIsolatedCwd(t *testing.T) {
	dir := t.TempDir()
	cwdf := filepath.Join(dir, "cwd.txt")
	t.Setenv("READCHECK_FAKE_CWD", cwdf)
	runFake(t, []string{cold(0, 0, 0, true), "A", "B"}, nil, "ep.md")
	got, _ := os.ReadFile(cwdf)
	here, _ := os.Getwd()
	if len(got) == 0 || string(got) == here {
		t.Errorf("fake saw cwd %q, caller %q", got, here)
	}
	if _, err := os.Stat(string(got)); err == nil {
		t.Errorf("temp cwd %q not removed", got)
	}
}

func TestReadcheckOpenIgnoredLegacyRejected(t *testing.T) {
	var cs []string
	for i := 0; i < 9; i++ {
		cs = append(cs, `{"kind":"open","text":"x"}`)
	}
	open9 := `{"summary":"s","confusions":[` + strings.Join(cs, ",") + `],"continue":true}`
	_, out := runFake(t, []string{open9, "A", "B"}, nil, "ep.md")
	if !strings.Contains(out, "error=0 open=9 term=0") || !strings.Contains(out, "PASS") {
		t.Errorf("open 9: %q", out)
	}
	legacy := `{"summary":"s","confusions":[{"kind":"structural","text":"x"}],"continue":true}`
	code, out := runFake(t, []string{legacy, legacy}, []string{"--budget", "6"}, "ep.md")
	if code != 1 || !strings.Contains(out, "UNDECIDED") {
		t.Errorf("legacy: exit %d %q", code, out)
	}
}

func TestReadcheckPickFirstLineOnly(t *testing.T) {
	for in, want := range map[string]bool{"Both are good.\nA is better": false, "B.\nwhy": true, "\n\nA\nwhy": true, "Warning: this sign-in ends\nA\nwhy": true, "A is better": false} {
		if _, ok := parsePick(in); ok != want {
			t.Errorf("parsePick(%q) ok=%v want %v", in, ok, want)
		}
	}
	if v, _ := parsePick("B."); v != "B" {
		t.Errorf("B. -> %q", v)
	}
}

func TestReadcheckEmptySummary(t *testing.T) {
	empty := `{"summary":"  ","confusions":[],"continue":true}`
	code, out := runFake(t, []string{empty, cold(3, 0, 0, true), "A", "B"}, []string{"--budget", "6"}, "ep.md")
	if code != 1 || !strings.Contains(out, "error=3") || !strings.Contains(out, "calls=4/6") {
		t.Errorf("exit %d out %q", code, out)
	}
}

func TestReadcheckColdOnlyExit(t *testing.T) {
	code, _ := runFake(t, []string{cold(3, 0, 0, true)}, []string{"--cold-only"}, "ep.md")
	if code != 1 {
		t.Errorf("cold-only fail exit %d", code)
	}
	code, _ = runFake(t, []string{cold(0, 0, 0, true)}, []string{"--cold-only"}, "ep.md")
	if code != 0 {
		t.Errorf("cold-only pass exit %d", code)
	}
}

// kinds builds a cold response with n confusions of each given kind.
func kinds(m map[string]int) string {
	cs := []map[string]string{}
	for k, n := range m {
		for i := 0; i < n; i++ {
			cs = append(cs, map[string]string{"kind": k, "text": "x"})
		}
	}
	b, _ := json.Marshal(map[string]any{"summary": "s", "confusions": cs, "continue": true})
	return string(b)
}

func TestReadcheckNewKinds(t *testing.T) {
	for _, c := range []struct {
		name string
		m    map[string]int
		want string
	}{
		{"repeat 1", map[string]int{"repeat": 1}, "FAIL"},
		{"dump 2", map[string]int{"dump": 2}, "FAIL"},
		{"flat 2", map[string]int{"flat": 2}, "FAIL"},
		{"dump 1 flat 1", map[string]int{"dump": 1, "flat": 1}, "PASS"},
	} {
		_, out := runFake(t, []string{kinds(c.m), "A", "B"}, nil, "ep.md")
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: %q", c.name, out)
		}
		for k, n := range c.m {
			if !strings.Contains(out, fmt.Sprintf("%s=%d", k, n)) {
				t.Errorf("%s: %s count missing in %q", c.name, k, out)
			}
		}
	}
}

func TestReadcheckStateFile(t *testing.T) {
	dir := t.TempDir()
	rf := filepath.Join(dir, "resp.json")
	b, _ := json.Marshal([]string{kinds(nil), "A", "B", kinds(nil), "A", "B"})
	os.WriteFile(rf, b, 0o644)
	ref := filepath.Join(dir, "ref.txt")
	os.WriteFile(ref, []byte("ref text"), 0o644)
	os.WriteFile(filepath.Join(dir, "ep02.md"), []byte("EPTWO"), 0o644)
	os.WriteFile(filepath.Join(dir, "ep03.md"), []byte("EPTHREE"), 0o644)
	os.WriteFile(filepath.Join(dir, "ep02.state.txt"), []byte("용어 칩: 신분 칩\n"), 0o644)
	exe, _ := os.Executable()
	t.Setenv("READCHECK_CLAUDE", exe)
	t.Setenv("READCHECK_FAKE_RESP", rf)
	sf := filepath.Join(dir, "stdin.txt")
	t.Setenv("READCHECK_FAKE_STDIN", sf)
	var out bytes.Buffer
	if code := run([]string{"--ref", ref, "--cold-only", filepath.Join(dir, "ep02.md")}, &out); code != 0 {
		t.Fatalf("exit %d %q", code, out.String())
	}
	got, _ := os.ReadFile(sf)
	if !strings.Contains(string(got), "당신은 앞 회차를 읽었습니다. 다음은 지금까지 당신이 알게 된 것입니다:") || !strings.Contains(string(got), "용어 칩: 신분 칩") {
		t.Errorf("state not in prompt: %q", got)
	}
	// no sibling state file -> first-time reader
	run([]string{"--ref", ref, "--cold-only", filepath.Join(dir, "ep03.md")}, &out)
	got, _ = os.ReadFile(sf)
	if strings.Contains(string(got), "다음은 지금까지 당신이 알게 된 것입니다") || !strings.Contains(string(got), "EPTHREE") {
		t.Errorf("ep03 should be cold: %q", got)
	}
	// explicit --state applies to every episode
	os.WriteFile(filepath.Join(dir, "all.txt"), []byte("등장인물: 안\n"), 0o644)
	run([]string{"--ref", ref, "--cold-only", "--state", filepath.Join(dir, "all.txt"), filepath.Join(dir, "ep03.md")}, &out)
	got, _ = os.ReadFile(sf)
	if !strings.Contains(string(got), "등장인물: 안") {
		t.Errorf("--state ignored: %q", got)
	}
}

func TestReadcheckUnknownNewKind(t *testing.T) {
	bad := `{"summary":"s","confusions":[{"kind":"boring","text":"x"}],"continue":true}`
	code, out := runFake(t, []string{bad, bad}, []string{"--budget", "6"}, "ep.md")
	if code != 1 || !strings.Contains(out, "UNDECIDED") {
		t.Errorf("exit %d %q", code, out)
	}
}

func TestColdPromptTermExcludesNames(t *testing.T) {
	p := coldPromptFor("")
	for _, w := range []string{"이름은 문맥상 이름임이 분명하면 term 이 아니다", "푸른 약국"} {
		if !strings.Contains(p, w) {
			t.Errorf("prompt lacks %q", w)
		}
	}
}

func TestPromptsAreMediumNeutral(t *testing.T) {
	for _, p := range []string{coldPromptFor(""), coldPromptFor("용어 칩: x"), pairIntro} {
		for _, bad := range []string{"novel", "소설", "옥호", "미닫이", "나루터"} {
			if strings.Contains(p, bad) {
				t.Errorf("prompt contains %q", bad)
			}
		}
	}
	for _, want := range []string{"푸른 약국", "출입 규칙"} {
		if !strings.Contains(coldPromptFor(""), want) {
			t.Errorf("cold prompt lacks example %q", want)
		}
	}
}

func TestReadcheckStateFailClosed(t *testing.T) {
	dir := t.TempDir()
	rf := filepath.Join(dir, "resp.json")
	b, _ := json.Marshal([]string{kinds(nil), kinds(nil), kinds(nil)})
	os.WriteFile(rf, b, 0o644)
	ref := filepath.Join(dir, "ref.txt")
	os.WriteFile(ref, []byte("ref"), 0o644)
	for _, n := range []string{"ep01", "ep02", "ep03"} {
		os.WriteFile(filepath.Join(dir, n+".md"), []byte(n), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "ep02.state.txt"), []byte("등장인물: 안\n"), 0o644)
	exe, _ := os.Executable()
	t.Setenv("READCHECK_CLAUDE", exe)
	t.Setenv("READCHECK_FAKE_RESP", rf)
	ep := func(n string) string { return filepath.Join(dir, n+".md") }
	// an explicit --state that cannot be read: exit 1, no model call
	var out bytes.Buffer
	var code int
	errs := captureStderr(t, func() {
		code = run([]string{"--ref", ref, "--cold-only", "--state", filepath.Join(dir, "nope.txt"), ep("ep02")}, &out)
	})
	if code != 1 || !strings.Contains(errs, "nope.txt") || strings.Contains(out.String(), "calls=") {
		t.Errorf("missing --state: exit %d out %q err %q", code, out.String(), errs)
	}
	if _, err := os.Stat(rf + ".n"); err == nil {
		t.Errorf("model was called despite unreadable --state")
	}
	// result lines show the state; ep01 has none (no warning for the first episode), ep03 lacks one while ep02 has one (warn)
	out.Reset()
	errs = captureStderr(t, func() {
		code = run([]string{"--ref", ref, "--cold-only", ep("ep01"), ep("ep02"), ep("ep03")}, &out)
	})
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if !strings.Contains(lines[0], "state=none") || !strings.Contains(lines[1], "state="+filepath.Join(dir, "ep02.state.txt")) || !strings.Contains(lines[2], "state=none") {
		t.Errorf("state= missing: %q", out.String())
	}
	if !strings.Contains(errs, "ep03.md") || strings.Contains(errs, "ep01.md") {
		t.Errorf("warning wrong: %q", errs)
	}
}

// captureStderr runs f with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	f()
	os.Stderr = old
	w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}
