package pg_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/pg"
)

var pgxReadOnly = pgx.TxOptions{AccessMode: pgx.ReadOnly}

// Review round 1: a suppression does not cover a real cycle in a shared SCC.
func TestTimeCycleWaiverSharedSCC(t *testing.T) {
	pool := open(t)
	f := newFx()
	a, b, c := f.pt("A"), f.pt("B"), f.pt("C")
	ab := f.edge(a, b, i64(0), nil)
	ba := f.edge(b, a, i64(0), nil)
	f.TimeEdges[len(f.TimeEdges)-1].Waived = true
	f.TimeEdges[len(f.TimeEdges)-1].WaiverRule = "time-cycle"
	bc := f.edge(b, c, i64(0), nil)
	cb := f.edge(c, b, i64(0), nil)
	f.load(t, pool)
	got := cycles(t, pool, f.id)
	var open_, waived []diag
	for _, d := range got {
		if d.Waived {
			waived = append(waived, d)
		} else {
			open_ = append(open_, d)
		}
	}
	if len(open_) != 1 || !slices.Equal(open_[0].Src, []int{bc, cb}) {
		t.Errorf("unwaived = %+v, want one with src [%d %d]", open_, bc, cb)
	}
	if len(waived) != 1 || !slices.Equal(waived[0].Src, []int{ab, ba}) {
		t.Errorf("waived = %+v, want one with src [%d %d]", waived, ab, ba)
	}
}

// Review round 1: a contradiction elsewhere does not erase the bounds of unrelated points.
func TestTimeCycleKeepsUnrelatedBounds(t *testing.T) {
	pool := open(t)
	f := newFx()
	a, b := f.pt("A"), f.pt("B")
	f.edge(a, b, i64(2*day), nil)
	f.edge(a, b, nil, i64(day))
	d := f.pt("D")
	f.edge(f.ref("start"), d, i64(day), i64(2*day))
	e := f.pt("E")
	f.edge(d, e, i64(0), nil)
	f.load(t, pool)
	if got := cycles(t, pool, f.id); len(got) != 1 {
		t.Fatalf("diags = %+v", got)
	}
	if lo, hi := bounds(t, pool, d); show(lo) != "86400" || show(hi) != "172800" {
		t.Errorf("D = [%s,%s], want [86400,172800]", show(lo), show(hi))
	}
	if got := precedes(t, pool, f.id, d, e); got != "yes" {
		t.Errorf("precedes(D, E) = %s, want yes", got)
	}
}

