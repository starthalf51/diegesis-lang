package render

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Meter wraps a Completer and writes one JSONL line per call (failures included).
type Meter struct {
	inner Completer
	label string
	s     *meterState
}

type meterState struct {
	mu    sync.Mutex
	w     io.Writer
	total Usage
}

func NewMeter(inner Completer, w io.Writer) *Meter {
	return &Meter{inner: inner, s: &meterState{w: w}}
}

// Label returns a copy that shares the recorder and total but tags its calls.
func (m *Meter) Label(l string) *Meter { return &Meter{inner: m.inner, label: l, s: m.s} }

func (m *Meter) Complete(ctx context.Context, system, user string) (Result, error) {
	start := time.Now()
	r, err := m.inner.Complete(ctx, system, user)
	rec := struct {
		T          string `json:"t"`
		Label      string `json:"label"`
		Input      int64  `json:"input"`
		Output     int64  `json:"output"`
		CacheRead  int64  `json:"cache_read"`
		CacheWrite int64  `json:"cache_write"`
		Stop       string `json:"stop"`
		Err        string `json:"err"`
		Ms         int64  `json:"ms"`
	}{start.UTC().Format(time.RFC3339), m.label, r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheWrite,
		r.StopReason, "", time.Since(start).Milliseconds()}
	if err != nil {
		rec.Err = err.Error()
	}
	b, _ := json.Marshal(rec)
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	m.s.w.Write(append(b, '\n'))
	m.s.total.Input += r.Usage.Input
	m.s.total.Output += r.Usage.Output
	m.s.total.CacheRead += r.Usage.CacheRead
	m.s.total.CacheWrite += r.Usage.CacheWrite
	return r, err
}

func (m *Meter) Total() Usage {
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	return m.s.total
}
