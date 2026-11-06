package render

import (
	"errors"
	"strings"
	"testing"
)

func TestMeterRecordsEveryCall(t *testing.T) {
	var buf strings.Builder
	f := &fakeCompleter{replies: []Result{
		{Text: "a", StopReason: "end", Usage: Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}},
		{Text: "b", StopReason: "end", Usage: Usage{Input: 10, Output: 20}},
	}}
	m := NewMeter(f, &buf)
	for i := 0; i < 2; i++ {
		if _, err := m.Complete(ctx, "s", "u"); err != nil {
			t.Fatal(err)
		}
	}
	f.err = errors.New("boom")
	if _, err := m.Complete(ctx, "s", "u"); err == nil {
		t.Fatal("want error")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], `"cache_write":4`) || !strings.Contains(lines[0], `"stop":"end"`) {
		t.Errorf("line 0: %s", lines[0])
	}
	if !strings.Contains(lines[2], `"err":"boom"`) {
		t.Errorf("error line lacks err: %s", lines[2])
	}
	if got := m.Total(); got != (Usage{Input: 11, Output: 22, CacheRead: 3, CacheWrite: 4}) {
		t.Errorf("total %+v", got)
	}
}

func TestMeterLabels(t *testing.T) {
	var buf strings.Builder
	f := &fakeCompleter{replies: []Result{{Usage: Usage{Input: 1}}, {Usage: Usage{Input: 2}}}}
	m := NewMeter(f, &buf)
	m.Label("code").Complete(ctx, "s", "u")
	m.Label("render").Complete(ctx, "s", "u")
	out := buf.String()
	if !strings.Contains(out, `"label":"code"`) || !strings.Contains(out, `"label":"render"`) {
		t.Errorf("labels missing: %s", out)
	}
	if m.Total().Input != 3 {
		t.Errorf("total %+v", m.Total())
	}
}
