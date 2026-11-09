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

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/mutate"
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/sema"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
	"github.com/starthalf51/diegesis-lang/rules"
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
		t.Fatal("SL_PG_DSN is required")
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
	f, diags := syntax.ParseFile("t.dieg", []byte(strings.ReplaceAll(fixture, "WK", id)))
	w, ds := sema.Build([]*syntax.File{f})
	for _, d := range append(diags, ds...) {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	return w
}

// Loading the same work under a different ID gives the same fingerprint, and injecting a defect makes it differ.
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

// "Is it used later" is decided by PG precedes: if there is only use earlier than the receipt, not applicable.
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

// Task 11 (Odyssey): a belief the hearer withdrew by doubt before using it causes no defect when removed (n/a).
// It is used as heard again after the doubt, so the original hearing act is not a source at the time of use (same as the withdrawal in lint_known).
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
	f, _ := syntax.ParseFile("t.dieg", []byte(src))
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

// Review Focus 5: a new diagnostic counts as detection only if it is at the mutation site (Site: that act/scene, acts that use the mutated proposition, edges through the mutated point).
// An info-flow in an unrelated scene is not detection even if it is an expected rule.
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

// Task 2: Bible 5:27 Methuselah's lifespan (969 years) +50 years → query methuselah-dies-am-1656 changes → detection (R1: query detection is independent of site).
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
	var log strings.Builder
	// A query that uses the work ID as a constant (like the graph name) gives a different result on the copy
	qs := append(slices.Clone(e.Queries), Query{Name: "work-constant", SQL: "SELECT $1 = '" + work + "'"})
	h, err := newHarness(ctx, pool, w, ds, qs, &log)
	if err != nil {
		t.Fatal(err)
	}
	// Review fix 1: a query that reads the diagnostic table is lint detection that bypasses the site restriction, so it is excluded from query detection.
	// A query whose graph name is a work constant gives a different result on the renamed-only copy, so it is excluded. One line of reason per excluded query.
	var kept []string
	for _, q := range h.qs {
		kept = append(kept, q.Name)
	}
	for _, x := range []struct{ name, why string }{
		{"prediction-paid-off-in-every-account", "diagnostic"},
		{"work-constant", "copy"},
	} {
		if slices.Contains(kept, x.name) {
			t.Errorf("%s kept for query detection, want excluded", x.name)
		}
		if !strings.Contains(log.String(), x.name) || !strings.Contains(log.String(), x.why) {
			t.Errorf("log does not name %s with reason %q:\n%s", x.name, x.why, log.String())
		}
	}
	if !slices.Contains(kept, "methuselah-dies-am-1656") {
		t.Errorf("kept queries %v, want methuselah-dies-am-1656", kept)
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

// A mutant that only changes order (scene 2 before scene 1) can leave the believes·uses rows and point bounds unchanged. The fingerprint also holds each edge's
// two-endpoint precedes, so such a mutant is not counted as equivalent (equivalent = same derived result — order is a derived result too).
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
