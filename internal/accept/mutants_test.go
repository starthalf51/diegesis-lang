package accept

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"story-lang/internal/ir"
	"story-lang/internal/mutate"
	"story-lang/internal/pg"
	"story-lang/internal/sema"
	"story-lang/internal/syntax"
	"story-lang/rules"
)

const fixture = `module t.s
work WK "T"
character a "A"
character b "B"
character d "D"
fact p "P" true
fact q "Q" true
scene 1
  enter a, b
  a sees p
  a tells b: p
  setup s promise: q
  a tells b: "nothing"
  a sees: alive(d)
  exit all
scene 2
  when after 1
  enter b, d
  b tells d: p
  payoff s
`

func open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
	}
	pool, err := pg.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func load(t *testing.T, pool *pgxpool.Pool, w *ir.Work) {
	t.Helper()
	id := w.Works[0].ID
	t.Cleanup(func() { drop(context.Background(), pool, id) })
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
}

func compileFixture(t *testing.T) *ir.Work {
	t.Helper()
	id := fmt.Sprintf("t_%d", rand.Int63())
	f, diags := syntax.ParseFile("t.sl", []byte(strings.ReplaceAll(fixture, "WK", id)))
	w, ds := sema.Build([]*syntax.File{f})
	for _, d := range append(diags, ds...) {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	return w
}

// 같은 작품을 다른 ID 로 적재하면 지문이 같고, 결함을 넣으면 달라진다.
func TestFingerprint(t *testing.T) {
	pool, ctx := open(t), context.Background()
	w := compileFixture(t)
	id := w.Works[0].ID
	load(t, pool, w)
	same := mutate.Rename(w, id+"~same")
	load(t, pool, same)
	dropped := mutate.Rename(mutate.Apply(w, mutate.Mutation{Op: "drop-tells", Target: id + "/t.s/1#a2"}), id+"~drop")
	load(t, pool, dropped)
	base, err := fingerprint(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	for name, other := range map[string]string{"renamed": id + "~same", "dropped": id + "~drop"} {
		fp, err := fingerprint(ctx, pool, other)
		if err != nil {
			t.Fatal(err)
		}
		if eq := base.equal(fp); eq != (name == "renamed") {
			t.Errorf("%s: equal = %v", name, eq)
		}
	}
}

func TestRunMutants(t *testing.T) {
	pool, ctx := open(t), context.Background()
	w := compileFixture(t)
	load(t, pool, w)
	ds, err := pg.Lint(ctx, pool, w.Works[0].ID, rules.FS)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := RunMutants(ctx, pool, w, ds, 1, 0, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Outcome{}
	for _, m := range ms {
		got[m.Mutation.Op+" "+strings.ReplaceAll(m.Mutation.Target, w.Works[0].ID+"/t.s/", "")] = m.Outcome
	}
	want := map[string]Outcome{
		"drop-tells 1#a2": Detected, "drop-enter b@1": Detected, "move-before 1#a2": Detected, "move-before 2#a1": Detected,
		"drop-setup s": Detected, "drop-payoff s": Detected, "flip-perceived 1#a1 p": Detected, "flip-perceived 1#a2 p": Detected,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("outcomes\n got %v\nwant %v", got, want)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM work WHERE id LIKE $1`, w.Works[0].ID+"~m%").Scan(&left); err != nil || left != 0 {
		t.Errorf("mutant works left behind: %d (%v)", left, err)
	}
}

// "뒤에 쓰는가" 는 PG precedes 로 정한다: 받은 시점보다 앞선 사용만 있으면 해당 없음.
func TestApplicableLaterUseOnly(t *testing.T) {
	pool, ctx := open(t), context.Background()
	w := compileFixture(t)
	load(t, pool, w)
	id := w.Works[0].ID
	for name, c := range map[string]struct {
		pair [2]string
		want bool
	}{
		"later use":   {[2]string{id + "/t.s/1#a2", id + "/t.s/2#a1"}, true},
		"earlier use": {[2]string{id + "/t.s/2#a1", id + "/t.s/1#a2"}, false},
	} {
		m := mutate.Mutation{Op: "drop-tells", Target: c.pair[0], Pairs: [][2]string{c.pair}}
		ok, _, err := applicable(ctx, pool, id, w, &m)
		if err != nil || ok != c.want {
			t.Errorf("%s: applicable = %v (%v), want %v", name, ok, err, c.want)
		}
	}
	m := mutate.Mutation{Op: "move-before", Target: id + "/t.s/2#a1", Uses: []string{id + "/t.s/p"}}
	if ok, _, err := applicable(ctx, pool, id, w, &m); !ok || err != nil || len(m.Before) != 1 || m.Before[0] != id+"/t.s/1#a2" {
		t.Errorf("move-before sources = %v (%v %v), want [1#a2] (b heard p there)", m.Before, ok, err)
	}
}
