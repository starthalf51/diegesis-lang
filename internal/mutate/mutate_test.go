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

	"silmari/internal/ir"
	"silmari/internal/mutate"
	"silmari/internal/pg"
	"silmari/internal/sema"
	"silmari/internal/syntax"
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
	ms := mutate.Generate(build(t, map[string]string{"t.sil": fixture}), 1, 0)
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
		// "뒤에 쓰는가" 는 하네스가 PG 로 판정할 쌍으로 넘긴다
		if (m.Op == "drop-tells" || m.Op == "drop-enter" || m.Op == "flip-perceived") && len(m.Pairs) == 0 {
			t.Errorf("%s %s: no (heard, used) pairs", m.Op, m.Target)
		}
		if m.Op == "move-before" && len(m.Uses) == 0 {
			t.Errorf("%s %s: no used props", m.Op, m.Target)
		}
	}
	if n := len(mutate.Generate(build(t, map[string]string{"t.sil": fixture}), 7, 3)); n != 3 {
		t.Errorf("Generate(n=3) = %d mutations", n)
	}
}

func TestApplyCopies(t *testing.T) {
	w := build(t, map[string]string{"t.sil": fixture})
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
	// move-before: 출처(Before) 1초 앞으로. 원래 장면 사슬의 간선은 떼고 원점 간선만 남긴다.
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

// Task 11: 페넬로페의 포옹(23.19, because beggar_is_odysseus)을 19권(19.34 앞)으로 옮기면 그녀는 아직 모른다 —
// 19.30 아테나가 눈길을 돌렸고 그녀가 믿게 되는 것은 23.18 침대의 비밀이다. because-unbelieved 로 잡혀야 한다.
func TestOdysseyEmbraceInto19Detected(t *testing.T) {
	if os.Getenv("SL_PG_DSN") == "" {
		t.Fatal("SL_PG_DSN 이 필요하다")
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

// Task 2: 시간 연산자. shift-duration = `by a..b` 구간 하나를 +50년, swap-order = 같은 사람의 born→died 간선이나
// `before` 간선 하나를 뒤집는다(before → after). 대상 = "from to"(간선의 두 끝점).
func TestGenerateTimeOps(t *testing.T) {
	w := build(t, map[string]string{"t.sil": timeFixture})
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
			// 지점: 두 끝점과 그 사람(a)의 행위
			if !slices.Contains(m.Site, "t/t.s/a.died") || !slices.Contains(m.Site, "t/t.s/1#a1") {
				t.Errorf("shift-duration site = %v, want a.died and a's act 1#a1", m.Site)
			}
		case m.Op == "swap-order" && m.Target == "t/t.s/e.end t/t.s/1.start":
			// `e before 1` → `e after 1`: e.end→1.start 를 1.end→e.start 로(장면 경계는 반대쪽 끝으로 바꾼다)
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

// 지점: drop-tells 1#a2(a tells b: p) = 그 행위, 그 장면, p 를 쓰는 행위(2#a1 b tells d: p, 1#a2 자신).
func TestGenerateSites(t *testing.T) {
	for _, m := range mutate.Generate(build(t, map[string]string{"t.sil": fixture}), 1, 0) {
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
