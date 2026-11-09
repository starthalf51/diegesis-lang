package dash_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/db"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// asRO runs q with SET LOCAL ROLE sl_ro inside a transaction and rolls back. Returns the row count and the error.
func asRO(t *testing.T, pool *pgxpool.Pool, q string) (int, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE sl_ro"); err != nil {
		t.Fatal(err)
	}
	r, err := tx.Query(ctx, q, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return 0, err
	}
	n := 0
	for r.Next() {
		n++
	}
	r.Close()
	return n, r.Err()
}

func denied(t *testing.T, pool *pgxpool.Pool, what, q string) {
	t.Helper()
	if _, err := asRO(t, pool, q); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("%s: sl_ro should be blocked, got err=%v", what, err)
	}
}

// List of dashboard functions (same as 016's GRANT).
var dashFuncs = []string{"metric_presence", "metric_asymmetry", "metric_open_setups", "metric_active_deceptions",
	"metric_tension", "metric_anachrony", "metric_pace", "graph_nodes", "graph_edges", "metric_ords"}

func TestReadOnlyRole(t *testing.T) {
	d := loadDashboard(t)
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mig, err := fs.Sub(db.FS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Migrate(ctx, pool, mig); err != nil {
		t.Fatal(err)
	}

	// Every panel SQL runs as sl_ro.
	for _, p := range d.Panels {
		for _, tg := range p.Targets {
			n, err := asRO(t, pool, panelSQL(t, tg.RawSQL, "macbeth"))
			if err != nil {
				t.Errorf("panel %d %q as sl_ro: %v", p.ID, p.Title, err)
			}
			if n == 0 && (p.ID == 1 || p.ID == 2 || p.ID == 5 || p.ID == 8) {
				t.Errorf("panel %d %q as sl_ro: 0 rows", p.ID, p.Title)
			}
		}
	}

	// The reviewer's exploit: close $c$ with graph_rows' q and slip in SQL with sl privileges. There is no execute privilege now.
	denied(t, pool, "graph_rows injection",
		`SELECT graph_rows('g_macbeth', $q$RETURN 1 $c$, $1) AS (a agtype) WHERE set_config('x.who', current_user, true) IS NOT NULL --$q$, '[1]'::jsonb)`)
	denied(t, pool, "graph_name", `SELECT graph_name('macbeth')`)
	denied(t, pool, "derive (write)", `SELECT derive('macbeth')`)
	denied(t, pool, "CREATE TABLE", `CREATE TABLE ro_probe(x int)`)
	denied(t, pool, "CREATE TEMP TABLE", `CREATE TEMP TABLE ro_probe(x int)`)
	denied(t, pool, "INSERT", `INSERT INTO metric_weights VALUES ('ro_probe', 1)`)
	denied(t, pool, "AGE cypher", `SELECT * FROM ag_catalog.cypher('g_macbeth', $c$ MATCH (n) RETURN n $c$) AS (a ag_catalog.agtype)`)
	// A multi-statement injection without quotes is blocked at the write too.
	denied(t, pool, "panel multi-statement injection", `SELECT * FROM metric_presence('x'); CREATE TABLE ro_probe(x int); --')`)

	// Even if an injection string is put into a permitted function, it is used only as a value: 0 rows, no side effects.
	for _, f := range dashFuncs {
		n, err := asRO(t, pool, "SELECT * FROM "+f+"($i$macbeth') UNION ALL SELECT NULL --$i$)")
		if err != nil || n != 0 {
			t.Errorf("%s injection: n=%d err=%v", f, n, err)
		}
	}

	// Structure check: every public function sl_ro can execute is INVOKER and uses no dynamic SQL (EXECUTE).
	r, err := pool.Query(ctx, `SELECT p.oid::regprocedure::text FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND has_function_privilege('sl_ro', p.oid, 'EXECUTE')
		  AND (p.prosecdef OR p.prosrc ~* '\mexecute\M' OR p.proname IN ('graph_rows', 'graph_name'))`)
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		var f string
		r.Scan(&f)
		t.Errorf("dangerous function executable by sl_ro: %s", f)
	}
	r.Close()
	var defs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.prosecdef`).Scan(&defs); err != nil || defs != 0 {
		t.Errorf("%d SECURITY DEFINER functions in public (err=%v), want 0", defs, err)
	}
}