// Review round 1: precedes only reads precomputed distances. 2000 calls on a 500-point chain < 2s.
func TestPrecedesSpeed(t *testing.T) {
	pool := open(t)
	ctx := context.Background()
	f := newFx()
	pts := []string{f.pt("p0")}
	for i := 1; i < 500; i++ {
		pts = append(pts, f.pt(fmt.Sprintf("p%d", i)))
		f.edge(pts[i-1], pts[i], i64(0), nil)
	}
	f.load(t, pool)
	t0 := time.Now()
	if _, err := pool.Exec(ctx, `SELECT derive($1)`, f.id); err != nil {
		t.Fatal(err)
	}
	t.Logf("derive(500-point chain) = %v", time.Since(t0))

	tx, err := pool.BeginTx(ctx, pgxReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	t0 = time.Now()
	yes := 0
	for i := 0; i < 2000; i++ {
		a, b := pts[(i*7)%500], pts[(i*13)%500]
		var s string
		if err := tx.QueryRow(ctx, `SELECT precedes($1,$2,$3)`, f.id, a, b).Scan(&s); err != nil {
			t.Fatal(err)
		}
		if s == "yes" {
			yes++
		}
	}
	el := time.Since(t0)
	t.Logf("2000 precedes = %v (%d yes)", el, yes)
	if el > 2*time.Second {
		t.Errorf("2000 precedes took %v, want < 2s", el)
	}
	if s := precedes(t, pool, f.id, pts[3], pts[400]); s != "yes" {
		t.Errorf("chain p3 → p400 = %s", s)
	}
	if s := precedes(t, pool, f.id, pts[400], pts[3]); s != "no" {
		t.Errorf("chain p400 → p3 = %s", s)
	}

	w, id := golden(t)
	cleanup(t, pool, id)
	if err := pg.Load(ctx, pool, w); err != nil {
		t.Fatal(err)
	}
	t0 = time.Now()
	if _, err := pool.Exec(ctx, `SELECT derive($1)`, id); err != nil {
		t.Fatal(err)
	}
	t.Logf("derive(macbeth) = %v", time.Since(t0))
}

// Controller verdict: explicit believes statements merge into derived believes. doubts makes no row.
func TestDeclaredBeliefs(t *testing.T) {
	pool := open(t)
	f := newFx()
	a := f.ent("a")
	p, q := f.fact("p"), f.fact("q")
	x := f.pt("x")
	s := f.src()
	f.Beliefs = []ir.Belief{
		{EntityID: a, Verb: "believes", PropID: p, AtPt: x, SrcID: s},
		{EntityID: a, Verb: "doubts", PropID: q, AtPt: x, SrcID: f.src()},
	}
	f.load(t, pool)
	got := strs(t, pool, `SELECT entity_id||' '||prop_id||' '||value::text||' '||from_pt||' '||declared::text||' '||src_id::text
		FROM believes WHERE work_id=$1`, f.id)
	want := []string{fmt.Sprintf("%s %s true %s true %d", a, p, x, s)}
	if !slices.Equal(got, want) {
		t.Errorf("believes = %v, want %v", got, want)
	}
}

// Controller verdict: a holding bound with maybe order does not break the load; it gives a role-overlap diagnostic. An act at the boundary point goes to the new holder.
func TestRoleOverlapAndBoundary(t *testing.T) {
	pool := open(t)
	t.Run("maybe-ordered overlap warns", func(t *testing.T) {
		f := newFx()
		x, y := f.ent("x"), f.ent("y")
		r := f.ref("king")
		f.Roles = []ir.Role{{ID: r, WorkID: f.id, SrcID: f.src()}}
		p := f.pt("P")
		f.edge(f.ref("start"), p, i64(5*day), i64(6*day))
		q := f.pt("Q") // order relative to P is unknown
		xs, hs := f.src(), f.src()
		f.RoleHolders = []ir.RoleHolder{
			{RoleID: r, EntityID: x, UntilPt: p, SrcID: xs},
			{RoleID: r, EntityID: y, FromPt: q, SrcID: hs},
		}
		f.load(t, pool)
		got := strs(t, pool, `SELECT severity||' '||src_id::text FROM diagnostic WHERE work_id=$1 AND rule_id='role-overlap'`, f.id)
		if !slices.Equal(got, []string{fmt.Sprintf("warn %d", hs)}) {
			t.Errorf("role-overlap = %v, want [warn %d]", got, hs)
		}
		if got := strs(t, pool, `SELECT entity_id FROM role_span WHERE work_id=$1`, f.id); !slices.Equal(got, []string{x}) {
			t.Errorf("role_span = %v, want only %s", got, x)
		}
	})
	t.Run("act at boundary resolves to new holder", func(t *testing.T) {
		f := newFx()
		x, y, z := f.ent("x"), f.ent("y"), f.ent("z")
		pr := f.fact("p")
		r := f.ref("king")
		f.Roles = []ir.Role{{ID: r, WorkID: f.id, SrcID: f.src()}}
		at := f.act("crown", r, "tells", pr, z)
		at.ActorIsRole = true
		before := f.act("before", r, "tells", pr, z)
		before.ActorIsRole = true
		f.edge(before.Pt, at.Pt, i64(0), nil)
		f.RoleHolders = []ir.RoleHolder{
			{RoleID: r, EntityID: x, UntilPt: at.Pt, SrcID: f.src()},
			{RoleID: r, EntityID: y, FromPt: at.Pt, SrcID: f.src()},
		}
		f.load(t, pool)
		for act, want := range map[string]string{at.ID: y, before.ID: x} {
			if got := strs(t, pool, `SELECT coalesce(resolved_actor_id,'?') FROM act WHERE id=$1`, act); !slices.Equal(got, []string{want}) {
				t.Errorf("%s resolves to %v, want %s", act, got, want)
			}
		}
		if got := strs(t, pool, `SELECT target_id FROM diagnostic WHERE work_id=$1 AND rule_id='role-ambiguous'`, f.id); len(got) != 0 {
			t.Errorf("role-ambiguous = %v, want none", got)
		}
	})
}
