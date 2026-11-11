package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr runs f with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	f()
	os.Stderr = old
	w.Close()
	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestLintRulesFlag(t *testing.T) {
	empty := t.TempDir()
	for _, c := range []struct{ rules, want string }{
		{filepath.Join(empty, "missing"), "--rules"},
		{empty, "no rules"},
	} {
		var code int
		msg := captureStderr(t, func() { code = cmdLint([]string{"somework", "--rules", c.rules}) })
		if code != 1 || !strings.Contains(msg, c.want) {
			t.Errorf("rules=%s: code %d, stderr %q, want %q", c.rules, code, msg, c.want)
		}
	}
}
