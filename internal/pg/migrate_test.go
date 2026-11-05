package pg_test

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"net/url"
	"os"
	"testing"

	"story-lang/db"
	"story-lang/internal/pg"
)

func migrations(t *testing.T) fs.FS {
	t.Helper()
	mig, err := fs.Sub(db.FS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	return mig
}

func TestMigrateIdempotent(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mig := migrations(t)
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
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		have[n] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, e := range []string{"age", "vector", "btree_gist"} {
		if !have[e] {
			t.Errorf("확장 %s 없음", e)
		}
	}
}

// 빈 DB 에서 풀 하나로 Migrate 직후 AGE 가 바로 동작해야 한다(재연결 없이).
func TestMigrateFreshDBSamePoolAGE(t *testing.T) {
	dsn := os.Getenv("SL_PG_DSN")
	if dsn == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	ctx := context.Background()
	admin, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("sl_t1_%d", rand.Int63())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	t.Setenv("SL_PG_DSN", u.String())
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pg.Migrate(ctx, pool, migrations(t)); err != nil {
		t.Fatal(err)
	}
	g := fmt.Sprintf("g_%d", rand.Int63())
	pool.Exec(ctx, fmt.Sprintf(`SELECT drop_graph('%s', true)`, g)) // 있으면 삭제(없으면 오류 무시)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`SELECT create_graph('%s')`, g)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`SELECT drop_graph('%s', true)`, g)); err != nil {
		t.Fatal(err)
	}
}
