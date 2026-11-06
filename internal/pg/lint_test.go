package pg_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"diegesis/internal/pg"
	"diegesis/internal/sema"
	"diegesis/internal/syntax"
)

// cast 는 규칙 픽스처의 공통 선언. WK 는 고유한 작품 ID 로 바뀐다.
const cast = `module t.s
work WK "T"
character a "A"
character b "B"
character c "C"
character m "M"
character o "O"
  knows oracle
fact p "P" true
fact np = not p
artifact letter "L"
  contains p
`

// compileLoad 는 cast + body 를 실제 경로(syntax → sema → Load)로 적재하고 작품 ID 를 돌려준다.
func compileLoad(t *testing.T, pool *pgxpool.Pool, body string) string {
	t.Helper()
	id := fmt.Sprintf("t_%d", rand.Int63())
	f, diags := syntax.ParseFile("t.dieg", []byte(strings.ReplaceAll(cast+body, "WK", id)))
	w, ds := sema.Build([]*syntax.File{f})
	for _, d := range append(diags, ds...) {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	return id
}

func lint(t *testing.T, pool *pgxpool.Pool, work string) []pg.Diagnostic {
	t.Helper()
	ds, err := pg.Lint(context.Background(), pool, work, os.DirFS("../../rules"))
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// of 는 한 규칙의 진단을 "대상(작품 ID 뺀 것)/등급[/waived]" 로 모은다.
func of(ds []pg.Diagnostic, rule, work string) []string {
	var out []string
	for _, d := range ds {
		if d.Rule == rule {
			s := strings.TrimPrefix(d.TargetID, work+"/") + "/" + d.Severity
			if d.Waived {
				s += "/waived"
			}
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

func TestRules(t *testing.T) {
	rs, err := pg.Rules(os.DirFS("../../rules"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range rs {
		ids = append(ids, r.ID)
		if r.Summary == "" {
			t.Errorf("%s: no summary", r.Path)
		}
	}
	slices.Sort(ids)
	want := []string{"account-variance", "attribute-conflict", "because-unbelieved", "belief-contradiction", "dead-actor", "inferred-deception", "info-flow",
		"letter-unknown-info", "object-location", "pattern-missing", "payoff-without-setup", "reveal-false", "time-cycle", "unpaid-promise", "undelivered-message"}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("rules = %v, want %v", ids, want)
	}
}

func TestWaiverExactTarget(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  #! allow info-flow: "rumour"
  a tells b: p
  #! allow dead-actor: "other rule"
  a tells b: p
  a tells b: p
`)
	ds := lint(t, pool, id)
	got := of(ds, "info-flow", id)
	want := []string{"t.s/1#a1/warn/waived", "t.s/1#a2/warn", "t.s/1#a3/warn"}
	if !slices.Equal(got, want) {
		t.Errorf("info-flow = %v, want %v", got, want)
	}
	for _, d := range ds {
		if d.Waived && d.Reason != "rumour" {
			t.Errorf("waived %s reason %q, want rumour", d.TargetID, d.Reason)
		}
	}
}

func TestLintReplacesRun(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  when after 2
  enter a, b
  a tells b: p
scene 2
  when after 1
`)
	first, second := lint(t, pool, id), lint(t, pool, id)
	if len(first) != len(second) || len(first) < 2 {
		t.Fatalf("lint twice: %d then %d diagnostics", len(first), len(second))
	}
	var lintRuns, derive int
	pool.QueryRow(context.Background(), `SELECT count(DISTINCT run_id) FILTER (WHERE run_id <> 'derive'),
		count(*) FILTER (WHERE run_id = 'derive') FROM diagnostic WHERE work_id = $1`, id).Scan(&lintRuns, &derive)
	if lintRuns != 1 || derive != 1 {
		t.Errorf("lint runs = %d (want 1), derive rows = %d (want 1, kept from load)", lintRuns, derive)
	}
}

// TestEveryDiagnosticHasSrc: 규칙마다 위반 픽스처의 모든 진단에 file:line 이 있어야 한다(Review Focus 4).
func TestEveryDiagnosticHasSrc(t *testing.T) {
	pool := open(t)
	seen := map[string]bool{}
	for rule, fx := range fixtures {
		id := compileLoad(t, pool, fx.bad)
		for _, d := range lint(t, pool, id) {
			seen[d.Rule] = true
			if d.File == "" || d.Line <= 0 {
				t.Errorf("%s fixture: %s %s has no source position", rule, d.Rule, d.TargetID)
			}
		}
	}
	for rule := range fixtures {
		if !seen[rule] {
			t.Errorf("no diagnostic from %s", rule)
		}
	}
}

// TestMacbethGoldenLint: 골든 맥베스(1막 일부)의 억제되지 않은 warn 은 아래 셋뿐이다. 이유는 task-6 보고서.
func TestMacbethGoldenLint(t *testing.T) {
	pool := open(t)
	w, id := golden(t)
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range lint(t, pool, id) {
		t.Log(d)
		if d.Severity == "warn" && !d.Waived {
			got = append(got, d.Rule+" "+strings.TrimPrefix(d.TargetID, id+"/"))
		}
	}
	slices.Sort(got)
	want := []string{
		"info-flow macbeth.act1/1.2#a2",            // ross 가 코더의 반역을 어디서 알았는지 포팅에 없다(무대 밖 전투)
		"info-flow macbeth.act1/1.3#a5",            // 맥베스가 코더 영주가 살아 있다는 걸 어디서 알았는지 없다(극 이전 상식)
		"unpaid-promise macbeth.act1/banquo_kings", // 회수(4.1 왕들의 행렬)는 골든 범위 밖
	}
	if !slices.Equal(got, want) {
		t.Errorf("unwaived warns:\n got %q\nwant %q", got, want)
	}
}
