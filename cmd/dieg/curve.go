package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

type curveRow struct {
	Module   string  `json:"module"`
	FirstOrd int     `json:"first_ord"`
	N        int     `json:"n"`
	Mean     float64 `json:"mean"`
	Valley   bool    `json:"valley"`
}

// Valleys flags values at or below mean - 1 standard deviation (population). Zero deviation: none.
func Valleys(xs []float64) []bool {
	out := make([]bool, len(xs))
	if len(xs) == 0 {
		return out
	}
	var sum, ss float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(ss / float64(len(xs)))
	// sd below float noise relative to the mean counts as flat (e.g. 0.1,0.1,0.1).
	if sd <= 1e-9*math.Max(1, math.Abs(mean)) {
		return out
	}
	for i, x := range xs {
		out[i] = x <= mean-sd
	}
	return out
}

func markValleys(rows []curveRow) []curveRow {
	xs := make([]float64, len(rows))
	for i, r := range rows {
		xs[i] = r.Mean
	}
	for i, v := range Valleys(xs) {
		rows[i].Valley = v
	}
	return rows
}

var chapterPart = regexp.MustCompile(`^(.*\.ch\d+)[a-z]+$`)

// groupChapters merges parts of one chapter (x.ch032a, x.ch032b -> x.ch032): n summed, mean n-weighted,
// first_ord the minimum, ordered by first_ord. Other names are kept as is.
func groupChapters(rows []curveRow) []curveRow {
	idx := map[string]int{}
	var out []curveRow
	var tot []float64
	for _, r := range rows {
		name := r.Module
		if m := chapterPart.FindStringSubmatch(name); m != nil {
			name = m[1]
		}
		i, ok := idx[name]
		if !ok {
			idx[name] = len(out)
			out = append(out, curveRow{Module: name, FirstOrd: r.FirstOrd})
			tot = append(tot, 0)
			i = len(out) - 1
		}
		out[i].FirstOrd = min(out[i].FirstOrd, r.FirstOrd)
		out[i].N += r.N
		tot[i] += r.Mean * float64(r.N)
	}
	for i := range out {
		if out[i].N > 0 {
			out[i].Mean = tot[i] / float64(out[i].N)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].FirstOrd < out[b].FirstOrd })
	return out
}

func writeCurve(w io.Writer, rows []curveRow, asJSON bool) {
	if asJSON {
		if rows == nil {
			rows = []curveRow{}
		}
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Fprintf(w, "%s\n", b)
		return
	}
	var vs []string
	for _, r := range rows {
		fmt.Fprintf(w, "%s %d %.2f", r.Module, r.N, r.Mean)
		if r.Valley {
			fmt.Fprint(w, "  valley")
			vs = append(vs, r.Module)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "valleys: %s\n", strings.Join(vs, ","))
}

// cmdCurve runs `dieg curve <work|dir> [--by module|scene] [--json]`: the engagement curve (migration 017)
// with valleys. --by module reports per chapter (parts merged).
func cmdCurve(args []string) int {
	var target string
	by, asJSON := "module", false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--json":
			asJSON = true
		case a == "--by" && i+1 < len(args):
			i++
			by = args[i]
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" || (by != "module" && by != "scene") {
		return usage()
	}
	work := target
	if _, err := os.Stat(target); err == nil {
		var err error
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
	q := `SELECT module, first_ord, n, mean::float8 FROM metric_module_engagement($1) ORDER BY first_ord`
	if by == "scene" {
		q = `SELECT scene, ord, 1, score::float8 FROM metric_engagement($1) ORDER BY ord`
	}
	rs, err := pool.Query(ctx, q, work)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer rs.Close()
	var rows []curveRow
	for rs.Next() {
		var r curveRow
		if err := rs.Scan(&r.Module, &r.FirstOrd, &r.N, &r.Mean); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	if by == "module" {
		rows = groupChapters(rows)
	}
	writeCurve(os.Stdout, markValleys(rows), asJSON)
	return 0
}
