// Package sema resolves names, checks verb argument kinds, runs the
// per-scene presence state machine (audience per act), turns `when` into
// time edges and attaches `#!` waivers. It emits IR rows only: time solving,
// role spans, beliefs and lint verdicts happen in PG (ADR-010).
package sema

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

type decl struct{ kind, id string }

// cont is a scene, event or account: anything with start/end points.
type cont struct {
	id, kind   string
	start, end string
	beats      map[string]string
}

type module struct {
	name, id  string
	files     []*syntax.File
	imports   []*module
	importPos map[*module]syntax.Pos
	importF   map[*module]string
	decls     map[string]decl
	conts     map[string]*cont
	setups    map[string]string
	out       ir.Work
	failed    bool
}

type srcKey struct {
	file      string
	line, col int
}

type builder struct {
	work, origin string
	epoch        int64 // origin day on the no-leap calendar
	hasEpoch     bool
	mods         map[string]*module
	order        []*module
	kinds        map[string]string   // id → decl kind
	members      map[string][]string // group → members
	lives        map[string]bool     // <char>.born / .died points
	dord         map[*syntax.Scene]int
	g            ir.Work // work-wide rows, never dropped
	facts        map[string]bool
	src          map[srcKey]int
	diags        []syntax.Diag
	cur          *module
	file         string
	pend         []*syntax.Pragma // waivers of the statement being handled
	whenEdges    []int            // edges made by the current statement's own `when`
	nattr        map[string]int   // attr count per container/module (attribute ids)
}

// Build turns parsed files (one work) into IR. Modules with errors emit no
// rows; diagnostics are still collected for every module.
func Build(files []*syntax.File) (*ir.Work, []syntax.Diag) {
	files = slices.Clone(files)
	sort.SliceStable(files, func(i, j int) bool { return naturalLess(files[i].Path, files[j].Path) })
	b := &builder{mods: map[string]*module{}, kinds: map[string]string{}, members: map[string][]string{},
		lives: map[string]bool{}, dord: map[*syntax.Scene]int{}, facts: map[string]bool{}, src: map[srcKey]int{}, nattr: map[string]int{}}
	for _, f := range files {
		if f.Module == nil {
			continue
		}
		m := b.mods[f.Module.Name]
		if m == nil {
			m = &module{name: f.Module.Name, importPos: map[*module]syntax.Pos{}, importF: map[*module]string{},
				decls: map[string]decl{}, conts: map[string]*cont{}, setups: map[string]string{}}
			b.mods[m.name] = m
			b.order = append(b.order, m)
		}
		m.files = append(m.files, f)
	}
	sort.Slice(b.order, func(i, j int) bool { return naturalLess(b.order[i].name, b.order[j].name) })
	b.findWork(files)
	for _, m := range b.order {
		m.id = b.work + "/" + m.name
	}
	b.collect(files)
	b.cycles()
	b.second()
	for _, m := range b.order {
		b.cur = m
		b.emitModule(m)
	}
	w := &ir.Work{Schema: ir.Schema}
	merge(w, &b.g)
	for _, m := range b.order {
		if !m.failed {
			merge(w, &m.out)
		}
	}
	lifeOrder(w)
	merge(w, &ir.Work{}) // nil tables → []
	sort.SliceStable(b.diags, func(i, j int) bool {
		a, c := b.diags[i], b.diags[j]
		if a.File != c.File {
			return a.File < c.File
		}
		return a.Line < c.Line || a.Line == c.Line && a.Col < c.Col
	})
	return w, b.diags
}

// lifeOrder: 태어나기 전에 죽을 수 없다 — born 과 dies 가 둘 다 있는 사람마다 @x.born → @x.died (min 0) 순서 간선.
// 시간 풀이(PG)가 죽음이 출생보다 앞서는 제약을 time-cycle 로 잡게 한다. 위치는 dies 문장이다.
func lifeOrder(w *ir.Work) {
	src := map[string]int{}
	for _, p := range w.Points {
		if p.Kind == "died" {
			src[strings.TrimSuffix(p.ID, ".died")] = p.SrcID
		}
	}
	for _, p := range w.Points {
		if who, ok := strings.CutSuffix(p.ID, ".born"); ok && p.Kind == "born" {
			if s, ok := src[who]; ok {
				zero := int64(0)
				w.TimeEdges = append(w.TimeEdges, ir.TimeEdge{From: p.ID, To: who + ".died", Min: &zero, SrcID: s})
			}
		}
	}
}

// merge appends every table of src to dst and turns nil tables into [].
func merge(dst, src *ir.Work) {
	dv, sv := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	for i := 0; i < dv.NumField(); i++ {
		f := dv.Field(i)
		if f.Kind() != reflect.Slice {
			continue
		}
		f.Set(reflect.AppendSlice(f, sv.Field(i)))
		if f.IsNil() {
			f.Set(reflect.MakeSlice(f.Type(), 0, 0))
		}
	}
}

