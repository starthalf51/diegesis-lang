package pg_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

const day = int64(86400)

func i64(n int64) *int64 { return &n }

// fx is a small hand-written IR. Every ID starts with a unique work ID.
type fx struct {
	*ir.Work
	id string
}

func newFx() *fx {
	id := fmt.Sprintf("t_%d", rand.Int63())
	f := &fx{Work: &ir.Work{Schema: ir.Schema}, id: id}
	f.Works = []ir.WorkRow{{ID: id, Title: "t", Unit: "day", SrcID: f.src()}}
	f.Points = []ir.Point{{ID: id + "/start", WorkID: id, Kind: "origin"}}
	return f
}

func (f *fx) ref(s string) string { return f.id + "/" + s }

// src makes a new source position and returns the ID (line = ID).
func (f *fx) src() int {
	n := len(f.Src) + 1
	f.Src = append(f.Src, ir.Src{ID: n, File: "t.dieg", Line: n, Col: 1})
	return n
}

// pt makes a point after the origin (origin→p [0,∞), same as the compiler).
func (f *fx) pt(name string) string {
	id := f.ref(name)
	f.Points = append(f.Points, ir.Point{ID: id, WorkID: f.id, Kind: "beat"})
	f.edge(f.ref("start"), id, i64(0), nil)
	return id
}

func (f *fx) edge(a, b string, min, max *int64) int {
	s := f.src()
	f.TimeEdges = append(f.TimeEdges, ir.TimeEdge{From: a, To: b, Min: min, Max: max, SrcID: s})
	return s
}

func (f *fx) ent(name string) string {
	id := f.ref(name)
	f.Entities = append(f.Entities, ir.Entity{ID: id, WorkID: f.id, Kind: "character", Name: name, SrcID: f.src()})
	return id
}

func (f *fx) fact(name string) string {
	id := f.ref(name)
	f.Facts = append(f.Facts, ir.Fact{ID: id, WorkID: f.id, Text: name, Truth: "true", SrcID: f.src()})
	return id
}

// act makes a new point and puts an act on it.
func (f *fx) act(name, actor, verb, prop string, audience ...string) *ir.Act {
	id := f.pt(name)
	f.Acts = append(f.Acts, ir.Act{ID: id, Pt: id, ActorID: actor, Verb: verb, PropID: prop, SrcID: f.src()})
	for _, e := range audience {
		f.Audience = append(f.Audience, ir.Audience{ActID: id, EntityID: e})
	}
	return &f.Acts[len(f.Acts)-1]
}

func (f *fx) load(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	cleanup(t, pool, f.id)
	if err := pg.Load(context.Background(), pool, f.Work); err != nil {
		t.Fatal(err)
	}
}

func strs(t *testing.T, pool *pgxpool.Pool, q string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func bounds(t *testing.T, pool *pgxpool.Pool, p string) (e, l *int64) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), `SELECT earliest, latest FROM point WHERE id=$1`, p).Scan(&e, &l); err != nil {
		t.Fatal(err)
	}
	return
}

func show(p *int64) string {
	if p == nil {
		return "NULL"
	}
	return fmt.Sprint(*p)
}

func TestSolveTimeBounds(t *testing.T) {
	pool := open(t)
	f := newFx()
	a, b := f.pt("A"), f.pt("B")
	f.edge(a, b, i64(day), i64(2*day)) // B after A by 1d..2d
	c := f.pt("C")
	f.edge(f.ref("start"), c, i64(3*day), i64(4*day)) // C at [3d,4d]
	f.edge(c, b, i64(0), nil)                         // C before B
	f.load(t, pool)
	for _, w := range []struct {
		p    string
		e, l string
	}{{a, "86400", "NULL"}, {b, "259200", "NULL"}, {c, "259200", "345600"}} {
		e, l := bounds(t, pool, w.p)
		if show(e) != w.e || show(l) != w.l {
			t.Errorf("%s = [%s,%s], want [%s,%s]", w.p, show(e), show(l), w.e, w.l)
		}
	}
	// Alone: B after A by 1d..2d → B.earliest = 86400
	g := newFx()
	a, b = g.pt("A"), g.pt("B")
	g.edge(a, b, i64(day), i64(2*day))
	g.load(t, pool)
	if e, _ := bounds(t, pool, b); show(e) != "86400" {
		t.Errorf("B.earliest = %s, want 86400", show(e))
	}
}

