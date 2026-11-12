package pg_test

import (
	"context"
	"testing"
)

// Four scenes in two modules: 1 new facts, 2 setup paid off, 3 nobody acts, 4 plain.
func TestEngagementFixture(t *testing.T) {
	pool := open(t)
	id := compileLoadFiles(t, pool, `fact q "Q" true
fact r "R" true
scene 1
  enter a, b
  a tells b: p
  setup s1 promise: r
scene 2
  when after 1
  enter a, b
  payoff s1
  a tells b: q
  a tells b: r
`, `module t.s2
import t.s
scene 3
  enter a
scene 4
  when after 3
  enter a, b
  a tells b: p
`)
	score := map[string]float64{}
	rows, err := pool.Query(context.Background(), `SELECT split_part(scene, '/', 3), score::float8 FROM metric_engagement($1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		var v float64
		if err := rows.Scan(&s, &v); err != nil {
			t.Fatal(err)
		}
		score[s] = v
	}
	rows.Close()
	t.Log(score)
	if len(score) != 4 {
		t.Fatalf("scenes = %v, want 4", score)
	}
	for _, s := range []string{"1", "3", "4"} {
		if score["2"] <= score[s] {
			t.Errorf("payoff scene %v not highest vs %s", score, s)
		}
	}
	for _, s := range []string{"1", "2", "4"} {
		if score["3"] >= score[s] {
			t.Errorf("actless scene %v not lowest vs %s", score, s)
		}
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM metric_module_engagement($1)`, id).Scan(&n); err != nil || n != 2 {
		t.Errorf("module rows = %d (%v), want 2", n, err)
	}
}

func TestEngagementEmpty(t *testing.T) {
	pool := open(t)
	for name, body := range map[string]string{"empty": "", "one": "scene 1\n  enter a\n"} {
		id := compileLoad(t, pool, body)
		for _, fn := range []string{"metric_novelty", "metric_payoff", "metric_drag", "metric_engagement", "metric_module_engagement"} {
			rows, err := pool.Query(context.Background(), `SELECT * FROM `+fn+`($1)`, id)
			if err != nil {
				t.Fatalf("%s %s: %v", name, fn, err)
			}
			n := 0
			for rows.Next() {
				n++
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("%s %s: %v", name, fn, err)
			}
			rows.Close()
			if want := map[string]int{"empty": 0, "one": 1}[name]; n != want {
				t.Errorf("%s %s rows = %d, want %d", name, fn, n, want)
			}
		}
	}
}
