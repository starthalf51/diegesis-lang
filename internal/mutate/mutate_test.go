package mutate_test

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/mutate"
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/sema"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

const fixture = `module t.s
work t "T"
character a "A"
character b "B"
character c "C"
character d "D"
group o "O"
  members c
  knows oracle
fact p "P" true
fact q "Q" true
fact r "R" true
fact al = alive(a)
b believes r
scene 1
  enter a, b, c
  a sees p
  a tells b: p
  b tells a: r
  setup s promise: q
  c tells a: q
  exit all
event e
  when after 1
  dies a
scene 2
  when after e
  enter b, d
  b tells d: p
  b says: r
  payoff s
`

func build(t *testing.T, files map[string]string) *ir.Work {
	t.Helper()
	var fs []*syntax.File
	var diags []syntax.Diag
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		f, ds := syntax.ParseFile(n, []byte(files[n]))
		fs, diags = append(fs, f), append(diags, ds...)
	}
	w, ds := sema.Build(fs)
	for _, d := range append(diags, ds...) {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	return w
}

func targets(ms []mutate.Mutation, op string) []string {
	var out []string
	for _, m := range ms {
		if m.Op == op {
			out = append(out, strings.ReplaceAll(m.Target, "t/t.s/", ""))
		}
	}
	return out
}

func TestGenerateTargets(t *testing.T) {
	ms := mutate.Generate(build(t, map[string]string{"t.dieg": fixture}), 1, 0)
	for op, want := range map[string][]string{
		// a tells b: p → b uses p in 2. b tells a: r → a does not use r. c (oracle group) tells → a does not use q
		"drop-tells": {"1#a2"},
		// b hears p in 1 and uses it in 2. a and c do not use what they heard elsewhere
		"drop-enter": {"b@1"},
		// Using acts: a tells p, b tells r (excluded as background knowledge), b tells d p. c (exempt) is excluded
		"move-before":    {"1#a2", "2#a1"},
		"drop-setup":     {"s"},
		"drop-payoff":    {"s"},
		"kill-early":     {"a.died"},
		"flip-perceived": {"1#a1 p", "1#a2 p"},
	} {
		if got := targets(ms, op); !slices.Equal(got, want) {
			t.Errorf("%s targets = %v, want %v", op, got, want)
		}
	}
	for _, m := range ms {
		if len(m.Expect) == 0 {
			t.Errorf("%s %s: no expected rule", m.Op, m.Target)
		}
		// "Is it used later" is passed to the harness as pairs to judge with PG
		if (m.Op == "drop-tells" || m.Op == "drop-enter" || m.Op == "flip-perceived") && len(m.Pairs) == 0 {
			t.Errorf("%s %s: no (heard, used) pairs", m.Op, m.Target)
		}
		if m.Op == "move-before" && len(m.Uses) == 0 {
			t.Errorf("%s %s: no used props", m.Op, m.Target)
		}
	}
	if n := len(mutate.Generate(build(t, map[string]string{"t.dieg": fixture}), 7, 3)); n != 3 {
		t.Errorf("Generate(n=3) = %d mutations", n)
	}
}

