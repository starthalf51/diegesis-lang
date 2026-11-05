package pg_test

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"story-lang/db"
	"story-lang/internal/pg"
)

func TestMigrateIdempotent(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool.Close() }()
	mig, _ := fs.Sub(db.FS, "migrations")
	if _, err := pg.Migrate(ctx, pool, mig); err != nil {
		t.Fatal(err)
	}
	again, err := pg.Migrate(ctx, pool, mig)
	if err != nil || len(again) != 0 {
		t.Fatalf("두 번째 적용: applied=%v err=%v", again, err)
	}
	rows, err := pool.Query(ctx, `SELECT extname FROM pg_extension`)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for rows.Next() {
		var n string
		rows.Scan(&n)
		have[n] = true
	}
	for _, e := range []string{"age", "vector", "btree_gist"} {
		if !have[e] {
			t.Errorf("확장 %s 없음", e)
		}
	}
	// 새 연결에서 LOAD 없이 AGE 사용 가능 (ALTER DATABASE 는 새 세션에만 적용)
	pool.Close()
	if pool, err = pg.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `SELECT * FROM create_graph('t_mig')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `SELECT drop_graph('t_mig', true)`); err != nil {
		t.Fatal(err)
	}
}
