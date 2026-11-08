package main

import (
	"context"
	"os"
	"sync"
	"testing"

	"diegesis/internal/render"
)

func TestGuardStopsAtCap(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "calls")
	defer f.Close()
	var mu sync.Mutex
	used := 0
	g := &guard{inner: &fake{}, mu: &mu, used: &used, cap: 2, log: f, label: "t"}
	for i := 0; i < 2; i++ {
		if _, err := g.Complete(context.Background(), "", "x"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.Complete(context.Background(), "", "x"); err == nil || used != 2 {
		t.Fatalf("third call should be refused, used=%d err=%v", used, err)
	}
	var _ render.Completer = g
}
