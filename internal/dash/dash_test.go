// Package dash_test 는 Grafana 대시보드의 패널 SQL 이 PG 에서 오류 없이 도는지 검사한다.
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

// sqlstring 은 Grafana 의 ${work:sqlstring} 치환을 흉내 낸다: ' 를 '' 로 바꾸고 작은따옴표로 감싼다.
func sqlstring(work string) string { return "'" + strings.ReplaceAll(work, "'", "''") + "'" }

// panelSQL 은 패널 SQL 의 ${work:sqlstring} 을 work 로 치환한다. 그 외의 $ 보간이 있으면 실패다.
func panelSQL(t *testing.T, raw, work string) string {
	t.Helper()
	if strings.Count(raw, "$") != strings.Count(raw, workVar) {
		t.Errorf("%q: ${work:sqlstring} 외의 보간", raw)
	}
	return strings.ReplaceAll(raw, workVar, sqlstring(work))
}

func loadDashboard(t *testing.T) dashboard {
	t.Helper()
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
	return d
}

// run 은 모든 패널 SQL 의 ${work:sqlstring} 을 work 로 바꿔 실행하고 패널 id 별 행 수를 돌려준다.
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
	// 따옴표가 든 작품 이름: 치환이 따옴표를 이스케이프하므로 문법 오류·주입 없이 0행.
	if got := run(t, d, "x'); CREATE TABLE dash_inj_probe(); --"); got[1] != 0 {
		t.Errorf("따옴표 작품 이름: %v", got)
	}
}
