package dash_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"diegesis/db"
	"diegesis/internal/pg"
)

// asRO 는 트랜잭션 안에서 SET LOCAL ROLE sl_ro 로 q 를 돌리고 되돌린다. 행 수와 오류를 돌려준다.
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
		t.Errorf("%s: sl_ro 가 막혀야 한다, got err=%v", what, err)
	}
}

// 대시보드 함수 목록(016 의 GRANT 와 같다).
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

	// 모든 패널 SQL 이 sl_ro 로 돈다.
	for _, p := range d.Panels {
		for _, tg := range p.Targets {
			n, err := asRO(t, pool, panelSQL(t, tg.RawSQL, "macbeth"))
			if err != nil {
				t.Errorf("panel %d %q as sl_ro: %v", p.ID, p.Title, err)
			}
			if n == 0 && (p.ID == 1 || p.ID == 2 || p.ID == 5 || p.ID == 8) {
				t.Errorf("panel %d %q as sl_ro: 0행", p.ID, p.Title)
			}
		}
	}

	// 리뷰어의 익스플로잇: graph_rows 의 q 로 $c$ 를 닫고 sl 권한으로 SQL 을 끼워 넣는다. 이제 실행권이 없다.
	denied(t, pool, "graph_rows 주입",
		`SELECT graph_rows('g_macbeth', $q$RETURN 1 $c$, $1) AS (a agtype) WHERE set_config('x.who', current_user, true) IS NOT NULL --$q$, '[1]'::jsonb)`)
	denied(t, pool, "graph_name", `SELECT graph_name('macbeth')`)
	denied(t, pool, "derive(쓰기)", `SELECT derive('macbeth')`)
	denied(t, pool, "CREATE TABLE", `CREATE TABLE ro_probe(x int)`)
	denied(t, pool, "CREATE TEMP TABLE", `CREATE TEMP TABLE ro_probe(x int)`)
	denied(t, pool, "INSERT", `INSERT INTO metric_weights VALUES ('ro_probe', 1)`)
	denied(t, pool, "AGE cypher", `SELECT * FROM ag_catalog.cypher('g_macbeth', $c$ MATCH (n) RETURN n $c$) AS (a ag_catalog.agtype)`)
	// 따옴표 없이 끼운 다중 문장 주입도 쓰기에서 막힌다.
	denied(t, pool, "패널 다중 문장 주입", `SELECT * FROM metric_presence('x'); CREATE TABLE ro_probe(x int); --')`)

	// 허가된 함수에 주입 문자열을 넣어도 값으로만 쓰인다: 0행, 부작용 없음.
	for _, f := range dashFuncs {
		n, err := asRO(t, pool, "SELECT * FROM "+f+"($i$macbeth') UNION ALL SELECT NULL --$i$)")
		if err != nil || n != 0 {
			t.Errorf("%s 주입: n=%d err=%v", f, n, err)
		}
	}

	// 구조 검사: sl_ro 가 실행할 수 있는 public 함수는 전부 INVOKER 이고 동적 SQL(EXECUTE)을 쓰지 않는다.
	r, err := pool.Query(ctx, `SELECT p.oid::regprocedure::text FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND has_function_privilege('sl_ro', p.oid, 'EXECUTE')
		  AND (p.prosecdef OR p.prosrc ~* '\mexecute\M' OR p.proname IN ('graph_rows', 'graph_name'))`)
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		var f string
		r.Scan(&f)
		t.Errorf("sl_ro 가 실행 가능한 위험 함수: %s", f)
	}
	r.Close()
	var defs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.prosecdef`).Scan(&defs); err != nil || defs != 0 {
		t.Errorf("public 의 SECURITY DEFINER 함수 %d 개 (err=%v), want 0", defs, err)
	}
}