// findWork picks the work id from the first `work` declaration. Without
// one there is no origin point: error no-work at the first file (the first
// module name segment stands in so diagnostics still carry ids).
func (b *builder) findWork(files []*syntax.File) {
	for _, m := range b.order {
		for _, f := range m.files {
			for _, it := range f.Items {
				if w, ok := it.(*syntax.Work); ok && b.work == "" {
					b.work = w.ID.Name
				}
			}
		}
	}
	if b.work == "" && len(b.order) > 0 {
		b.work, _, _ = strings.Cut(b.order[0].name, ".")
		for _, f := range files {
			if f.Module != nil {
				b.cur, b.file = b.mods[f.Module.Name], f.Path
				b.errf(syntax.Pos{Line: 1, Col: 1}, "no-work", "no `work` declaration in this build (it defines the time origin)")
				break
			}
		}
	}
	b.origin = b.work + "/start"
}

func (b *builder) diag(pos syntax.Pos, sev, rule, format string, args ...any) *syntax.Diag {
	b.diags = append(b.diags, syntax.Diag{File: b.file, Line: pos.Line, Col: pos.Col, Severity: sev,
		Rule: rule, Msg: fmt.Sprintf(format, args...)})
	return &b.diags[len(b.diags)-1]
}

func (b *builder) errf(pos syntax.Pos, rule, format string, args ...any) {
	if b.cur != nil {
		b.cur.failed = true
	}
	b.diag(pos, "error", rule, format, args...)
}

// warn reports a waivable warning; a matching `#!` on the current statement
// keeps it but marks it waived.
func (b *builder) warn(pos syntax.Pos, rule, format string, args ...any) {
	d := b.diag(pos, "warn", rule, format, args...)
	for _, p := range b.pend {
		if p.Rule == rule && strings.TrimSpace(p.Reason) != "" {
			d.Waived = true
		}
	}
}

func (b *builder) srcID(pos syntax.Pos) int {
	k := srcKey{b.file, pos.Line, pos.Col}
	if id, ok := b.src[k]; ok {
		return id
	}
	id := len(b.g.Src) + 1
	mod := ""
	if b.cur != nil {
		mod = b.cur.id
	}
	b.g.Src = append(b.g.Src, ir.Src{ID: id, Module: mod, File: b.file, Line: pos.Line, Col: pos.Col})
	b.src[k] = id
	return id
}

func (b *builder) emitModule(m *module) {
	row := ir.Module{ID: m.id, WorkID: b.work, Name: m.name, Paths: []string{}, Imports: []string{}}
	for _, f := range m.files {
		row.Paths = append(row.Paths, f.Path)
	}
	for _, im := range m.imports {
		row.Imports = append(row.Imports, im.id)
	}
	m.out.Modules = append(m.out.Modules, row)
	for _, f := range m.files {
		b.file = f.Path
		b.stmts(m, f.Items, func(n syntax.Node) (string, string) { return b.top(m, n) })
	}
}

// stmts walks a statement list, attaching each `#!` run to the next
// statement. handle returns the waiver target (kind, id).
func (b *builder) stmts(m *module, body []syntax.Node, handle func(syntax.Node) (string, string)) {
	var pend []*syntax.Pragma
	for _, n := range body {
		if p, ok := n.(*syntax.Pragma); ok {
			pend = append(pend, p)
			continue
		}
		cur := pend
		pend = nil
		b.pend = cur
		_, isWhen := n.(*syntax.When)
		_, isLife := n.(*syntax.Life)
		b.whenEdges = nil
		kind, id := handle(n)
		own := b.whenEdges
		b.pend, b.whenEdges = nil, nil
		for _, i := range own {
			m.out.TimeEdges[i].TargetKind, m.out.TimeEdges[i].TargetID = kind, id
		}
		for _, p := range cur {
			// only time-cycle waives edges, and only the statement's own
			// `when` edges: origin and statement-order edges stay in the solve
			if !b.waiver(m, p, kind, id) || p.Rule != "time-cycle" || !(isWhen || isLife) {
				continue
			}
			for _, i := range own {
				e := &m.out.TimeEdges[i]
				e.Waived, e.WaiverRule, e.WaiverReason = true, p.Rule, p.Reason
			}
		}
	}
	for _, p := range pend {
		b.errf(p.Pos, "waiver-target", "#! allow %s is not followed by a statement", p.Rule)
	}
}

// waiver records `#! allow` against a target; an empty reason is an error.
func (b *builder) waiver(m *module, p *syntax.Pragma, kind, id string) bool {
	if strings.TrimSpace(p.Reason) == "" {
		b.errf(p.Pos, "waiver-reason", "#! allow %s needs a reason", p.Rule)
		return false
	}
	if id == "" {
		return false
	}
	m.out.Waivers = append(m.out.Waivers, ir.Waiver{Rule: p.Rule, TargetKind: kind, TargetID: id,
		Reason: p.Reason, SrcID: b.srcID(p.Pos)})
	return true
}

// naturalLess orders strings with digit runs compared as numbers, so
// book-2 < book-10 and 1.2 < 1.10 (file order is discourse order).
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}
