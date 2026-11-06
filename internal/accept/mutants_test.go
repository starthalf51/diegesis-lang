package accept

import (
	"context"
	"fmt"
	"io"
	iofs "io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"silmari/internal/ir"
	"silmari/internal/mutate"
	"silmari/internal/pg"
	"silmari/internal/sema"
	"silmari/internal/syntax"
	"silmari/rules"
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
	f, diags := syntax.ParseFile("t.sil", []byte(strings.ReplaceAll(fixture, "WK", id)))
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
	base, err := fingerprint(ctx, pool, id, w)
	if err != nil {
		t.Fatal(err)
	}
	for name, other := range map[string]string{"renamed": id + "~same", "dropped": id + "~drop"} {
		fp, err := fingerprint(ctx, pool, other, w)
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
	ms, err := RunMutants(ctx, pool, w, ds, nil, 1, 0, io.Discard)
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

// Task 11(오디세이): 들은 사람이 쓰기 전에 의심으로 거둔 믿음은 지워도 결함이 생기지 않는다(n/a).
// 의심 뒤 다시 들은 것으로 쓰므로 원래 들은 행위는 쓰는 시점에 출처가 아니다(lint_known 의 거둠과 같다).
func TestApplicableSkipsWithdrawnHearsay(t *testing.T) {
	pool, ctx := open(t), context.Background()
	id := fmt.Sprintf("t_%d", rand.Int63())
	src := strings.ReplaceAll(`module t.s
work WK "T"
character a "A"
character b "B"
character o "O"
fact p "P" true
scene 1
  enter a, b, o
  o tells a: p
  a doubts p
  o tells a: p
  a tells b: p
`, "WK", id)
	f, _ := syntax.ParseFile("t.sil", []byte(src))
	w, _ := sema.Build([]*syntax.File{f})
	load(t, pool, w)
	for name, c := range map[string]struct {
		pair [2]string
		want bool
	}{
		"withdrawn before the use": {[2]string{id + "/t.s/1#a1", id + "/t.s/1#a3"}, false},
		"live at the use":          {[2]string{id + "/t.s/1#a2", id + "/t.s/1#a3"}, true},
	} {
		m := mutate.Mutation{Op: "drop-tells", Target: c.pair[0], Pairs: [][2]string{c.pair}}
		ok, _, err := applicable(ctx, pool, id, w, &m)
		if err != nil || ok != c.want {
			t.Errorf("%s: applicable = %v (%v), want %v", name, ok, err, c.want)
		}
	}
}

// Review Focus 5: 새 진단이 변이 지점(Site: 그 행위·장면, 변이한 명제를 쓰는 행위, 변이한 시점을 지나는 간선)에
// 있어야 탐지다. 무관한 장면의 info-flow 는 기대 규칙이어도 탐지가 아니다.
func TestSiteLocalDetection(t *testing.T) {
	m := mutate.Mutation{Op: "drop-tells", Target: "w/t.s/1#a2", Expect: []string{"info-flow", "time-cycle"},
		Site: []string{"w/t.s/1#a2", "w/t.s/1", "w/t.s/2#a1", "w/t.s/1#a2.pt"}}
	d := func(rule, target, data string) pg.Diagnostic {
		return pg.Diagnostic{Rule: rule, Severity: "warn", TargetID: "w~m1/" + target, Data: []byte(data)}
	}
	for name, c := range map[string]struct {
		d    pg.Diagnostic
		want bool
	}{
		"using act":            {d("info-flow", "t.s/2#a1", "{}"), true},
		"unrelated scene":      {d("info-flow", "t.s/3#a1", "{}"), false},
		"unexpected rule":      {d("dead-actor", "t.s/2#a1", "{}"), false},
		"cycle through site":   {d("time-cycle", "t.s/9", `{"points": ["w~m1/t.s/0", "w~m1/t.s/1#a2.pt"]}`), true},
		"cycle elsewhere":      {d("time-cycle", "t.s/9", `{"points": ["w~m1/t.s/0", "w~m1/t.s/9.start"]}`), false},
		"no data (time-cycle)": {d("time-cycle", "t.s/9", ""), false},
	} {
		if got := hits(m, "w", "w~m1", c.d); got != c.want {
			t.Errorf("%s: local hit = %v, want %v", name, got, c.want)
		}
	}
}

// Task 2: 성경 5:27 므두셀라의 수명(969년)을 +50년 → 질의 methuselah-dies-am-1656 이 바뀐다 → 탐지(R1: 질의 탐지는 지점과 무관).
func TestShiftDurationGenesis(t *testing.T) {
	pool, ctx := open(t), context.Background()
	w := mutate.Rename(corpusWork(t, "../../corpus/bible"), fmt.Sprintf("bible_t%d", rand.Int63()))
	work := w.Works[0].ID
	b, err := os.ReadFile("../../corpus/bible/expect.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	load(t, pool, w)
	ds, err := pg.Lint(ctx, pool, work, rules.FS)
	if err != nil {
		t.Fatal(err)
	}
	h, err := newHarness(ctx, pool, w, ds, e.Queries)
	if err != nil {
		t.Fatal(err)
	}
	var m *mutate.Mutation
	var nTime int
	for _, x := range mutate.Generate(w, 1, 0) {
		if x.Op == "shift-duration" || x.Op == "swap-order" {
			nTime++
		}
		if x.Op == "shift-duration" && x.Target == work+"/bible.cast/methuselah.born "+work+"/bible.cast/methuselah.died" {
			m = &x
		}
	}
	if nTime < 5 {
		t.Errorf("time mutations on the genealogy = %d, want ≥ 5", nTime)
	}
	if m == nil {
		t.Fatal("no shift-duration on methuselah's lifespan (5:27)")
	}
	res, err := h.run(ctx, work+"~m1", *m)
	if err != nil {
		t.Fatal(err)
	}
	if (res.Outcome != Detected && res.Outcome != DetectedByQuery) || !slices.Contains(res.Queries, "methuselah-dies-am-1656") {
		t.Errorf("outcome %s, changed queries %v, want methuselah-dies-am-1656 changed", res.Outcome, res.Queries)
	}
}

func corpusWork(t *testing.T, dir string) *ir.Work {
	t.Helper()
	var fs []*syntax.File
	err := filepath.WalkDir(dir, func(p string, d iofs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !syntax.IsSource(p) {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		f, _ := syntax.ParseFile(filepath.ToSlash(rel), b)
		fs = append(fs, f)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	w, ds := sema.Build(fs)
	for _, d := range ds {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	return w
}

// 순서만 바꾼 변이(scene 2 를 scene 1 앞으로)는 believes·uses 행도 시점 경계도 그대로일 수 있다. 지문은 간선마다
// 두 끝점의 precedes 도 담아서 이런 변이를 동등으로 치지 않는다(동등 = 파생 결과가 같다 — 선후도 파생 결과다).
func TestFingerprintSeesOrder(t *testing.T) {
	pool, ctx := open(t), context.Background()
	w := compileFixture(t)
	id := w.Works[0].ID
	load(t, pool, w)
	m := mutate.Mutation{Op: "swap-order", Target: id + "/t.s/1.end " + id + "/t.s/2.start"}
	load(t, pool, mutate.Rename(mutate.Apply(w, m), id+"~swap"))
	base, err := fingerprint(ctx, pool, id, w)
	if err != nil {
		t.Fatal(err)
	}
	fp, err := fingerprint(ctx, pool, id+"~swap", w)
	if err != nil {
		t.Fatal(err)
	}
	if base.equal(fp) {
		t.Error("scene 2 moved before scene 1: fingerprint equal, want a precedes difference")
	}
}
