package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// statsMetrics are the SQL functions of migrations 014 and 017 (all take the work id).
var statsMetrics = []string{"presence", "asymmetry", "open_setups", "active_deceptions", "tension", "anachrony", "pace", "novelty", "payoff", "drag", "engagement", "module_engagement"}

// cmdStats runs `dieg stats <work> [--metric name] [--json]`: prints migrations 014 and 017 metric_* functions
// (default: all) or graph_nodes / graph_edges. Computation is SQL; this only formats rows.
func cmdStats(args []string) int {
	var work, metric string
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--json":
			asJSON = true
		case a == "--metric" && i+1 < len(args):
			i++
			metric = strings.TrimPrefix(args[i], "metric_")
		case work == "" && !strings.HasPrefix(a, "-"):
			work = a
		default:
			return usage()
		}
	}
	if work == "" {
		return usage()
	}
	names := statsMetrics
	if metric != "" {
		names = []string{metric}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer pool.Close()
	all := map[string][]map[string]any{}
	for _, n := range names {
		fn := "metric_" + n
		switch {
		case n == "graph_nodes" || n == "graph_edges":
			fn = n
		case !contains(statsMetrics, n):
			fmt.Fprintf(os.Stderr, "dieg: unknown metric %q (want %s|graph_nodes|graph_edges)\n", n, strings.Join(statsMetrics, "|"))
			return 2
		}
		rows, err := pool.Query(ctx, `SELECT * FROM public.`+fn+`($1)`, work)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		var cols []string
		for _, fd := range rows.FieldDescriptions() {
			cols = append(cols, fd.Name)
		}
		out := []map[string]any{}
		for rows.Next() {
			vs, err := rows.Values()
			if err != nil {
				rows.Close()
				fmt.Fprintln(os.Stderr, "dieg:", err)
				return 1
			}
			m := map[string]any{}
			for i, c := range cols {
				m[c] = vs[i]
				if n, ok := vs[i].(pgtype.Numeric); ok {
					f, _ := n.Float64Value()
					m[c] = f.Float64
				}
			}
			out = append(out, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		all[n] = out
		if !asJSON {
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "# %s\n%s\n", fn, strings.ToUpper(strings.Join(cols, "\t")))
			for _, m := range out {
				cells := make([]string, len(cols))
				for i, c := range cols {
					cells[i] = cell(m[c], work)
				}
				fmt.Fprintln(tw, strings.Join(cells, "\t"))
			}
			tw.Flush()
		}
	}
	if asJSON {
		var v any = all
		if metric != "" {
			v = all[names[0]]
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Printf("%s\n", b)
	}
	return 0
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func cell(v any, work string) string {
	if v == nil {
		return "-"
	}
	return strings.ReplaceAll(fmt.Sprint(v), work+"/", "")
}
