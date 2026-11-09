package sema

import (
	"slices"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

// want resolves a name and checks its kind (any kind when none given).
// undefined-ref / arg-kind on failure, returning "".
func (b *builder) want(m *module, id syntax.Ident, what string, kinds ...string) string {
	d, ok := b.lookup(m, id.Name)
	if !ok {
		b.errf(id.Pos, "undefined-ref", "undefined %s %q", what, id.Name)
		return ""
	}
	if len(kinds) > 0 && !slices.Contains(kinds, d.kind) {
		b.errf(id.Pos, "arg-kind", "%s %q is a %s, want %s", what, id.Name, d.kind, strings.Join(kinds, " or "))
		return ""
	}
	return d.id
}

func (b *builder) wantAll(m *module, ids []syntax.Ident, what string, kinds ...string) []string {
	out := []string{}
	for _, id := range ids {
		if r := b.want(m, id, what, kinds...); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// wantRole: a `role:x` / holds(x, …) slot must name a declared role.
func (b *builder) wantRole(m *module, id syntax.Ident) string {
	d, ok := b.lookup(m, id.Name)
	if !ok || d.kind != "role" {
		b.errf(id.Pos, "role-unknown", "%q is not a declared role", id.Name)
		return ""
	}
	return d.id
}

// prop returns the fact id of a top-level proposition ("" for none/error).
func (b *builder) prop(m *module, p *syntax.Prop) string {
	if p == nil {
		return ""
	}
	id, _, _ := b.propDef(m, p, true)
	return id
}

func (b *builder) props(m *module, ps []*syntax.Prop) []string {
	out := []string{}
	for _, p := range ps {
		if id := b.prop(m, p); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// propDef resolves a proposition. Names resolve to their declaration
// (top level: must be a fact); calls and negations become derived facts with
// id work/<canonical text>, the text using resolved ids so equal
// propositions share one row. Returns id, canonical text, ok.
func (b *builder) propDef(m *module, p *syntax.Prop, top bool) (string, string, bool) {
	var id, canon string
	switch {
	case p.Call:
		args, canons := make([]string, len(p.Args)), make([]string, len(p.Args))
		ok := true
		for i, a := range p.Args {
			var good bool
			args[i], canons[i], good = b.propDef(m, a, false)
			ok = ok && good
		}
		b.checkPred(m, p, args)
		if !ok {
			return "", "", false
		}
		canon = p.Name + "(" + strings.Join(canons, ",") + ")"
		id = b.work + "/" + canon
		b.fact(id, &ir.PropDef{Pred: p.Name, Args: args}, p.Pos)
	case top:
		id = b.want(m, syntax.Ident{Pos: p.Pos, Name: p.Name}, "proposition", "fact")
		canon = id
	default:
		id = b.want(m, syntax.Ident{Pos: p.Pos, Name: p.Name}, "argument")
		canon = id
	}
	if id == "" {
		return "", "", false
	}
	if p.Not {
		inner := id
		canon = "not " + canon
		id = b.work + "/" + canon
		b.fact(id, &ir.PropDef{Not: inner}, p.Pos)
	}
	return id, canon, true
}

// builtin predicate arities and slot kinds ("" = any).
var preds = map[string][]string{"holds": {"role", ""}, "alive": {""}, "at": {"", "place"}}

func (b *builder) checkPred(m *module, p *syntax.Prop, args []string) {
	kinds, ok := preds[p.Name]
	if !ok {
		return // user predicate: free form, PG interprets
	}
	if len(args) != len(kinds) {
		b.errf(p.Pos, "arg-kind", "%s takes %d arguments, got %d", p.Name, len(kinds), len(args))
		return
	}
	for i, k := range kinds {
		a := p.Args[i]
		if k == "" || args[i] == "" || a.Call || a.Not {
			continue
		}
		if got := b.kinds[args[i]]; got != k {
			if k == "role" {
				b.errf(a.Pos, "role-unknown", "%s needs a role, %q is a %s", p.Name, a.Name, got)
			} else {
				b.errf(a.Pos, "arg-kind", "%s argument %q is a %s, want %s", p.Name, a.Name, got, k)
			}
		}
	}
}

// fact emits a derived fact row once per work.
func (b *builder) fact(id string, def *ir.PropDef, pos syntax.Pos) {
	if b.facts[id] {
		return
	}
	b.facts[id] = true
	b.g.Facts = append(b.g.Facts, ir.Fact{ID: id, WorkID: b.work, Text: strings.TrimPrefix(id, b.work+"/"),
		Truth: "derived", Def: def, SrcID: b.srcID(pos)})
}
