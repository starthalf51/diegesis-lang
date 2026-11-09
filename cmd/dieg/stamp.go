package main

import (
	"context"
	"fmt"
	"os"

	"github.com/starthalf51/diegesis-lang/internal/write"
)

// cmdStamp runs `dieg stamp <work-dir>`: loads <work-dir>/story into PG and upgrades the legacy (IR-only) .hash files of
// its rendered scenes to carry the PG state digest hash, so a later `dieg revise` can tell state-stale scenes. No model call.
func cmdStamp(args []string) int {
	if len(args) != 1 {
		return usage()
	}
	o := write.DefaultOpts()
	o.Out = args[0]
	ctx := context.Background()
	b, err := backend("styles", o.Backend)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	work, all, err := shownScenes(ctx, o.Out+"/story")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	o, closeDigest, err := withDigest(ctx, work, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer closeDigest()
	n, skipped, err := write.Stamp(ctx, all, b, o.Out, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	fmt.Printf("stamp: %d of %d scenes stamped (%d skipped: stored hash is not this scene's request) → %s\n", n, len(all), skipped, o.Out)
	return 0
}
