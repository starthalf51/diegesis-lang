package sema

import (
	"strings"

	"story-lang/internal/ir"
	"story-lang/internal/syntax"
)

var entityKinds = map[string]bool{"character": true, "group": true, "place": true, "thing": true,
	"faction": true, "thread": true}

// collect is pass 1: imports, declarations, containers (with beats),
// setups, discourse order and the time origin.
func (b *builder) collect(files []*syntax.File) {
	for _, m := range b.order {
		b.cur = m
		for _, f := range m.files {
			b.file = f.Path
			for _, im := range f.Imports {
				t := b.mods[im.Name]
				if t == nil {
					b.errf(im.Pos, "undefined-ref", "undefined module %q", im.Name)
					continue
				}
				if _, dup := m.importPos[t]; !dup {
					m.imports = append(m.imports, t)
					m.importPos[t], m.importF[t] = im.Pos, f.Path
				}
			}
			for _, it := range f.Items {
				b.collectItem(m, it)
			}
		}
	}
	var dates []string
	var start *syntax.Start
	date := func(s syntax.Node) {
		if l, ok := s.(*syntax.Life); ok && l.When != nil {
			s = l.When
		}
		if w, ok := s.(*syntax.When); ok && w.Op == "at" {
			dates = append(dates, w.Date)
		}
	}
	for _, f := range files { // discourse order = file order
		for _, it := range f.Items {
			date(it)
			eachStmt(bodyOf(it), date)
			switch n := it.(type) {
			case *syntax.Scene:
				b.dord[n] = len(b.dord) + 1
			case *syntax.Work:
				for _, a := range n.Attrs {
					if s, ok := a.(*syntax.Start); ok && start == nil {
						start = s
					}
				}
			}
		}
	}
	b.setEpoch(start, dates)
}

func (b *builder) declare(m *module, id syntax.Ident, kind string) {
	if _, dup := m.decls[id.Name]; dup {
		b.errf(id.Pos, "dup-decl", "%q is declared twice in module %s", id.Name, m.name)
		return
	}
	full := m.id + "/" + id.Name
	m.decls[id.Name] = decl{kind, full}
	b.kinds[full] = kind
}

func (b *builder) collectItem(m *module, it syntax.Node) {
	var c *cont
	var body []syntax.Node
	switch n := it.(type) {
	case *syntax.Decl:
		b.declare(m, n.ID, n.Kind)
	case *syntax.Scene:
		c, body = b.addCont(m, n.ID, "scene"), n.Body
	case *syntax.Event:
		c, body = b.addCont(m, n.ID, "event"), n.Body
	case *syntax.Account:
		c, body = b.addCont(m, n.ID, "account"), n.Body
	case *syntax.Setup:
		b.addSetup(m, n)
	}
	if c == nil {
		return
	}
	eachStmt(body, func(s syntax.Node) {
		switch n := s.(type) {
		case *syntax.Beat:
			if _, dup := c.beats[n.ID.Name]; dup {
				b.errf(n.ID.Pos, "dup-decl", "beat %q is declared twice in %s", n.ID.Name, c.id)
			}
			c.beats[n.ID.Name] = c.id + "#" + n.ID.Name
		case *syntax.Setup:
			b.addSetup(m, n)
		}
	})
}

func (b *builder) addCont(m *module, id syntax.Ident, kind string) *cont {
	if _, dup := m.conts[id.Name]; dup {
		b.errf(id.Pos, "dup-decl", "%s %q is declared twice in module %s", kind, id.Name, m.name)
		return nil
	}
	full := m.id + "/" + id.Name
	c := &cont{id: full, kind: kind, start: full + ".start", end: full + ".end", beats: map[string]string{}}
	m.conts[id.Name] = c
	return c
}

func (b *builder) addSetup(m *module, s *syntax.Setup) {
	if _, dup := m.setups[s.ID.Name]; dup {
		b.errf(s.ID.Pos, "dup-decl", "setup %q is declared twice in module %s", s.ID.Name, m.name)
	}
	m.setups[s.ID.Name] = m.id + "/" + s.ID.Name
}

// eachStmt visits every statement, descending into beats.
func eachStmt(body []syntax.Node, fn func(syntax.Node)) {
	for _, n := range body {
		fn(n)
		if bt, ok := n.(*syntax.Beat); ok {
			eachStmt(bt.Body, fn)
		}
	}
}

// cycles reports each import cycle once, at the import that closes it.
func (b *builder) cycles() {
	state := map[*module]int{} // 0 new, 1 on stack, 2 done
	var visit func(m *module)
	visit = func(m *module) {
		state[m] = 1
		for _, t := range m.imports {
			switch state[t] {
			case 0:
				visit(t)
			case 1:
				b.cur, b.file = m, m.importF[t]
				b.errf(m.importPos[t], "import-cycle", "import %s closes a cycle back to %s", t.name, t.name)
			}
		}
		state[m] = 2
	}
	for _, m := range b.order {
		if state[m] == 0 {
			visit(m)
		}
	}
}

