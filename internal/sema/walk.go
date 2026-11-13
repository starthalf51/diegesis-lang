package sema

import (
	"fmt"
	"slices"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

// walker emits one container's body in statement order.
type walker struct {
	b     *builder
	m     *module
	c     *cont
	row   int // index in m.out.Scenes
	place string
	last  string // last point emitted
	nact  int
	beat  string
	stage *stage // nil off stage (events, accounts)
}

func (b *builder) container(m *module, id syntax.Ident, kind, title, place, thread string, body []syntax.Node,
	pos syntax.Pos, ord int) string {
	c := m.conts[id.Name]
	if c == nil {
		return ""
	}
	m.out.Scenes = append(m.out.Scenes, ir.Scene{ID: c.id, WorkID: b.work, ModuleID: m.id, Kind: kind, Title: title,
		PlaceID: place, ThreadID: thread, StartPt: c.start, EndPt: c.end, DiscourseOrd: ord, Shown: kind == "scene",
		SrcID: b.srcID(pos)})
	w := &walker{b: b, m: m, c: c, row: len(m.out.Scenes) - 1, place: place}
	if kind == "scene" {
		w.stage = newStage()
	}
	w.point(c.start, "scene_start", pos)
	w.body(body)
	w.point(c.end, "scene_end", pos)
	w.closeAll()
	return c.id
}

// point emits a point after the previous one in statement order.
func (w *walker) point(id, kind string, pos syntax.Pos) {
	w.b.addPoint(w.m, id, kind, pos)
	if w.last != "" {
		w.b.edge(w.m, w.last, id, i64(0), nil, pos)
	}
	w.last = id
	w.closePending(id)
}

func (w *walker) body(body []syntax.Node) {
	w.b.stmts(w.m, body, w.stmt)
}

func (w *walker) stmt(n syntax.Node) (string, string) {
	b, m, c := w.b, w.m, w.c
	switch n := n.(type) {
	case *syntax.When:
		b.when(m, n, c.start, c.end)
		return "when", c.id
	case *syntax.Presence:
		w.presence(n)
		return "presence", c.id
	case *syntax.Beat:
		id := c.beats[n.ID.Name]
		w.point(id, "beat", n.Pos)
		prev := w.beat
		w.beat = id
		w.body(n.Body)
		w.beat = prev
		return "beat", id
	case *syntax.Act:
		return "act", w.act(n)
	case *syntax.Belief:
		b.belief(m, n, b.want(m, n.Who, "believer", "character", "group"), c.id, w.last)
		return "belief", c.id
	case *syntax.Reveal:
		r := ir.Reveal{PropID: b.prop(m, n.Prop), SceneID: c.id, AtPt: w.last, SrcID: b.srcID(n.Pos)}
		if n.At != nil {
			r.AtPt, _ = b.tref(m, n.At)
		}
		m.out.Reveals = append(m.out.Reveals, r)
		return "reveal", c.id
	case *syntax.Withhold:
		until, _ := b.tref(m, &n.Until)
		m.out.Withholds = append(m.out.Withholds, ir.Withhold{PropID: b.prop(m, n.Prop), SceneID: c.id, UntilPt: until,
			SrcID: b.srcID(n.Pos)})
		return "withhold", c.id
	case *syntax.Render:
		for _, e := range n.Entries {
			vals := append([]string{}, e.Values...)
			if e.Key == "pov" { // the stored value stays as written; only checked here
				for _, v := range e.Values[:1] {
					b.want(m, syntax.Ident{Pos: e.Pos, Name: v}, "pov character", "character")
				}
			}
			if e.Key == "narration" && (len(e.Values) != 1 || (e.Values[0] != "first-person" && e.Values[0] != "third-person")) {
				b.errf(e.Pos, "arg-kind", "narration takes first-person or third-person")
				continue
			}
			way := ""
			if e.Key == "explain" || e.Key == "recap" { // values are term ids
				vals = []string{}
				ids := e.Values
				if i := slices.Index(ids, "via"); e.Key == "explain" && i >= 0 { // explain <id>… via <way>
					if i == 0 || len(ids) != i+2 || !slices.Contains([]string{"action", "dialogue", "consequence", "sense"}, ids[i+1]) {
						b.errf(e.Pos, "arg-kind", "explain takes term ids, optionally followed by via action, dialogue, consequence or sense")
						continue
					}
					ids, way = ids[:i], ids[i+1]
				}
				for _, v := range ids {
					if id := b.want(m, syntax.Ident{Pos: e.Pos, Name: v}, "explained term", "term"); id != "" {
						vals = append(vals, id)
					}
				}
			}
			m.out.Directives = append(m.out.Directives, ir.Directive{SceneID: c.id, Key: e.Key,
				Values: vals, SrcID: b.srcID(e.Pos)})
			if way != "" {
				m.out.Directives = append(m.out.Directives, ir.Directive{SceneID: c.id, Key: "explain.via",
					Values: append([]string{way}, vals...), SrcID: b.srcID(e.Pos)})
			}
		}
		return "directive", c.id
	case *syntax.Design:
		for _, e := range n.Entries {
			vals := append([]string{}, e.Values...)
			switch e.Key {
			case "goal":
				id := b.want(m, syntax.Ident{Pos: e.Pos, Name: vals[0]}, "goal character", "character")
				if id == "" {
					continue
				}
				vals[0] = id
			case "tempo":
				if !slices.Contains([]string{"scene", "summary", "ellipsis"}, vals[0]) {
					b.errf(e.Pos, "arg-kind", "tempo takes scene, summary or ellipsis")
					continue
				}
			}
			m.out.Directives = append(m.out.Directives, ir.Directive{SceneID: c.id, Key: "design." + e.Key,
				Values: vals, SrcID: b.srcID(e.Pos)})
		}
		return "directive", c.id
	case *syntax.Setup:
		return "setup", b.setup(m, n, c.id, w.last)
	case *syntax.Payoff:
		sid := b.setupRef(m, n.Ref)
		m.out.Payoffs = append(m.out.Payoffs, ir.Payoff{SetupID: sid, SceneID: c.id, AtPt: w.last, ModuleID: m.id,
			SrcID: b.srcID(n.Pos)})
		return "payoff", sid
	case *syntax.Discourse:
		if x := b.findCont(m, n.After.Path); x != nil && x.kind == "scene" {
			m.out.Scenes[w.row].DiscourseAfter = x.id
		} else {
			b.errf(n.After.Pos, "undefined-ref", "undefined scene %q", n.After.Path)
		}
		return "scene", c.id
	case *syntax.Parent:
		return "parent", b.parent(m, n)
	case *syntax.Life:
		return "life", b.life(m, n, w)
	case *syntax.Attribute:
		acc := ""
		if c.kind == "account" {
			acc = c.id
		}
		return "attribute", b.attribute(m, n, c.id, w.last, acc)
	case *syntax.PatternDecl:
		return "pattern", b.patternDecl(m, n, c.id)
	}
	return "", ""
}

// setupRef resolves `payoff [module/]setup`.
func (b *builder) setupRef(m *module, ref syntax.Ident) string {
	t, local := m, ref.Name
	for i := len(ref.Name) - 1; i > 0; i-- {
		if ref.Name[i] == '/' {
			t, local = b.mods[ref.Name[:i]], ref.Name[i+1:]
			break
		}
	}
	if t != nil {
		if id, ok := t.setups[local]; ok {
			return id
		}
	}
	b.errf(ref.Pos, "undefined-ref", "undefined setup %q", ref.Name)
	return ""
}

// object kinds per verb ("" = any declared name)
func objectKinds(verb string) []string {
	switch verb {
	case "reads", "writes":
		return []string{"artifact"}
	case "sees": // a fact object drops out of act as content. What remains must be an entity
		return []string{"character", "group", "place", "thing", "faction", "thread"}
	case "does":
		return nil
	case "shows": // what is shown is a person or an object/account. An object is not an audience (presence.go)
		return []string{"character", "group", "thing", "artifact"}
	}
	return []string{"character", "group"}
}

func (w *walker) act(a *syntax.Act) string {
	b, m := w.b, w.m
	w.nact++
	id := fmt.Sprintf("%s#a%d", w.c.id, w.nact)
	var actor string
	if a.Role {
		actor = b.wantRole(m, a.Actor)
	} else {
		actor = b.want(m, a.Actor, "actor", "character", "group")
	}
	// `X sees p`: a fact (or alias) object is the content seen (moved to with so derive makes the perception belief)
	objs, seen := a.Objects, []string{}
	if a.Verb == "sees" {
		objs = nil
		for _, o := range a.Objects {
			if d, ok := b.lookup(m, o.Name); ok && d.kind == "fact" {
				seen = append(seen, d.id)
			} else {
				objs = append(objs, o)
			}
		}
	}
	row := ir.Act{ID: id, SceneID: w.c.id, BeatID: w.beat, Ord: w.nact, Pt: id, ActorID: actor, ActorIsRole: a.Role,
		Verb: a.Verb, Action: a.Action, Objects: b.wantAll(m, objs, a.Verb+" object", objectKinds(a.Verb)...),
		To: b.wantAll(m, a.To, "recipient", "character", "group"), With: append(b.props(m, a.With), seen...), PropID: b.prop(m, a.Prop),
		Quote: a.Quote, BecauseID: b.prop(m, a.Because), SrcRef: a.Src, SrcID: b.srcID(a.Pos)}
	w.point(id, "act", a.Pos)
	m.out.Acts = append(m.out.Acts, row)
	if w.stage != nil && !a.Role && actor != "" && !w.stage.on[actor] {
		b.warn(a.Actor.Pos, "offstage-speaker", "%q %s but is not on stage", a.Actor.Name, a.Verb)
	}
	for _, e := range w.audience(row) {
		m.out.Audience = append(m.out.Audience, ir.Audience{ActID: id, EntityID: e})
	}
	return id
}

// belief emits one `believes`/`doubts` row. scene/at "" = background
// knowledge (top level or a declaration block): held from the start.
func (b *builder) belief(m *module, n *syntax.Belief, who, scene, at string) {
	bl := ir.Belief{EntityID: who, Verb: n.Verb, PropID: b.prop(m, n.Prop), SceneID: scene, AtPt: at, SrcID: b.srcID(n.Pos)}
	if n.Because != nil {
		bl.BecausePt, _ = b.tref(m, n.Because)
	}
	m.out.Beliefs = append(m.out.Beliefs, bl)
}
