package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

//go:embed demo/*.dieg.md
var demoFS embed.FS

// cmdDemo runs `dieg demo [--scene <id>]`: loads the embedded story and prints
// lint warnings, who knows what, and the world-state digest. No model calls.
func cmdDemo(args []string, out io.Writer) int {
	scene := "1.3"
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--scene":
		scene = args[1]
	default:
		return usage()
	}
	if err := runDemo(scene, out); err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return 0
}

func runDemo(scene string, out io.Writer) error {
	dir, err := os.MkdirTemp("", "dieg-demo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	ents, _ := demoFS.ReadDir("demo")
	for _, e := range ents {
		b, _ := demoFS.ReadFile("demo/" + e.Name())
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			return err
		}
	}
	work, err := load(dir, io.Discard) // loading the same work ID again makes pg.Load delete it and reinsert
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	fmt.Fprintln(out, "== Consistency warnings ==")
	ds, err := pg.Lint(ctx, pool, work, rules.FS)
	if err != nil {
		return err
	}
	for _, d := range ds {
		fmt.Fprintln(out, d)
	}

	fmt.Fprintf(out, "\n== Who knows what (scene %s) ==\n", scene)
	// Drop facts held back from the reader (withhold) from the beliefs gained before this scene.
	rows, err := pool.Query(ctx, `
	  SELECT who, txt, yes FROM (
	  SELECT DISTINCT ON (b.entity_id, z.base) render_name($1, b.entity_id, 'ko') AS who, render_fact_text($1, z.base) AS txt, z.val = b.value AS yes,
	         point_ord($1, b.from_pt) AS o
	    FROM scene s JOIN believes b ON b.work_id = s.work_id
	    JOIN entity e ON e.work_id = b.work_id AND e.id = b.entity_id AND e.kind = 'character'
	   CROSS JOIN LATERAL prop_norm($1, b.prop_id) z
	   WHERE s.work_id = $1 AND lint_short(s.id) = $2 AND point_ord($1, b.from_pt) < s.discourse_ord
	     AND NOT EXISTS (SELECT 1 FROM withhold wh CROSS JOIN LATERAL prop_norm($1, wh.prop_id) w
	                      WHERE wh.work_id = $1 AND w.base = z.base
	                        AND NOT EXISTS (SELECT 1 FROM reader_state($1, s.discourse_ord - 1) r WHERE r.fact_id = z.base))
	   ORDER BY b.entity_id, z.base, point_ord($1, b.from_pt) DESC) y ORDER BY who, txt`, work, scene)
	if err != nil {
		return err
	}
	for rows.Next() {
		var who, text string
		var yes bool
		if err := rows.Scan(&who, &text, &yes); err != nil {
			return err
		}
		v := "believes it is true"
		if !yes {
			v = "believes it is false"
		}
		fmt.Fprintf(out, "- %s: 「%s」 — %s\n", who, text, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n== World state for scene %s ==\n", scene)
	text, _, err := pg.Digest(ctx, pool, work, scene)
	if err != nil {
		return err
	}
	fmt.Fprint(out, text)
	return nil
}
