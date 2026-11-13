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
		io.Copy(io.Discard, os.Stdin)
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
