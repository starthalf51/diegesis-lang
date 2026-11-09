package render

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaComplete(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"message":{"role":"assistant","content":"code"},"done_reason":"length","prompt_eval_count":7,"eval_count":3}`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	c, ok := NewCompleter("ollama:qwen3.8:27b", "high").(*Ollama)
	if !ok {
		t.Fatal("ollama: prefix did not select Ollama")
	}
	r, err := c.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "code" || r.StopReason != "max_tokens" || r.Usage.Input != 7 || r.Usage.Output != 3 {
		t.Fatalf("result %+v", r)
	}
	if got["model"] != "qwen3.8:27b" || got["options"].(map[string]any)["num_ctx"] != float64(32768) || len(got["messages"].([]any)) != 2 {
		t.Fatalf("request %v", got)
	}
	if _, ok := NewCompleter("claude-haiku-5-5", "low").(*ClaudeCLI); !ok {
		t.Fatal("claude model did not select ClaudeCLI")
	}
	if ValidModel("ollama:") == nil || ValidModel("ollama:gemma4:26b") != nil {
		t.Fatal("ValidModel ollama prefix")
	}
}

func TestOllamaPromptFilledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"message":{"content":"cut"},"prompt_eval_count":32768,"eval_count":1}`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	_, err := NewOllama("ollama:m").Complete(context.Background(), "s", "u")
	if err == nil || !strings.Contains(err.Error(), "DIEG_OLLAMA_CTX") {
		t.Fatalf("err %v", err)
	}
}
