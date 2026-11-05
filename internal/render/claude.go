package render

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type Usage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

type Result struct {
	Text, StopReason string
	Usage            Usage
}

// Completer is the seam later tasks use for both rendering and re-extraction.
type Completer interface {
	Complete(ctx context.Context, system, user string) (Result, error)
}

type Claude struct {
	Client    anthropic.Client
	Model     string
	Effort    string
	MaxTokens int64
}

func NewClaude() *Claude {
	return &Claude{Client: anthropic.NewClient(), Model: "claude-opus-5-5", Effort: "high", MaxTokens: 16000}
}

// Params sends no thinking/temperature/top_p/top_k and no prefill, by design.
func (c *Claude) Params(system, user string) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Model:        anthropic.Model(c.Model),
		MaxTokens:    c.MaxTokens,
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(c.Effort)},
		System:       []anthropic.TextBlockParam{{Text: system, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	}
}

// Complete returns refusal/max_tokens as a Result (caller judges); only transport/HTTP errors are errors.
func (c *Claude) Complete(ctx context.Context, system, user string) (Result, error) {
	resp, err := c.Client.Messages.New(ctx, c.Params(system, user))
	if err != nil {
		return Result{}, err
	}
	var sb strings.Builder
	for _, b := range resp.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	return Result{
		Text:       sb.String(),
		StopReason: string(resp.StopReason),
		Usage: Usage{
			Input: resp.Usage.InputTokens, Output: resp.Usage.OutputTokens,
			CacheRead: resp.Usage.CacheReadInputTokens, CacheWrite: resp.Usage.CacheCreationInputTokens,
		},
	}, nil
}

// Cost in USD; prices per 1M tokens (claude-opus-5-5, 2026-10-06).
func Cost(u Usage) float64 {
	return (4.00*float64(u.Input) + 20.00*float64(u.Output) + 0.20*float64(u.CacheRead) + 5.00*float64(u.CacheWrite)) / 1e6
}