type diag struct {
	Src    []int `json:"src_ids"`
	Waived bool
}

func cycles(t *testing.T, pool *pgxpool.Pool, work string) []diag {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT data, waived FROM diagnostic WHERE work_id=$1 AND rule_id='time-cycle' ORDER BY src_id`, work)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []diag
	for rows.Next() {
		var raw []byte
		var d diag
		if err := rows.Scan(&raw, &d.Waived); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func TestTimeCycle(t *testing.T) {
	pool := open(t)
	t.Run("before-loop", func(t *testing.T) {
		f := newFx()
		a, b := f.pt("A"), f.pt("B")
		s1 := f.edge(a, b, i64(0), nil) // A before B
		s2 := f.edge(b, a, i64(0), nil) // B before A
		f.load(t, pool)
		got := cycles(t, pool, f.id)
		if len(got) != 1 || !slices.Equal(got[0].Src, []int{s1, s2}) || got[0].Waived {
			t.Fatalf("time-cycle diags = %+v, want one unwaived with src [%d %d]", got, s1, s2)
		}
		var sev string
		pool.QueryRow(context.Background(), `SELECT severity FROM diagnostic WHERE work_id=$1`, f.id).Scan(&sev)
		if sev != "warn" {
			t.Errorf("severity = %q", sev)
		}
	})
	t.Run("negative-quantitative", func(t *testing.T) {
		f := newFx()
		a, b := f.pt("A"), f.pt("B")
		s1 := f.edge(a, b, i64(2*day), nil) // B ≥ A+2d
		s2 := f.edge(a, b, nil, i64(day))   // B ≤ A+1d
		f.load(t, pool)
		got := cycles(t, pool, f.id)
		if len(got) != 1 || !slices.Equal(got[0].Src, []int{s1, s2}) {
			t.Fatalf("diags = %+v, want src [%d %d]", got, s1, s2)
		}
		// Cycle edges drop out of the solve (review round 1). Only "B is after the origin" remains.
		if e, l := bounds(t, pool, b); show(e) != "0" || l != nil {
			t.Errorf("B = [%s,%s], want [0,NULL] (cycle edges left out)", show(e), show(l))
		}
	})
	t.Run("waived", func(t *testing.T) {
		f := newFx()
		a, b := f.pt("A"), f.pt("B")
		f.edge(a, b, i64(day), nil)
		f.edge(b, a, i64(0), nil)
		f.TimeEdges[len(f.TimeEdges)-1].Waived = true
		f.TimeEdges[len(f.TimeEdges)-1].WaiverRule = "time-cycle"
		f.load(t, pool)
		got := cycles(t, pool, f.id)
		if len(got) != 1 || !got[0].Waived {
			t.Fatalf("diags = %+v, want one waived", got)
		}
		if e, _ := bounds(t, pool, b); show(e) != "86400" {
			t.Errorf("waived edge must be left out of solving: B.earliest = %s", show(e))
		}
	})
}

func precedes(t *testing.T, pool *pgxpool.Pool, w, a, b string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT precedes($1,$2,$3)`, w, a, b).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPrecedesMaybe(t *testing.T) {
	pool := open(t)
	f := newFx()
	a, b := f.pt("A"), f.pt("B") // siblings: only knows they are after the origin
	c, d := f.pt("C"), f.pt("D")
	f.edge(a, c, i64(0), nil) // A ≤ C
	f.edge(c, d, i64(day), nil)
	e := f.pt("E")
	f.edge(f.ref("start"), e, i64(10*day), i64(10*day)) // E = 10d
	g := f.pt("G")
	f.edge(f.ref("start"), g, i64(day), i64(2*day)) // G ∈ [1d,2d]
	f.load(t, pool)
	for _, w := range []struct{ a, b, want string }{
		{a, b, "maybe"}, {b, a, "maybe"},
		{a, d, "yes"}, {d, a, "no"}, {c, b, "maybe"},
		{g, e, "yes"}, {e, g, "no"}, // decided by the origin-relative interval alone
		{a, e, "maybe"},
	} {
		if got := precedes(t, pool, f.id, w.a, w.b); got != w.want {
			t.Errorf("precedes(%s, %s) = %s, want %s", w.a, w.b, got, w.want)
		}
	}
	var two string
	pool.QueryRow(context.Background(), `SELECT precedes($1,$2)`, a, d).Scan(&two)
	if two != "yes" {
		t.Errorf("precedes(a, d) = %q", two)
	}
}

