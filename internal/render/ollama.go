package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// OllamaPrefix marks a local model: "ollama:<name>" goes to an Ollama server instead of `claude -p`.
const OllamaPrefix = "ollama:"

// Ollama runs a local model through an Ollama server's /api/chat (OLLAMA_HOST, default localhost:11434).
type Ollama struct {
	Host, Model string
	Ctx         int // num_ctx; Ollama's small default would silently cut the long coding prompts
	Timeout     time.Duration
	Client      *http.Client
}

func NewOllama(model string) *Ollama {
	h := os.Getenv("OLLAMA_HOST")
	if h == "" {
		h = "localhost:11434"
	}
	if !strings.Contains(h, "://") {
		h = "http://" + h
	}
	n, err := strconv.Atoi(os.Getenv("DIEG_OLLAMA_CTX"))
	if err != nil || n <= 0 {
		n = 32768
	}
	return &Ollama{Host: strings.TrimRight(h, "/"), Model: strings.TrimPrefix(model, OllamaPrefix), Ctx: n, Timeout: 30 * time.Minute, Client: http.DefaultClient}
}

// NewCompleter picks the backend by model name: "ollama:<name>" is local, anything else is `claude -p`.
// effort applies only to claude models; it is ignored for Ollama models.
func NewCompleter(model, effort string) Completer {
	if strings.HasPrefix(model, OllamaPrefix) {
		return NewOllama(model)
	}
	return NewClaudeCLIWith(model, effort)
}

func (o *Ollama) Complete(ctx context.Context, system, user string) (Result, error) {
	body, err := json.Marshal(map[string]any{
		"model": o.Model, "stream": false,
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"options":  map[string]any{"num_ctx": o.Ctx},
	})
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("ollama: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("ollama: %s: %s", resp.Status, trunc(b))
	}
	var r struct {
		Message    struct{ Content string } `json:"message"`
		DoneReason string                   `json:"done_reason"`
		In         int64                    `json:"prompt_eval_count"`
		Out        int64                    `json:"eval_count"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return Result{}, fmt.Errorf("ollama: bad output: %v (output: %s)", err, trunc(b))
	}
	if r.In >= int64(o.Ctx) { // Ollama truncates an over-long prompt and still answers 200
		return Result{}, fmt.Errorf("ollama: prompt filled the whole context (%d tokens, num_ctx %d); raise DIEG_OLLAMA_CTX or shorten the input", r.In, o.Ctx)
	}
	stop := "end_turn"
	if r.DoneReason == "length" {
		stop = "max_tokens"
	}
	return Result{Text: r.Message.Content, StopReason: stop, Usage: Usage{Input: r.In, Output: r.Out}}, nil
}
