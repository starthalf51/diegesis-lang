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

func cold(structural, term, vocab int, cont bool) string {
	var cs []map[string]string
	for i := 0; i < structural; i++ {
		cs = append(cs, map[string]string{"kind": "structural", "text": "x"})
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
		{"structural", cold(2, 0, 0, true), []string{"A", "B"}, "FAIL", "2/2", false},
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
