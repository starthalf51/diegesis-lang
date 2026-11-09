package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCLI runs Claude through `claude -p` on the user's subscription login.
type ClaudeCLI struct {
	Bin, Model, Effort string
	Timeout            time.Duration
	Run                func(ctx context.Context, name string, args []string, stdin string, dir string) ([]byte, error)
}

func NewClaudeCLI() *ClaudeCLI {
	return &ClaudeCLI{Bin: "claude", Model: "claude-opus-5-5", Effort: "high", Timeout: 10 * time.Minute, Run: execRun}
}

// NewClaudeCLIWith is a CLI with model/effort overridden. An empty string is NewClaudeCLI's default.
func NewClaudeCLIWith(model, effort string) *ClaudeCLI {
	c := NewClaudeCLI()
	if model != "" {
		c.Model = model
	}
	if effort != "" {
		c.Effort = effort
	}
	return c
}

// KnownModels are the IDs --*-model accepts. A typo does not silently become the default; ValidModel blocks it.
var KnownModels = []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-1"}

func ValidModel(m string) error {
	if strings.HasPrefix(m, OllamaPrefix) && len(m) > len(OllamaPrefix) {
		return nil
	}
	for _, k := range KnownModels {
		if m == k {
			return nil
		}
	}
	return fmt.Errorf("unknown model %q (known: %s)", m, strings.Join(KnownModels, ", "))
}

func execRun(ctx context.Context, name string, args []string, stdin, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("%w: %s", err, trunc(errb.Bytes()))
	}
	return out.Bytes(), nil
}

// Args: no --bare (it skips keychain reads and breaks subscription login).
func (c *ClaudeCLI) Args(system string) []string {
	return []string{"-p", "--model", c.Model, "--effort", c.Effort, "--tools", "", "--strict-mcp-config",
		"--setting-sources", "", "--no-session-persistence", "--output-format", "json", "--system-prompt", system}
}

func (c *ClaudeCLI) Complete(ctx context.Context, system, user string) (Result, error) {
	dir, err := os.MkdirTemp("", "m0cli-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, err := c.Run(ctx, c.Bin, c.Args(system), user, dir)
	if err != nil {
		return Result{}, fmt.Errorf("claude cli: %w (output: %s)", err, trunc(out))
	}
	return parseCLIResult(out)
}

func trunc(b []byte) string {
	if len(b) > 300 {
		b = b[:300]
	}
	return string(b)
}

func parseCLIResult(out []byte) (Result, error) {
	var o struct {
		Result     string `json:"result"`
		StopReason string `json:"stop_reason"`
		IsError    bool   `json:"is_error"`
		Subtype    string `json:"subtype"`
		U          struct {
			In  int64 `json:"input_tokens"`
			Out int64 `json:"output_tokens"`
			CR  int64 `json:"cache_read_input_tokens"`
			CW  int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(out, &o); err != nil {
		return Result{}, fmt.Errorf("claude cli: bad output: %v (output: %s)", err, trunc(out))
	}
	if o.IsError && o.StopReason != "refusal" {
		return Result{}, fmt.Errorf("claude cli: is_error subtype=%q (output: %s)", o.Subtype, trunc(out))
	}
	return Result{Text: o.Result, StopReason: o.StopReason,
		Usage: Usage{Input: o.U.In, Output: o.U.Out, CacheRead: o.U.CR, CacheWrite: o.U.CW}}, nil
}