func TestRoleExclusion(t *testing.T) {
	pool := open(t)
	t.Run("overlap fails load", func(t *testing.T) {
		f := newFx()
		x, y := f.ent("x"), f.ent("y")
		r := f.ref("king")
		f.Roles = []ir.Role{{ID: r, WorkID: f.id, SrcID: f.src()}}
		f.RoleHolders = []ir.RoleHolder{
			{RoleID: r, EntityID: x, FromPt: f.ref("start"), SrcID: f.src()},
			{RoleID: r, EntityID: y, FromPt: f.ref("start"), SrcID: f.src()},
		}
		cleanup(t, pool, f.id)
		err := pg.Load(context.Background(), pool, f.Work)
		if err == nil || !strings.Contains(err.Error(), "role_span") {
			t.Fatalf("overlapping holders must fail the load, err = %v", err)
		}
	})
	t.Run("succession and role speaker", func(t *testing.T) {
		f := newFx()
		x, y, z := f.ent("x"), f.ent("y"), f.ent("z")
		p := f.fact("p")
		r := f.ref("king")
		f.Roles = []ir.Role{{ID: r, WorkID: f.id, SrcID: f.src()}}
		dec := f.pt("decree")
		f.RoleHolders = []ir.RoleHolder{
			{RoleID: r, EntityID: x, UntilPt: dec, SrcID: f.src()},
			{RoleID: r, EntityID: y, FromPt: dec, SrcID: f.src()},
		}
		after := f.act("after", r, "tells", p, y, z) // speaker = y; even if y remains in the audience it must be dropped
		after.ActorIsRole = true
		f.edge(dec, after.Pt, i64(0), nil)
		vague := f.act("vague", r, "tells", p, z) // the order relative to the decree is unknown
		vague.ActorIsRole = true
		f.load(t, pool)
		if got := strs(t, pool, `SELECT coalesce(resolved_actor_id,'?') FROM act WHERE id=$1`, after.ID); !slices.Equal(got, []string{y}) {
			t.Errorf("after resolves to %v, want %s", got, y)
		}
		if got := strs(t, pool, `SELECT entity_id FROM believes WHERE cause_act=$1`, after.ID); !slices.Equal(got, []string{z}) {
			t.Errorf("believers of role speech = %v, want [%s] (holder excluded)", got, z)
		}
		if got := strs(t, pool, `SELECT target_id FROM diagnostic WHERE work_id=$1 AND rule_id='role-ambiguous'`, f.id); !slices.Equal(got, []string{vague.ID}) {
			t.Errorf("role-ambiguous = %v, want [%s]", got, vague.ID)
		}
		if got := strs(t, pool, `SELECT entity_id || ' ' || span::text FROM role_span WHERE work_id=$1`, f.id); len(got) != 2 {
			t.Errorf("role_span = %v", got)
		}
	})
}

