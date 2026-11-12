// curvecheck: compares `dieg curve <work> --json` (chapter level) with labelled slump/high chapters.
// Usage: curvecheck <work> <expect.yaml>   (env DIEG = path to the dieg binary, default "dieg")
//
// Ranking: chapters sorted by mean ascending, rank 1 = lowest (ties share the lower index order).
// N = number of chapters. "bottom 10%" = rank <= ceil(0.10*N). "top quartile" = rank >= N-ceil(0.25*N)+1.
// PASS = every valley chapter is in the bottom 10% AND at least 3 of the 4 high chapters are in the top quartile.
// Chapter labels match by the part after the last '.' (ch032 <-> mobydick.ch032).
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type row struct {
	Module string  `json:"module"`
	N      int     `json:"n"`
	Mean   float64 `json:"mean"`
}

type Expect struct {
	Valley []string `yaml:"valley"`
	High   []string `yaml:"high"`
}

func label(m string) string { return m[strings.LastIndex(m, ".")+1:] }

// Check returns the verdict text (last line PASS|FAIL) and the result.
func Check(rows []row, e Expect) (string, bool) {
	n := len(rows)
	sorted := append([]row(nil), rows...)
	sort.SliceStable(sorted, func(a, b int) bool { return sorted[a].Mean < sorted[b].Mean })
	rank := map[string]int{}
	for i, r := range sorted {
		rank[label(r.Module)] = i + 1
	}
	low := int(math.Ceil(0.10 * float64(n)))
	topFrom := n - int(math.Ceil(0.25*float64(n))) + 1
	var sb strings.Builder
	ok := true
	for _, v := range e.Valley {
		r, found := rank[v]
		in := found && r <= low
		ok = ok && in
		fmt.Fprintf(&sb, "valley %s: rank %d/%d (bottom 10%%? %s)\n", v, r, n, yn(in))
	}
	k := 0
	for _, h := range e.High {
		r := rank[h]
		if r >= topFrom && r > 0 {
			k++
		}
		fmt.Fprintf(&sb, "  high %s: rank %d/%d\n", h, r, n)
	}
	fmt.Fprintf(&sb, "high: %d/%d in top quartile\n", k, len(e.High))
	ok = ok && k >= 3
	if ok {
		sb.WriteString("PASS\n")
	} else {
		sb.WriteString("FAIL\n")
	}
	return sb.String(), ok
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: curvecheck <work> <expect.yaml>")
		os.Exit(2)
	}
	bin := os.Getenv("DIEG")
	if bin == "" {
		bin = "dieg"
	}
	b, err := exec.Command(bin, "curve", os.Args[1], "--json").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "curvecheck:", err)
		os.Exit(1)
	}
	var rows []row
	if err := json.Unmarshal(b, &rows); err != nil {
		fmt.Fprintln(os.Stderr, "curvecheck:", err)
		os.Exit(1)
	}
	raw, err := os.ReadFile(os.Args[2])
	var e Expect
	if err == nil {
		err = yaml.Unmarshal(raw, &e)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "curvecheck:", err)
		os.Exit(1)
	}
	out, _ := Check(rows, e)
	fmt.Print(out)
}
