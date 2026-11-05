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

	"story-lang/internal/ir"
	"story-lang/internal/mutate"
	"story-lang/internal/pg"
	"story-lang/internal/sema"
	"story-lang/internal/syntax"
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
	ms := mutate.Generate(build(t, map[string]string{"t.sl": fixture}), 1, 0)
	for op, want := range map[string][]string{
		// a tells b: p → b 가 2 에서 p 를 쓴다. b tells a: r → a 는 r 을 안 쓴다. c(oracle 그룹) tells → a 가 q 를 안 쓴다
		"drop-tells": {"1#a2"},
		// b 는 1 에서 p 를 듣고 2 에서 쓴다. a·c 는 들은 것을 다른 데서 쓰지 않는다
		"drop-enter": {"b@1"},
		// 쓰는 행위: a tells p, b tells r(배경 지식이라 제외), b tells d p. c(면제)는 제외
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
	}
	if n := len(mutate.Generate(build(t, map[string]string{"t.sl": fixture}), 7, 3)); n != 3 {
		t.Errorf("Generate(n=3) = %d mutations", n)
	}
}

func TestApplyCopies(t *testing.T) {
	w := build(t, map[string]string{"t.sl": fixture})
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
	if r := mutate.Rename(w, "t~m1"); r.Works[0].ID != "t~m1" || !strings.HasPrefix(r.Acts[0].ID, "t~m1/") {
		t.Errorf("Rename: %s %s", r.Works[0].ID, r.Acts[0].ID)
	}
}

// 맥베스에서 drop-tells 한 건(1.2 로스가 덩컨에게 코더의 반역을 알림) → 그 자리에 있던 앵거스가
// 1.3 에서 같은 말을 할 때 출처가 없다 → info-flow.
func TestMacbethDropTellsDetected(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
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
	t.Cleanup(pool.Close) // 아래 정리보다 나중에 돈다(LIFO)
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
	if !slices.Contains(got, "macbeth.act1/1.3#a13") {
		t.Errorf("info-flow warns = %v, want angus's 1.3#a13", got)
	}
}

func corpus(t *testing.T, dir string) *ir.Work {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".sl.md") {
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