func TestBeliefsFromAudience(t *testing.T) {
	pool := open(t)
	f := newFx()
	a, b, c := f.ent("a"), f.ent("b"), f.ent("c")
	p, q, l, s, u, v := f.fact("p"), f.fact("q"), f.fact("l"), f.fact("s"), f.fact("u"), f.fact("v")
	tell := f.act("tell", a, "tells", p, b, c)
	f.act("whisper", a, "whispers", q, b)
	f.act("aside", a, "aside", s)
	lie := f.act("lie", a, "lies", l, b, c)
	letter := f.ref("letter")
	f.Artifacts = []ir.Artifact{{ID: letter, WorkID: f.id, Name: "letter", Contains: []string{u, v}, SrcID: f.src()}}
	rd := f.act("read", c, "reads", "")
	rd.Objects = []string{letter}
	wr := f.act("write", b, "writes", "")
	wr.Objects = []string{letter}
	f.act("see", b, "sees", s)
	f.load(t, pool)

	got := strs(t, pool, `SELECT split_part(entity_id,'/',2)||' '||split_part(prop_id,'/',2)||' '||value::text||' '||split_part(cause_act,'/',2)
		FROM believes WHERE work_id=$1`, f.id)
	want := []string{
		"a l false lie", // the lying speaker believes ¬P
		"b l true lie", "b p true tell", "b q true whisper", "b s true see",
		"c l true lie", "c p true tell", "c u true read", "c v true read",
	}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("believes:\n got %v\nwant %v", got, want)
	}
	from := strs(t, pool, `SELECT DISTINCT from_pt FROM believes WHERE cause_act=$1`, tell.ID)
	if !slices.Equal(from, []string{tell.Pt}) {
		t.Errorf("from_pt = %v, want the act point", from)
	}
	uses := strs(t, pool, `SELECT split_part(entity_id,'/',2)||' '||split_part(prop_id,'/',2)||' '||split_part(act_id,'/',2) FROM uses WHERE work_id=$1`, f.id)
	if want := []string{"a p tell", "a q whisper", "b u write", "b v write"}; !slices.Equal(uses, want) {
		t.Errorf("uses = %v, want %v", uses, want)
	}
	dec := strs(t, pool, `SELECT act_id||' '||liar_id||' '||prop_id FROM deception WHERE work_id=$1`, f.id)
	if !slices.Equal(dec, []string{lie.ID + " " + a + " " + l}) {
		t.Errorf("deception = %v", dec)
	}
}

func TestGraphPath(t *testing.T) {
	pool := open(t)
	w, id := golden(t)
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	var n int
	q := fmt.Sprintf(`SELECT count(*) FROM cypher('g_%[1]s', $$
		MATCH p=(:Entity {id:'%[1]s/macbeth.cast/banquo'})-[:BELIEVES]->(:Fact {id:'%[1]s/macbeth.cast/king_hereafter'})
		RETURN p $$) AS (p agtype)`, id)
	if err := pool.QueryRow(context.Background(), q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("BELIEVES paths = %d, want 1", n)
	}
	for _, label := range []string{"Entity", "Point", "Fact", "Setup"} {
		q := fmt.Sprintf(`SELECT count(*) FROM cypher('g_%s', $$ MATCH (v:%s) RETURN v $$) AS (v agtype)`, id, label)
		if err := pool.QueryRow(context.Background(), q).Scan(&n); err != nil || n == 0 {
			t.Errorf("%s vertices = %d, err %v", label, n, err)
		}
	}
	for _, label := range []string{"BEFORE", "TOLD", "BELIEVES", "PRESENT_AT"} {
		q := fmt.Sprintf(`SELECT count(*) FROM cypher('g_%s', $$ MATCH ()-[e:%s]->() RETURN e $$) AS (e agtype)`, id, label)
		if err := pool.QueryRow(context.Background(), q).Scan(&n); err != nil || n == 0 {
			t.Errorf("%s edges = %d, err %v", label, n, err)
		}
	}
}

func TestLoadMacbethGolden(t *testing.T) {
	pool := open(t)
	w, id := golden(t)
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	got := strs(t, pool, `SELECT a.scene_id FROM believes b JOIN act a ON a.id = b.cause_act
		WHERE b.entity_id=$1 AND b.prop_id=$2 AND b.value`, id+"/macbeth.cast/banquo", id+"/macbeth.cast/king_hereafter")
	if len(got) == 0 || got[0] != id+"/macbeth.act1/1.3" {
		t.Errorf("banquo believes king_hereafter from %v, want an act in 1.3", got)
	}
	verbs := strs(t, pool, `SELECT DISTINCT a.verb FROM believes b JOIN act a ON a.id = b.cause_act WHERE b.entity_id=$1`,
		id+"/macbeth.cast/lady_macbeth")
	if !slices.Equal(verbs, []string{"reads"}) {
		t.Errorf("lady_macbeth belief causes = %v, want [reads]", verbs)
	}
	var diags int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM diagnostic WHERE work_id=$1`, id).Scan(&diags)
	if diags != 0 {
		t.Errorf("golden macbeth: %d derive diagnostics, want 0", diags)
	}
	var unsolved int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM point WHERE work_id=$1 AND earliest IS NULL`, id).Scan(&unsolved)
	if unsolved != 0 {
		t.Errorf("%d points without earliest", unsolved)
	}
}