func TestApplyCopies(t *testing.T) {
	w := build(t, map[string]string{"t.dieg": fixture})
	before := len(w.Acts)
	for _, m := range mutate.Generate(w, 1, 0) {
		got := mutate.Apply(w, m)
		if got == w {
			t.Fatalf("%s: Apply returned its input", m.Op)
		}
	}
	if len(w.Acts) != before {
		t.Error("Apply changed its input")
	}
	m := mutate.Mutation{Op: "drop-tells", Target: "t/t.s/1#a2"}
	got := mutate.Apply(w, m)
	if len(got.Acts) != before-1 || slices.ContainsFunc(got.Audience, func(a ir.Audience) bool { return a.ActID == m.Target }) {
		t.Errorf("drop-tells left the act or its audience")
	}
	flip := mutate.Apply(w, mutate.Mutation{Op: "flip-perceived", Target: "t/t.s/1#a1 t/t.s/p"})
	for _, a := range flip.Acts {
		if a.ID == "t/t.s/1#a1" && (len(a.With) != 1 || !strings.HasPrefix(a.With[0], "t/not ")) {
			t.Errorf("flip-perceived: with = %v, want the negation of p", a.With)
		}
	}
	r := mutate.Rename(w, "t~m1")
	if r.Works[0].ID != "t~m1" || !strings.HasPrefix(r.Acts[0].ID, "t~m1/") {
		t.Errorf("Rename: %s %s", r.Works[0].ID, r.Acts[0].ID)
	}
	for _, f := range r.Facts {
		if strings.Contains(f.ID, "t/") && !strings.Contains(f.ID, "t~m1/") || strings.Contains(strings.ReplaceAll(f.ID, "t~m1/", ""), "t/") {
			t.Errorf("Rename left the old work prefix inside %q", f.ID)
		}
	}
	// move-before: 1 second before the source (Before). Drop the edges of the original scene chain and keep only the origin edge.
	mv := mutate.Apply(w, mutate.Mutation{Op: "move-before", Target: "t/t.s/2#a1", Before: []string{"t/t.s/1#a2"}})
	var into, outof []string
	for _, e := range mv.TimeEdges {
		if e.To == "t/t.s/2#a1" {
			into = append(into, e.From)
		}
		if e.From == "t/t.s/2#a1" {
			outof = append(outof, fmt.Sprintf("%s>=%d", e.To, *e.Min))
		}
	}
	if !slices.Equal(into, []string{"t/start"}) || !slices.Equal(outof, []string{"t/t.s/1#a2>=1"}) {
		t.Errorf("move-before edges: into %v, out of %v", into, outof)
	}
}

// One drop-tells in Macbeth (1.2 Ross tells Duncan of Cawdor's treason) → Angus, who was there,
// has no source when he says the same thing in 1.3 → info-flow.
func TestMacbethDropTellsDetected(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN is required")
	}
	w := corpus(t, "../../corpus/macbeth")
	var m *mutate.Mutation
	for _, x := range mutate.Generate(w, 1, 0) {
		if x.Op == "drop-tells" && x.Target == "macbeth/macbeth.act1/1.2#a2" {
			m = &x
		}
	}
	if m == nil {
		t.Fatal("no drop-tells mutation for 1.2#a2 (ross tells duncan: cawdor_traitor)")
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // runs after the cleanup below (LIFO)
	id := fmt.Sprintf("macbeth~t%d", rand.Int63())
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM work WHERE id=$1`, id)
		pool.Exec(ctx, `SELECT drop_graph(name, true) FROM ag_graph WHERE name = graph_name($1)`, id)
	})
	if err := pg.Load(ctx, pool, mutate.Rename(mutate.Apply(w, *m), id)); err != nil {
		t.Fatal(err)
	}
	ds, err := pg.Lint(ctx, pool, id, os.DirFS("../../rules"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		if d.Rule == "info-flow" && d.Severity == "warn" && !d.Waived {
			got = append(got, strings.TrimPrefix(d.TargetID, id+"/"))
		}
	}
	if !slices.Contains(got, "macbeth.act1/1.3#a16") {
		t.Errorf("info-flow warns = %v, want angus's 1.3#a16", got)
	}
}

func corpus(t *testing.T, dir string) *ir.Work {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !syntax.IsSource(p) {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return build(t, files)
}

// Task 11: if Penelope's embrace (23.19, because beggar_is_odysseus) is moved to book 19 (before 19.34) she does not know yet —
// in 19.30 Athena turned the gaze away, and what she comes to believe is the secret of the bed at 23.18. It must be caught by because-unbelieved.
func TestOdysseyEmbraceInto19Detected(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN is required")
	}
	w := corpus(t, "../../corpus/odyssey")
	var embrace, in19 string
	for _, a := range w.Acts {
		switch {
		case a.SrcRef == "23.19" && a.Verb == "does":
			embrace = a.ID
		case a.SrcRef == "19.34":
			in19 = a.Pt
		}
	}
	if embrace == "" || in19 == "" {
		t.Fatalf("embrace %q, 19.34 %q not found", embrace, in19)
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	id := fmt.Sprintf("odyssey~t%d", rand.Int63())
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM work WHERE id=$1`, id)
		pool.Exec(ctx, `SELECT drop_graph(name, true) FROM ag_graph WHERE name = graph_name($1)`, id)
	})
	m := mutate.Mutation{Op: "move-before", Target: embrace, Before: []string{in19}}
	if err := pg.Load(ctx, pool, mutate.Rename(mutate.Apply(w, m), id)); err != nil {
		t.Fatal(err)
	}
	ds, err := pg.Lint(ctx, pool, id, os.DirFS("../../rules"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		if d.Severity == "warn" && !d.Waived {
			got = append(got, d.Rule+" "+strings.TrimPrefix(d.TargetID, id+"/"))
		}
	}
	if !slices.Contains(got, "because-unbelieved odyssey.b23/23.9#a8") {
		t.Errorf("warns = %v, want because-unbelieved on the embrace (23.9#a8)", got)
	}
}

