// Package dash_test 는 Grafana 대시보드의 패널 SQL 이 PG 에서 오류 없이 도는지 검사한다.
package dash_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"diegesis/internal/pg"
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

// run 은 모든 패널 SQL 의 $work 를 work 로 바꿔 실행하고 패널 id 별 행 수를 돌려준다.
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
			if strings.Contains(tg.RawSQL, "${") || strings.Count(tg.RawSQL, "$") != strings.Count(tg.RawSQL, "$work") {
				t.Errorf("panel %d %q: $work 외의 보간", p.ID, p.Title)
			}
			q := strings.ReplaceAll(tg.RawSQL, "$work", work)
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
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	b, err := os.ReadFile("../../deploy/grafana/dashboards/diegesis.json")
	if err != nil {
		t.Fatal(err)
	}
	var d dashboard
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Panels) != 10 {
		t.Fatalf("패널 %d 개, want 10", len(d.Panels))
	}
	if len(d.Templating.List) != 1 || d.Templating.List[0].Name != "work" ||
		d.Templating.List[0].Query != "SELECT id FROM work ORDER BY id" {
		t.Errorf("변수 work 가 다르다: %+v", d.Templating.List)
	}
	got := run(t, d, "macbeth")
	for _, id := range []int{1, 2, 5, 8} {
		if got[id] < 1 {
			t.Errorf("macbeth 패널 %d: 0행 (%v)", id, got)
		}
	}
	t.Logf("macbeth rows: %v", got)
	// 존재하지 않는 작품(빈 작품): 오류 0.
	run(t, d, "no-such-work")
}