// second is pass 1.5: group members and life points, which need every
// declaration in place.
func (b *builder) second() {
	for _, m := range b.order {
		b.cur = m
		for _, f := range m.files {
			b.file = f.Path
			for _, it := range f.Items {
				switch n := it.(type) {
				case *syntax.Decl:
					gid := m.id + "/" + n.ID.Name
					for _, a := range n.Attrs {
						ms, ok := a.(*syntax.Members)
						if !ok {
							continue
						}
						if n.Kind != "group" {
							b.errf(ms.Pos, "arg-kind", "members only applies to a group, %q is a %s", n.ID.Name, n.Kind)
							continue
						}
						for _, id := range ms.IDs {
							if mid := b.want(m, id, "member", "character", "group"); mid != "" {
								b.members[gid] = append(b.members[gid], mid)
								m.out.GroupMembers = append(m.out.GroupMembers, ir.GroupMember{GroupID: gid, MemberID: mid, SrcID: b.srcID(id.Pos)})
							}
						}
					}
				case *syntax.Life:
					b.addLife(m, n)
				case *syntax.Scene, *syntax.Event, *syntax.Account:
					eachStmt(bodyOf(it), func(s syntax.Node) {
						if l, ok := s.(*syntax.Life); ok {
							b.addLife(m, l)
						}
					})
				}
			}
		}
	}
}

func bodyOf(n syntax.Node) []syntax.Node {
	switch n := n.(type) {
	case *syntax.Scene:
		return n.Body
	case *syntax.Event:
		return n.Body
	case *syntax.Account:
		return n.Body
	}
	return nil
}

func (b *builder) addLife(m *module, l *syntax.Life) {
	d, ok := b.lookup(m, l.Who.Name)
	if !ok || d.kind != "character" {
		return // reported in pass 2
	}
	pt := d.id + lifeSuffix(l.Kind)
	if b.lives[pt] {
		b.errf(l.Pos, "dup-decl", "%s %s is declared twice", l.Kind, l.Who.Name)
	}
	b.lives[pt] = true
}

func lifeSuffix(kind string) string {
	if kind == "born" {
		return ".born"
	}
	return ".died"
}

// lookup resolves a name in the module, then its imports. `mod/local`
// names a module explicitly.
func (b *builder) lookup(m *module, name string) (decl, bool) {
	if i := strings.LastIndex(name, "/"); i > 0 {
		t := b.mods[name[:i]]
		if t == nil {
			return decl{}, false
		}
		d, ok := t.decls[name[i+1:]]
		return d, ok
	}
	if d, ok := m.decls[name]; ok {
		return d, true
	}
	for _, t := range m.imports {
		if d, ok := t.decls[name]; ok {
			return d, true
		}
	}
	return decl{}, false
}

// findCont resolves a scene/event/account id: own module, imports, then
// the whole work (scene numbers are work-wide).
func (b *builder) findCont(m *module, name string) *cont {
	if i := strings.LastIndex(name, "/"); i > 0 {
		if t := b.mods[name[:i]]; t != nil {
			return t.conts[name[i+1:]]
		}
		return nil
	}
	if c := m.conts[name]; c != nil {
		return c
	}
	for _, t := range m.imports {
		if c := t.conts[name]; c != nil {
			return c
		}
	}
	for _, t := range b.order {
		if c := t.conts[name]; c != nil {
			return c
		}
	}
	return nil
}

// tref resolves a time reference to the start and end points it names
// (equal for a single point). "" on error.
func (b *builder) tref(m *module, t *syntax.Tref) (string, string) {
	p := t.Path
	if p == "start" {
		return b.origin, b.origin
	}
	for _, suf := range []string{".start", ".end"} {
		if c := b.findCont(m, strings.TrimSuffix(p, suf)); strings.HasSuffix(p, suf) && c != nil {
			if suf == ".start" {
				return c.start, c.start
			}
			return c.end, c.end
		}
	}
	for _, suf := range []string{".born", ".died"} {
		if d, ok := b.lookup(m, strings.TrimSuffix(p, suf)); strings.HasSuffix(p, suf) && ok && b.lives[d.id+suf] {
			return d.id + suf, d.id + suf
		}
	}
	if c := b.findCont(m, p); c != nil {
		return c.start, c.end
	}
	if i := strings.LastIndex(p, "."); i > 0 {
		if c := b.findCont(m, p[:i]); c != nil && c.beats[p[i+1:]] != "" {
			bt := c.beats[p[i+1:]]
			return bt, bt
		}
	}
	b.errf(t.Pos, "undefined-ref", "undefined time reference %q", trefText(t))
	return "", ""
}

func trefText(t *syntax.Tref) string {
	if t == nil {
		return ""
	}
	if t.At {
		return "@" + t.Path
	}
	return t.Path
}
