package main

import (
	"bytes"
	"strings"
	"testing"
)

func runDemoT(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	if code := cmdDemo(nil, &b); code != 0 {
		t.Fatalf("cmdDemo exit %d\n%s", code, b.String())
	}
	return b.String()
}

func TestDemoOutput(t *testing.T) {
	out := runDemoT(t)
	for _, want := range []string{"== Consistency warnings ==", "== Who knows what (scene 1.3) ==", "== World state for scene 1.3 ==", "놋쇠 열쇠(key): at = 등대"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	warn := strings.SplitN(strings.SplitN(out, "== Consistency warnings ==\n", 2)[1], "\n== Who knows", 2)[0]
	for _, rule := range []string{"stays-gone", "relative-time-fact"} {
		ok := false
		for _, l := range strings.Split(warn, "\n") {
			ok = ok || strings.Contains(l, ": warn "+rule+":")
		}
		if !ok {
			t.Errorf("no %s warning line in warnings section:\n%s", rule, warn)
		}
	}
	if strings.Contains(warn, "waived") {
		t.Errorf("demo shows a waived line:\n%s", warn)
	}
	if strings.Contains(out, "등불 밑에 숨겼다") {
		t.Errorf("withheld fact leaked:\n%s", out)
	}
}

func TestDemoTwice(t *testing.T) {
	if a, b := runDemoT(t), runDemoT(t); a != b {
		t.Errorf("outputs differ:\n%s\n---\n%s", a, b)
	}
}
