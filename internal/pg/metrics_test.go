package pg_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// loadCorpus loads corpus/<work> under a unique work ID.
func loadCorpus(t *testing.T, pool *pgxpool.Pool, work string) string {
	t.Helper()
	w, id := corpusIR(t, work)
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	return id
}

// sceneOrd finds the narration position from the end part of a scene ID (e.g. "1.3").
func sceneOrd(t *testing.T, pool *pgxpool.Pool, work, suffix string) int {
	t.Helper()
	var o int
	if err := pool.QueryRow(context.Background(),
		`SELECT discourse_ord FROM scene WHERE work_id = $1 AND shown AND id LIKE '%/' || $2`, work, suffix).Scan(&o); err != nil {
		t.Fatalf("scene %s: %v", suffix, err)
	}
	return o
}

func TestMetricPresence(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "macbeth")
	rows, err := pool.Query(context.Background(),
		`SELECT lint_short(entity), acts FROM metric_presence($1) WHERE scene LIKE '%/1.3'`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	acts := map[string]int{}
	for rows.Next() {
		var e string
		var n int
		if err := rows.Scan(&e, &n); err != nil {
			t.Fatal(err)
		}
		acts[e] = n
	}
	for _, e := range []string{"macbeth", "banquo"} {
		if n, ok := acts[e]; !ok || n < 1 {
			t.Errorf("1.3 presence %v: %s missing or no acts", acts, e)
		}
	}
}

func TestMetricOpenSetups(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "macbeth")
	o41 := sceneOrd(t, pool, id, "4.1")
	rows, err := pool.Query(context.Background(), `SELECT ord, n FROM metric_open_setups($1) ORDER BY ord`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	peak := 0
	for rows.Next() {
		var o, n int
		if err := rows.Scan(&o, &n); err != nil {
			t.Fatal(err)
		}
		peak = max(peak, n)
		if o >= o41 && n != 0 {
			t.Errorf("open_setups(%d) = %d after 4.1 (ord %d), want 0", o, n, o41)
		}
	}
	if peak < 1 {
		t.Error("open_setups never above 0 before 4.1")
	}
}

func TestMetricActiveDeceptionsChibi(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "chibi")
	o := sceneOrd(t, pool, id, "45.129")
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT n FROM metric_active_deceptions($1) WHERE ord = $2`, id, o).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("active_deceptions at 45.129 = %d, want >= 1", n)
	}
}

func TestMetricAsymmetryOdyssey(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "odyssey")
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT coalesce(max(a.n), 0) FROM metric_asymmetry($1) a JOIN scene s ON s.work_id = $1 AND s.discourse_ord = a.ord AND s.shown
		 WHERE s.id LIKE '%/19.%' AND lint_short(a.entity) = 'penelope'`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("asymmetry penelope in book 19 = %d, want >= 1", n)
	}
}

// The time series has one row per scene and tension is within [0, sum of weights].
func TestMetricTensionRange(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "macbeth")
	var rows, bad int
	if err := pool.QueryRow(context.Background(), `SELECT count(*), count(*) FILTER (WHERE value < 0 OR value > (SELECT sum(weight) FROM metric_weights)) FROM metric_tension($1)`, id).Scan(&rows, &bad); err != nil {
		t.Fatal(err)
	}
	var scenes int
	if err := pool.QueryRow(context.Background(), `SELECT count(DISTINCT discourse_ord) FROM scene WHERE work_id = $1 AND shown`, id).Scan(&scenes); err != nil {
		t.Fatal(err)
	}
	if rows != scenes || rows == 0 || bad != 0 {
		t.Errorf("tension rows = %d (scenes %d), out of range = %d", rows, scenes, bad)
	}
}

var metricQueries = []string{
	"metric_presence", "metric_asymmetry", "metric_open_setups", "metric_active_deceptions", "metric_tension",
	"metric_anachrony", "metric_pace", "graph_nodes", "graph_edges",
}

// Review Focus 3: an empty work gives 0 rows with no error.
func TestMetricsEmptyWork(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, "")
	for _, f := range metricQueries {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM `+f+`($1)`, id).Scan(&n); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if n != 0 && f != "graph_nodes" { // even an empty work has the cast's characters as nodes
			t.Errorf("%s: %d rows, want 0", f, n)
		}
	}
}

// No error even for scenes with only offstage events or no time, and pace's span is NULL.
func TestMetricsNoTimeScene(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter m, o
  m tells o: p
event e "off stage"
  dies m
`)
	for _, f := range metricQueries {
		rows, err := pool.Query(context.Background(), `SELECT * FROM `+f+`($1)`, id)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	var span *int64
	if err := pool.QueryRow(context.Background(), `SELECT span_s FROM metric_pace($1)`, id).Scan(&span); err != nil {
		t.Fatal(err)
	}
	if span != nil {
		t.Errorf("pace span = %d, want NULL for a scene without time bounds", *span)
	}
}

func TestGraphColumns(t *testing.T) {
	pool := open(t)
	id := loadCorpus(t, pool, "macbeth")
	for f, want := range map[string][]string{
		"graph_nodes": {"id", "title", "mainStat"},
		"graph_edges": {"id", "source", "target", "mainStat"},
	} {
		rows, err := pool.Query(context.Background(), `SELECT * FROM `+f+`($1)`, id)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, fd := range rows.FieldDescriptions() {
			got = append(got, fd.Name)
		}
		n := 0
		for rows.Next() {
			n++
		}
		rows.Close()
		if !slices.Equal(got, want) {
			t.Errorf("%s columns = %v, want %v", f, got, want)
		}
		if n == 0 {
			t.Errorf("%s: no rows on macbeth", f)
		}
	}
}

// A point with no time constraint (solver default earliest = 0) does not appear in the time-warp graph. A work with no anchor gives 0 rows, and a work with an anchor has no fake 0.
func TestMetricAnachronyUnconstrained(t *testing.T) {
	pool := open(t)
	ctx := context.Background()
	mb := loadCorpus(t, pool, "macbeth")
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM metric_anachrony($1)`, mb).Scan(&n); err != nil || n != 0 {
		t.Errorf("macbeth anachrony rows = %d, err %v, want 0", n, err)
	}
	h := loadCorpus(t, pool, "holmes")
	var rows, fake int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE p.earliest = 0 AND p.latest IS NULL)
		FROM metric_anachrony($1) a JOIN scene s ON s.id = a.scene JOIN point p ON p.id = s.start_pt`, h).Scan(&rows, &fake); err != nil {
		t.Fatal(err)
	}
	if rows == 0 || fake != 0 {
		t.Errorf("holmes anachrony rows = %d, unconstrained = %d, want rows > 0 and 0", rows, fake)
	}
}
