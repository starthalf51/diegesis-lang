package pg_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// scenes builds n scenes; first/last use the "bookend" cast (hall, a+b, p), the others use mid(i).
func structBody(n int, first, last string, mid func(i int) string) string {
	var sb strings.Builder
	sb.WriteString("place hall \"H\"\nplace yard \"Y\"\nfact q \"Q\" true\n")
	for i := 1; i <= n; i++ {
		body := mid(i)
		if i == 1 {
			body = first
		} else if i == n {
			body = last
		}
		after := ""
		if i > 1 {
			after = fmt.Sprintf("  when after %d\n", i-1)
		}
		fmt.Fprintf(&sb, "scene %d at %s\n%s%s", i, strings.SplitN(body, "|", 2)[0], after, strings.SplitN(body, "|", 2)[1])
	}
	return sb.String()
}

const hallAB = "hall|  enter a, b\n  a tells b: p\n"
const yardCM = "yard|  enter c, m\n  c tells m: q\n"

func structure(t *testing.T, body string) (map[string]float64, string, []pg.Match) {
	t.Helper()
	pool := open(t)
	id := compileLoad(t, pool, body)
	rows, err := pool.Query(context.Background(), `SELECT term, value::float8 FROM metric_structure($1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]float64{}
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		got[k] = v
	}
	ms, err := pg.Patterns(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return got, id, ms
}

func TestStructureBookend(t *testing.T) {
	// 10 scenes: the first and last share place, cast and fact; the middle is a different world → positive, pattern matches.
	got, id, ms := structure(t, structBody(10, hallAB, hallAB, func(int) string { return yardCM }))
	if got["bookend"] <= 0.2 {
		t.Errorf("bookend = %v, want > 0.2", got)
	}
	if m := matches(ms, "bookend", id); len(m) != 1 {
		t.Errorf("bookend pattern matches = %v, want 1", m)
	}
	// Control: the ends differ from each other and the middle repeats the opening → <= 0, no match.
	got, id, ms = structure(t, structBody(10, hallAB, yardCM, func(int) string { return hallAB }))
	if v, ok := got["bookend"]; !ok || v > 0 {
		t.Errorf("control bookend = %v, want a row <= 0", got)
	}
	if m := matches(ms, "bookend", id); len(m) != 0 {
		t.Errorf("control matched: %v", m)
	}
}

func TestStructureSmall(t *testing.T) {
	// Two scenes: windows of 1 do not overlap → computed (same ends → Jaccard 1).
	got, _, _ := structure(t, structBody(2, hallAB, hallAB, func(int) string { return yardCM }))
	if _, ok := got["bookend"]; !ok {
		t.Errorf("2 scenes: no bookend row: %v", got)
	}
	// One scene: the front and back windows are the same scene → no row.
	got, _, _ = structure(t, structBody(1, hallAB, hallAB, func(int) string { return yardCM }))
	if _, ok := got["bookend"]; ok {
		t.Errorf("1 scene: bookend row present: %v", got)
	}
}

func TestStructurePayoffRatio(t *testing.T) {
	got, _, _ := structure(t, `scene 1
  enter a
  setup s1 promise: p
  setup s2 promise: p
scene 2
  when after 1
  enter a
  payoff s1
`)
	if got["payoff_ratio"] != 0.5 {
		t.Errorf("payoff_ratio = %v, want 0.5", got)
	}
	got, _, _ = structure(t, "scene 1\n  enter a\n")
	if _, ok := got["payoff_ratio"]; ok {
		t.Errorf("no setups but payoff_ratio row: %v", got)
	}
}
