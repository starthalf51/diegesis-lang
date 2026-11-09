// scenereq writes each scene's render request (render_scene JSON, the input dieg render hands to the renderer) of an already loaded work (`dieg load <dir>`)
// to out-dir/<scene>.json and prints the sha256. It is used only to compare requests before and after a revision and pick the scenes to re-render.
// Usage: go run ./examples/idea-to-novel/tools/scenereq <work> <out-dir> <scene...>
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: scenereq <work> <out-dir> <scene...>")
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, s := range os.Args[3:] {
		b, err := pg.RenderScene(ctx, pool, os.Args[1], s)
		if err != nil {
			fmt.Fprintln(os.Stderr, s, err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(os.Args[2], s+".json"), b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s %x\n", s, sha256.Sum256(b))
	}
}
