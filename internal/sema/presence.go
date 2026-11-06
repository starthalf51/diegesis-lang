package sema

import (
	"slices"

	"diegesis/internal/ir"
	"diegesis/internal/syntax"
)

// stage is the presence state machine of one scene. A group on stage puts
// its members on stage too.
type stage struct {
	on      map[string]bool
	order   []string       // first-entry order, for stable audience rows
	open    map[string]int // entity → open presence row
	pending []int          // rows exited, waiting for the next point as to_pt
}

func newStage() *stage { return &stage{on: map[string]bool{}, open: map[string]int{}} }

// expand returns the entity followed by its members, depth first through
// nested groups (visited set: a membership cycle cannot loop).
func (b *builder) expand(id string) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(e string) {
		if seen[e] {
			return
		}
		seen[e] = true
		out = append(out, e)
		for _, mem := range b.members[e] {
			walk(mem)
		}
	}
	walk(id)
	return out
}

func (w *walker) presence(p *syntax.Presence) {
	b, m := w.b, w.m
	if p.Op == "mention" {
		b.wantAll(m, p.IDs, "mentioned name")
		return
	}
	if w.stage == nil {
		b.errf(p.Pos, "arg-kind", "%s only applies inside a scene", p.Op)
		return
	}
	s := w.stage
	if p.All {
		for _, e := range s.order {
			w.leave(e)
		}
		return
	}
	for _, ident := range p.IDs {
		id := b.want(m, ident, p.Op+" target", "character", "group")
		if id == "" {
			continue
		}
		if p.Op == "exit" && !s.on[id] {
			b.warn(ident.Pos, "offstage-exit", "%q exits but is not on stage", ident.Name)
			continue
		}
		for _, e := range b.expand(id) {
			if p.Op == "exit" {
				w.leave(e)
				continue
			}
			if s.on[e] {
				continue
			}
			s.on[e] = true
			if !slices.Contains(s.order, e) {
				s.order = append(s.order, e)
			}
			m.out.Presence = append(m.out.Presence, ir.Presence{EntityID: e, SceneID: w.c.id, PlaceID: w.place,
				FromPt: w.last, SrcID: b.srcID(ident.Pos)})
			s.open[e] = len(m.out.Presence) - 1
		}
	}
}

func (w *walker) leave(e string) {
	s := w.stage
	if !s.on[e] {
		return
	}
	s.on[e] = false
	s.pending = append(s.pending, s.open[e])
	delete(s.open, e)
}

func (w *walker) closePending(pt string) {
	if w.stage == nil {
		return
	}
	for _, i := range w.stage.pending {
		w.m.out.Presence[i].ToPt = pt
	}
	w.stage.pending = nil
}

func (w *walker) closeAll() {
	if w.stage == nil {
		return
	}
	for _, e := range w.stage.order {
		if i, ok := w.stage.open[e]; ok {
			w.m.out.Presence[i].ToPt = w.c.end
		}
	}
}

var speech = map[string]bool{"tells": true, "says": true, "hails": true, "lies": true, "asks": true,
	"shows": true, "orders": true, "decrees": true, "sends": true}

// audience: who receives an act. Speech reaches everyone on stage (off
// stage: the named recipients); whispers only the named recipients;
// aside, thinks, sees, reads, writes, does reach nobody in the story.
func (w *walker) audience(a ir.Act) []string {
	self := map[string]bool{}
	for _, e := range w.b.expand(a.ActorID) {
		self[e] = true
	}
	for g := range w.b.members {
		if g != a.ActorID && slices.Contains(w.b.expand(g), a.ActorID) {
			self[g] = true
		}
	}
	keep := func(ids []string) []string {
		var out []string
		for _, e := range ids {
			if !self[e] && !slices.Contains(out, e) {
				out = append(out, e)
			}
		}
		return out
	}
	if a.Verb == "whispers" || speech[a.Verb] && w.stage == nil {
		named := append(slices.Clone(a.Objects), a.To...)
		if a.Verb == "sends" {
			named = slices.Clone(a.Objects) // 받는 사람은 전령이 말해 줄 때 안다
		}
		var targets []string
		for _, t := range named {
			targets = append(targets, w.b.expand(t)...)
		}
		return keep(targets)
	}
	if !speech[a.Verb] {
		return nil
	}
	var on []string
	for _, e := range w.stage.order {
		if w.stage.on[e] {
			on = append(on, e)
		}
	}
	return keep(on)
}
