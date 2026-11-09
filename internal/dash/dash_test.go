// Package dash_test checks that the panel SQL of the Grafana dashboard runs in PG without errors.
package dash_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

type dashboard struct {
	Panels []struct {
		ID      int    `json:"id"`
		Title   string `json:"title"`
		Targets []struct {
			RawSQL string `json:"rawSql"`
		} `json:"targets"`
	} `json:"panels"`
	Templating struct {
		List []struct {
			Name  string `json:"name"`
			Query string `json:"query"`
		} `json:"list"`
	} `json:"templating"`
}

const workVar = "${work:sqlstring}"

// sqlstring imitates Grafana's ${work:sqlstring} substitution: replace ' with '' and wrap in single quotes.
func sqlstring(work string) string { return "'" + strings.ReplaceAll(work, "'", "''") + "'" }

// panelSQL substitutes work for ${work:sqlstring} in the panel SQL. Any other $ interpolation is a failure.
func panelSQL(t *testing.T, raw, work string) string {
	t.Helper()
	if strings.Count(raw, "$") != strings.Count(raw, workVar) {
		t.Errorf("%q: interpolation other than ${work:sqlstring}", raw)
	}
	return strings.ReplaceAll(raw, workVar, sqlstring(work))
}

func loadDashboard(t *testing.T) dashboard {
	t.Helper()
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN is required")
	}
	b, err := os.ReadFile("../../deploy/grafana/dashboards/diegesis.json")
	if err != nil {
		t.Fatal(err)
	}
	var d dashboard
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// run substitutes work for ${work:sqlstring} in every panel SQL, runs it, and returns the row count per panel id.
func run(t *testing.T, d dashboard, work string) map[int]int {
	t.Helper()
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows := map[int]int{}
	for _, p := range d.Panels {
		for _, tg := range p.Targets {
			q := panelSQL(t, tg.RawSQL, work)
			r, err := pool.Query(ctx, q, pgx.QueryExecModeSimpleProtocol)
			if err != nil {
				t.Errorf("panel %d %q: %v", p.ID, p.Title, err)
				continue
			}
			n := 0
			for r.Next() {
				n++
			}
			if err := r.Err(); err != nil {
				t.Errorf("panel %d %q: %v", p.ID, p.Title, err)
			}
			r.Close()
			rows[p.ID] += n
		}
	}
	return rows
}

func TestDashboardQueries(t *testing.T) {
	d := loadDashboard(t)
	if len(d.Panels) != 10 {
		t.Fatalf("%d panels, want 10", len(d.Panels))
	}
	if len(d.Templating.List) != 1 || d.Templating.List[0].Name != "work" ||
		d.Templating.List[0].Query != "SELECT id FROM work ORDER BY id" {
		t.Errorf("variable work differs: %+v", d.Templating.List)
	}
	got := run(t, d, "macbeth")
	for _, id := range []int{1, 2, 5, 8} {
		if got[id] < 1 {
			t.Errorf("macbeth panel %d: 0 rows (%v)", id, got)
		}
	}
	t.Logf("macbeth rows: %v", got)
	// A nonexistent work (empty work): 0 errors.
	run(t, d, "no-such-work")
	// A work name containing a quote: substitution escapes the quote, so 0 rows with no syntax error or injection.
	if got := run(t, d, "x'); CREATE TABLE dash_inj_probe(); --"); got[1] != 0 {
		t.Errorf("work name with quote: %v", got)
	}
}
