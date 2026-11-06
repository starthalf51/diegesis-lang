package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"diegesis/internal/pg"
)

// cmdPatterns runs `dieg patterns <work|dir> [--json]`: the whole pattern library on one work (a dir is
// built and loaded first). Prints pattern, target, span (discourse_ord), declared, data. Matches are
// reported, not judged: a declared pattern without a match is lint's pattern-missing.
func cmdPatterns(args []string) int {
	var target string
	asJSON := false
	for _, a := range args {
		switch {
		case a == "--json":
			asJSON = true
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" {
		return usage()
	}
	work := target
	if _, err := os.Stat(target); err == nil {
		if work, err = load(target, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer pool.Close()
	ms, err := pg.Patterns(ctx, pool, work)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	if asJSON {
		if ms == nil {
			ms = []pg.Match{}
		}
		b, _ := json.MarshalIndent(ms, "", "  ")
		fmt.Printf("%s\n", b)
		return 0
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PATTERN\tTARGET\tSPAN\tDECLARED\tDATA")
	for _, m := range ms {
		fmt.Fprintf(tw, "%s\t%s %s\t%s..%s\t%v\t%s\n", m.Pattern, m.TargetKind, strings.TrimPrefix(m.TargetID, work+"/"),
			ord(m.SpanFrom), ord(m.SpanTo), m.Declared, strings.ReplaceAll(string(m.Data), work+"/", ""))
	}
	tw.Flush()
	return 0
}

func ord(p *int) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprint(*p)
}
