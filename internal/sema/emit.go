package sema

import (
	"silmari/internal/ir"
	"silmari/internal/syntax"
)

// top emits one top-level item and returns its waiver target.
func (b *builder) top(m *module, n syntax.Node) (string, string) {
	switch n := n.(type) {
	case *syntax.Work:
		return b.workItem(m, n)
	case *syntax.Decl:
		return b.declItem(m, n)
	case *syntax.Scene:
		var at, in string
		if n.At != nil {
			at = b.want(m, *n.At, "scene place", "place")
		}
		if n.In != nil {
			in = b.want(m, *n.In, "thread", "thread")
		}
		return "scene", b.container(m, n.ID, "scene", "", at, in, n.Body, n.Pos, b.dord[n])
	case *syntax.Event:
		return "event", b.container(m, n.ID, "event", n.Title, "", "", n.Body, n.Pos, 0)
	case *syntax.Account:
		ev := b.findCont(m, n.Of.Name)
		if ev == nil || ev.kind != "event" {
			b.errf(n.Of.Pos, "undefined-ref", "undefined event %q", n.Of.Name)
		}
		by := b.want(m, n.By, "account source")
		id := b.container(m, n.ID, "account", "", "", "", n.Body, n.Pos, 0)
		if ev != nil && id != "" {
			c := m.conts[n.ID.Name]
			b.edge(m, ev.start, c.start, i64(0), nil, n.Pos)
			b.edge(m, c.end, ev.end, i64(0), nil, n.Pos)
			m.out.Accounts = append(m.out.Accounts, ir.Account{ID: id, EventID: ev.id, SourceID: by, SrcID: b.srcID(n.Pos)})
		}
		return "account", id
	case *syntax.Setup:
		return "setup", b.setup(m, n, "", "")
	case *syntax.Distance:
		pa, pb := b.want(m, n.A, "place", "place"), b.want(m, n.B, "place", "place")
		m.out.Distances = append(m.out.Distances, ir.Distance{PlaceA: pa, PlaceB: pb, Min: n.Min.Sec, Max: n.Max.Sec,
			SrcID: b.srcID(n.Pos)})
		return "distance", pa
	case *syntax.Parent:
		return "parent", b.parent(m, n)
	case *syntax.Life:
		return "life", b.life(m, n, nil)
	case *syntax.Belief:
		who := b.want(m, n.Who, "believer", "character", "group")
		b.belief(m, n, who, "", "")
		return "belief", who
	}
	return "", ""
}

func (b *builder) workItem(m *module, w *syntax.Work) (string, string) {
	if w.ID.Name != b.work {
		b.errf(w.ID.Pos, "dup-decl", "second work %q (this build is work %q)", w.ID.Name, b.work)
		return "", ""
	}
	if len(b.g.Works) > 0 {
		b.errf(w.ID.Pos, "dup-decl", "work %q is declared twice", w.ID.Name)
		return "", ""
	}
	row := ir.WorkRow{ID: b.work, Title: w.Title, SrcID: b.srcID(w.Pos)}
	for _, a := range w.Attrs {
		switch a := a.(type) {
		case *syntax.Unit:
			row.Unit = a.Unit
		case *syntax.Start:
			row.Start = a.Date
			if _, _, ok := parseDate(a.Date); !ok {
				b.errf(a.Pos, "syntax", "bad date %q (want Y, Y-MM or Y-MM-DD)", a.Date)
			}
		case *syntax.Name:
			m.out.Names = append(m.out.Names, ir.Name{OwnerID: b.work, Lang: a.Lang, Text: a.Text, SrcID: b.srcID(a.Pos)})
		case *syntax.Pragma: // work-wide waiver
			b.waiver(m, a, "work", b.work)
		}
	}
	b.g.Works = append(b.g.Works, row)
	b.g.Points = append(b.g.Points, ir.Point{ID: b.origin, WorkID: b.work, Kind: "origin", SrcID: row.SrcID})
	return "work", b.work
}

