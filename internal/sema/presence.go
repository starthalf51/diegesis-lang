package sema

import (
	"slices"

	"story-lang/internal/ir"
	"story-lang/internal/syntax"
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

// expand returns the entity followed by its members (groups).
func (b *builder) expand(id string) []string {
	return append([]string{id}, b.members[id]...)
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
	targets := append(slices.Clone(a.Objects), a.To...)
	switch {
	case a.Verb == "whispers":
		return dedup(targets)
	case !speech[a.Verb]:
		return nil
	case w.stage == nil:
		return dedup(targets)
	}
	self := map[string]bool{a.ActorID: true}
	for _, e := range w.b.members[a.ActorID] {
		self[e] = true
	}
	for g, ms := range w.b.members {
		if slices.Contains(ms, a.ActorID) {
			self[g] = true
		}
	}
	var out []string
	for _, e := range w.stage.order {
		if w.stage.on[e] && !self[e] {
			out = append(out, e)
		}
	}
	return out
}

func dedup(ids []string) []string {
	var out []string
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
