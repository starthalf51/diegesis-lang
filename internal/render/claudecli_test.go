package render

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

const cliSample = `{"type":"result","subtype":"success","is_error":false,"stop_reason":"end_turn","result":"hello","usage":{"input_tokens":2,"cache_creation_input_tokens":686,"cache_read_input_tokens":0,"output_tokens":520},"total_cost_usd":0.0159}`

func TestCLIArgs(t *testing.T) {
	c := NewClaudeCLI()
	want := []string{"-p", "--model", "claude-opus-5-5", "--effort", "high", "--tools", "", "--strict-mcp-config",
		"--setting-sources", "", "--no-session-persistence", "--output-format", "json", "--system-prompt", "SYS"}
	got := c.Args("SYS")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
	for _, a := range got {
		if a == "--bare" {
			t.Fatal("must not use --bare")
		}
	}
}

func TestParseCLIResult(t *testing.T) {
	r, err := parseCLIResult([]byte(cliSample))
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Text: "hello", StopReason: "end_turn", Usage: Usage{Input: 2, Output: 520, CacheRead: 0, CacheWrite: 686}}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}
}

func TestCLICompletePassesStdinAndTempDir(t *testing.T) {
	c := NewClaudeCLI()
	cwd, _ := os.Getwd()
	called := false
	c.Run = func(ctx context.Context, name string, args []string, stdin, dir string) ([]byte, error) {
		called = true
		if stdin != "USER" {
			t.Errorf("stdin = %q", stdin)
		}
		if dir == "" || dir == cwd {
			t.Errorf("dir = %q (cwd %q)", dir, cwd)
		}
		if !reflect.DeepEqual(args, c.Args("SYS")) {
			t.Errorf("args = %q", args)
		}
		return []byte(cliSample), nil
	}
	r, err := c.Complete(context.Background(), "SYS", "USER")
	if err != nil || !called || r.Text != "hello" {
		t.Fatalf("r=%+v err=%v called=%v", r, err, called)
	}
}

func TestCLICompleteErrors(t *testing.T) {
	run := func(out string, err error) (Result, error) {
		c := NewClaudeCLI()
		c.Run = func(context.Context, string, []string, string, string) ([]byte, error) { return []byte(out), err }
		return c.Complete(context.Background(), "s", "u")
	}
	if _, err := run("", errors.New("exit 1")); err == nil {
		t.Error("exec error: want error")
	}
	if _, err := run("not json", nil); err == nil || !strings.Contains(err.Error(), "not json") {
		t.Errorf("non-json: err = %v", err)
	}
	if _, err := run(`{"is_error":true,"subtype":"error_during_execution","result":"boom"}`, nil); err == nil {
		t.Error("is_error: want error")
	}
	r, err := run(`{"is_error":true,"stop_reason":"refusal","result":""}`, nil)
	if err != nil || r.StopReason != "refusal" {
		t.Errorf("refusal: r=%+v err=%v", r, err)
	}
}

func TestClaudeArgsModel(t *testing.T) {
	a := strings.Join(NewClaudeCLIWith("claude-sonnet-5-5", "medium").Args("s"), " ")
	if !strings.Contains(a, "--model claude-sonnet-5-5") || !strings.Contains(a, "--effort medium") {
		t.Fatal(a)
	}
	d := NewClaudeCLIWith("", "")
	if d.Model != "claude-opus-5-5" || d.Effort != "high" {
		t.Fatal(d.Model, d.Effort)
	}
}

func TestValidModel(t *testing.T) {
	if ValidModel("claude-sonet-5-5") == nil {
		t.Fatal("typo accepted")
	}
	for _, m := range KnownModels {
		if err := ValidModel(m); err != nil {
			t.Fatal(m, err)
		}
	}
}