func (b *builder) declItem(m *module, d *syntax.Decl) (string, string) {
	id := m.id + "/" + d.ID.Name
	src := b.srcID(d.Pos)
	for i, s := range d.Names {
		m.out.Names = append(m.out.Names, ir.Name{OwnerID: id, Text: s, Ord: i, SrcID: src})
	}
	name := ""
	if len(d.Names) > 0 {
		name = d.Names[0]
	}
	var role *ir.Role
	var art *ir.Artifact
	var ent *ir.Entity
	switch d.Kind {
	case "role":
		role = &ir.Role{ID: id, WorkID: b.work, Names: append([]string{}, d.Names...), SrcID: src}
	case "artifact":
		art = &ir.Artifact{ID: id, WorkID: b.work, ModuleID: m.id, Name: name, Contains: []string{}, Text: []string{}, SrcID: src}
	case "fact":
		f := ir.Fact{ID: id, WorkID: b.work, Text: name, Truth: "unknown", SrcID: src}
		if d.Value != "" {
			f.Truth = d.Value
		}
		if d.Def != nil {
			f.Truth = "derived"
			if did, _, ok := b.propDef(m, d.Def, false); ok {
				f.Def = &ir.PropDef{Alias: did}
			}
		}
		m.out.Facts = append(m.out.Facts, f)
	default:
		ent = &ir.Entity{ID: id, WorkID: b.work, ModuleID: m.id, Kind: d.Kind, Tier: d.Tier, Name: name, SrcID: src}
	}
	var holders []ir.RoleHolder
	misplaced := func(pos syntax.Pos, attr string) {
		b.errf(pos, "arg-kind", "%s does not apply to %s %q", attr, d.Kind, d.ID.Name)
	}
	for _, a := range d.Attrs {
		switch a := a.(type) {
		case *syntax.Name:
			m.out.Names = append(m.out.Names, ir.Name{OwnerID: id, Lang: a.Lang, Text: a.Text, SrcID: b.srcID(a.Pos)})
		case *syntax.Knows:
			if ent == nil {
				misplaced(a.Pos, "knows")
				continue
			}
			ent.Knows = a.Source
		case *syntax.Held:
			if role == nil {
				misplaced(a.Pos, "held")
				continue
			}
			holders = append(holders, b.held(m, id, a))
		case *syntax.Shared:
			if role == nil {
				misplaced(a.Pos, "shared")
				continue
			}
			role.Shared = true
		case *syntax.Contains:
			if art == nil {
				misplaced(a.Pos, "contains")
				continue
			}
			art.Contains = append(art.Contains, b.props(m, a.Props)...)
		case *syntax.Text:
			if art == nil {
				misplaced(a.Pos, "text")
				continue
			}
			art.Text = append(art.Text, a.Text)
		case *syntax.Belief:
			if ent == nil || (d.Kind != "character" && d.Kind != "group") {
				misplaced(a.Pos, a.Verb)
				continue
			}
			b.belief(m, a, id, "", "")
		case *syntax.Pragma:
			b.waiver(m, a, waiverKind(d.Kind), id)
		}
	}
	switch {
	case role != nil:
		m.out.Roles = append(m.out.Roles, *role)
		for _, h := range holders {
			h.Shared = role.Shared
			m.out.RoleHolders = append(m.out.RoleHolders, h)
		}
	case art != nil:
		m.out.Artifacts = append(m.out.Artifacts, *art)
	case ent != nil:
		m.out.Entities = append(m.out.Entities, *ent)
	}
	return waiverKind(d.Kind), id
}

func waiverKind(kind string) string {
	if kind == "role" || kind == "fact" || kind == "artifact" {
		return kind
	}
	return "entity"
}

func (b *builder) held(m *module, role string, h *syntax.Held) ir.RoleHolder {
	r := ir.RoleHolder{RoleID: role, HolderIsRole: h.Role, FromTref: trefText(h.From), UntilTref: trefText(h.Until),
		SrcID: b.srcID(h.Pos)}
	if h.Role {
		r.EntityID = b.wantRole(m, h.Holder)
	} else {
		r.EntityID = b.want(m, h.Holder, "role holder", "character", "group", "faction")
	}
	if h.From != nil {
		r.FromPt, _ = b.tref(m, h.From)
	}
	if h.Until != nil {
		r.UntilPt, _ = b.tref(m, h.Until)
	}
	return r
}

func (b *builder) setup(m *module, s *syntax.Setup, scene, at string) string {
	id := m.id + "/" + s.ID.Name
	m.out.Setups = append(m.out.Setups, ir.Setup{ID: id, Kind: s.Kind, PropID: b.prop(m, s.Prop), Text: s.Text,
		SceneID: scene, AtPt: at, SrcID: b.srcID(s.Pos)})
	return id
}

func (b *builder) parent(m *module, p *syntax.Parent) string {
	pid := b.want(m, p.Parent, "parent", "character")
	for _, c := range b.wantAll(m, p.Children, "child", "character") {
		m.out.Parents = append(m.out.Parents, ir.Parent{ParentID: pid, ChildID: c, SrcID: b.srcID(p.Pos)})
	}
	return pid
}

// life emits @x.born / @x.died. Inside a container without `when` the
// point joins the statement order; otherwise only its `when` places it.
func (b *builder) life(m *module, l *syntax.Life, w *walker) string {
	who := b.want(m, l.Who, l.Kind+" subject", "character")
	if who == "" {
		return ""
	}
	pt := who + lifeSuffix(l.Kind)
	kind := map[string]string{"born": "born", "dies": "died"}[l.Kind]
	if l.When == nil && w != nil {
		w.point(pt, kind, l.Pos)
		return pt
	}
	b.addPoint(m, pt, kind, l.Pos)
	if l.When != nil {
		b.when(m, l.When, pt, pt)
	}
	return pt
}
