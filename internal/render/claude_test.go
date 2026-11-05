package render

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestCost(t *testing.T) {
	got := Cost(Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000})
	if math.Abs(got-(4.00+20.00+0.20+5.00)) > 1e-9 {
		t.Fatal(got)
	}
}

func TestParams(t *testing.T) {
	p := NewClaude().Params("SYS", "USER")
	if p.Model != "claude-opus-5-5" || p.MaxTokens != 16000 {
		t.Fatal(p.Model, p.MaxTokens)
	}
	if len(p.System) != 1 || p.System[0].Text != "SYS" {
		t.Fatal("system")
	}
	b, _ := json.Marshal(p)
	for _, want := range []string{`"effort":"high"`, `"cache_control"`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("missing %s", want)
		}
	}
	for _, bad := range []string{`"temperature"`, `"thinking"`, `"top_p"`, `"top_k"`} {
		if bytes.Contains(b, []byte(bad)) {
			t.Errorf("must not send %s", bad)
		}
	}
}
