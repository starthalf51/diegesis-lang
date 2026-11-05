package pg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"story-lang/internal/ir"
	"story-lang/internal/pg"
)

// open 은 마이그레이션까지 끝난 풀을 돌려준다.
func open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pg.Migrate(ctx, pool, migrations(t)); err != nil {
		t.Fatal(err)
	}
	return pool
}

// cleanup 은 테스트가 끝나면 작품 행과 그래프를 지운다.
func cleanup(t *testing.T, pool *pgxpool.Pool, work string) {
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM work WHERE id=$1`, work)
		pool.Exec(ctx, `SELECT drop_graph(name, true) FROM ag_graph WHERE name = graph_name($1)`, work)
	})
}

// golden 은 맥베스 골든 IR 을 고유한 작품 ID 로 바꿔 읽는다(다른 적재와 겹치지 않게).
func golden(t *testing.T) (*ir.Work, string) {
	t.Helper()
	b, err := os.ReadFile("../../testdata/golden/macbeth.ir.json")
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("t_mb_%d", rand.Int63())
	b = bytes.ReplaceAll(b, []byte(`"macbeth/`), []byte(`"`+id+`/`))
	b = bytes.ReplaceAll(b, []byte(`"macbeth"`), []byte(`"`+id+`"`))
	var w ir.Work
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return &w, id
}

var tables = []string{"module", "src", "entity", "group_member", "name", "role", "role_holder",
	"point", "time_edge", "scene", "presence", "act", "audience", "belief", "fact", "artifact",
	"setup", "payoff", "account", "parent", "distance", "reveal", "withhold", "directive", "waiver",
	"believes", "uses", "deception", "role_span", "diagnostic", "point_dist"}

func counts(t *testing.T, pool *pgxpool.Pool, work string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, tb := range append([]string{"work"}, tables...) {
		col := "work_id"
		if tb == "work" {
			col = "id"
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			fmt.Sprintf(`SELECT count(*) FROM public.%s WHERE %s=$1`, tb, col), work).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[tb] = n
	}
	return out
}

func TestLoadCountsMatchIR(t *testing.T) {
	pool := open(t)
	w, id := golden(t)
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	got := counts(t, pool, id)
	want := map[string]int{"work": len(w.Works), "module": len(w.Modules), "src": len(w.Src),
		"entity": len(w.Entities), "group_member": len(w.GroupMembers), "name": len(w.Names),
		"role": len(w.Roles), "role_holder": len(w.RoleHolders), "point": len(w.Points),
		"time_edge": len(w.TimeEdges), "scene": len(w.Scenes), "presence": len(w.Presence),
		"act": len(w.Acts), "audience": len(w.Audience), "belief": len(w.Beliefs), "fact": len(w.Facts),
		"artifact": len(w.Artifacts), "setup": len(w.Setups), "payoff": len(w.Payoffs),
		"account": len(w.Accounts), "parent": len(w.Parents), "distance": len(w.Distances),
		"reveal": len(w.Reveals), "withhold": len(w.Withholds), "directive": len(w.Directives),
		"waiver": len(w.Waivers)}
	for tb, n := range want {
		if got[tb] != n {
			t.Errorf("%s: %d rows, IR has %d", tb, got[tb], n)
		}
	}
	// src_id 0 → NULL, 빈 참조 → NULL
	var nullPlace int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM scene WHERE work_id=$1 AND thread_id IS NULL`, id).Scan(&nullPlace)
	if nullPlace != len(w.Scenes) {
		t.Errorf("empty thread_id should load as NULL: %d/%d", nullPlace, len(w.Scenes))
	}
}

func TestLoadReplace(t *testing.T) {
	pool := open(t)
	w, id := golden(t)
	cleanup(t, pool, id)
	ctx := context.Background()
	if err := pg.Load(ctx, pool, w); err != nil {
		t.Fatal(err)
	}
	first := counts(t, pool, id)
	if err := pg.Load(ctx, pool, w); err != nil {
		t.Fatal(err)
	}
	second := counts(t, pool, id)
	for tb, n := range first {
		if second[tb] != n {
			t.Errorf("%s: %d rows after reload, %d before", tb, second[tb], n)
		}
	}
	if first["work"] != 1 {
		t.Errorf("work rows = %d", first["work"])
	}
}

func TestLoadRejectsBadWork(t *testing.T) {
	pool := open(t)
	if err := pg.Load(context.Background(), pool, &ir.Work{}); err == nil {
		t.Fatal("IR without a works row must be rejected")
	}
}
