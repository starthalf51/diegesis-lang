package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const orientJSON = `{"name":"Hong","situation":"servant's son","world":"Joseon","want":"a name","next":"yes, hooked","dangling":["a red door","a limp"]}`

func runOrient(t *testing.T, resp []string, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	rf := filepath.Join(dir, "resp.json")
	b, _ := json.Marshal(resp)
	os.WriteFile(rf, b, 0o644)
	exe, _ := os.Executable()
	t.Setenv("READCHECK_CLAUDE", exe)
	t.Setenv("READCHECK_FAKE_RESP", rf)
	ep := filepath.Join(dir, "ep.md")
	os.WriteFile(ep, []byte("episode text"), 0o644)
	var out bytes.Buffer
	code := run(append(append([]string{"--orient"}, args...), ep), &out)
	return code, out.String(), ep
}

func TestOrientPlainAndFenced(t *testing.T) {
	for _, r := range []string{orientJSON, "```json\n" + orientJSON + "\n```"} {
		code, out, ep := runOrient(t, []string{r})
		if code != 0 {
			t.Fatalf("exit %d out %q", code, out)
		}
		want := "orient " + ep + " dangling=2\nname: Hong\nsituation: servant's son\nworld: Joseon\nwant: a name\n"
		if out != want {
			t.Errorf("out %q want %q", out, want)
		}
		b, err := os.ReadFile(ep + ".orient.json")
		if err != nil || !strings.Contains(string(b), `"dangling"`) {
			t.Errorf("orient.json: %v %q", err, b)
		}
	}
}

func TestOrientRetryThenExit2(t *testing.T) {
	code, out, _ := runOrient(t, []string{"garbage", orientJSON})
	if code != 0 || !strings.Contains(out, "dangling=2") {
		t.Errorf("retry: exit %d out %q", code, out)
	}
	code, out, ep := runOrient(t, []string{"garbage", "still garbage"})
	if code != 2 {
		t.Errorf("exit %d out %q", code, out)
	}
	if _, err := os.Stat(ep + ".orient.json"); err == nil {
		t.Error("orient.json written on failure")
	}
}

func TestOrientPromptEnglish(t *testing.T) {
	for _, w := range []string{"first-time reader", "\"guess\"", "Answer in the language of the text.", "6. List every piece"} {
		if !strings.Contains(orientPrompt, w) {
			t.Errorf("prompt lacks %q", w)
		}
	}
}