const timeFixture = `module t.s
work t "T"
character a "A"
character b "B"
fact p "P" true
born a when after start by 0y..0y
born b when after @a.born by 20y..20y
dies a when after @a.born by 70y..70y
event e "E"
  when before 1
scene 1
  when after @b.born by 1y..1y
  enter a, b
  a tells b: p
`

// Task 2: time operators. shift-duration = +50 years on one `by a..b` interval, swap-order = flips the born→died edge of the same person or one
// `before` edge (before → after). Target = "from to" (the edge's two endpoints).
func TestGenerateTimeOps(t *testing.T) {
	w := build(t, map[string]string{"t.dieg": timeFixture})
	ms := mutate.Generate(w, 1, 0)
	for op, want := range map[string][]string{
		"shift-duration": {"t/start a.born", "a.born b.born", "a.born a.died", "b.born 1.start"},
		"swap-order":     {"a.born a.died", "e.end 1.start"},
	} {
		if got := targets(ms, op); !slices.Equal(got, want) {
			t.Errorf("%s targets = %v, want %v", op, got, want)
		}
	}
	const year = 365 * 24 * 3600
	edge := func(w *ir.Work, from, to string) *ir.TimeEdge {
		for i, e := range w.TimeEdges {
			if e.From == "t/t.s/"+from && e.To == "t/t.s/"+to && e.TargetID != "" {
				return &w.TimeEdges[i]
			}
		}
		return nil
	}
	for _, m := range ms {
		switch {
		case m.Op == "shift-duration" && m.Target == "t/t.s/a.born t/t.s/a.died":
			if e := edge(mutate.Apply(w, m), "a.born", "a.died"); e == nil || *e.Min != 120*year || *e.Max != 120*year {
				t.Errorf("shift-duration: edge %+v, want 70y + 50y", e)
			}
			// Site: the two endpoints and the acts of that person (a)
			if !slices.Contains(m.Site, "t/t.s/a.died") || !slices.Contains(m.Site, "t/t.s/1#a1") {
				t.Errorf("shift-duration site = %v, want a.died and a's act 1#a1", m.Site)
			}
		case m.Op == "swap-order" && m.Target == "t/t.s/e.end t/t.s/1.start":
			// `e before 1` → `e after 1`: e.end→1.start becomes 1.end→e.start (a scene boundary switches to the opposite end)
			if c := mutate.Apply(w, m); edge(c, "1.end", "e.start") == nil || edge(c, "e.end", "1.start") != nil {
				t.Error("swap-order: the before edge was not turned into after")
			}
			if !slices.Contains(m.Site, "t/t.s/1") || !slices.Contains(m.Site, "t/t.s/e") {
				t.Errorf("swap-order site = %v, want scene 1 and event e", m.Site)
			}
		case m.Op == "swap-order":
			if c := mutate.Apply(w, m); edge(c, "a.died", "a.born") == nil || edge(c, "a.born", "a.died") != nil {
				t.Error("swap-order: the lifespan edge was not reversed")
			}
		}
		if len(m.Expect) == 0 || len(m.Site) == 0 {
			t.Errorf("%s %s: expect %v site %v", m.Op, m.Target, m.Expect, m.Site)
		}
	}
}

// Site: drop-tells 1#a2 (a tells b: p) = that act, that scene, the acts that use p (2#a1 b tells d: p, and 1#a2 itself).
func TestGenerateSites(t *testing.T) {
	for _, m := range mutate.Generate(build(t, map[string]string{"t.dieg": fixture}), 1, 0) {
		if len(m.Site) == 0 {
			t.Errorf("%s %s: empty site", m.Op, m.Target)
		}
		if m.Op == "drop-tells" {
			got := strings.ReplaceAll(strings.Join(m.Site, " "), "t/t.s/", "")
			if got != "1#a2 1 2#a1" {
				t.Errorf("drop-tells site = %q, want %q", got, "1#a2 1 2#a1")
			}
		}
	}
}
