package mutate

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/ir"
)

// Apply applies m to a copy of w (w is not changed). If there is no target, the copy is returned as is.
func Apply(w *ir.Work, m Mutation) *ir.Work {
	c := clone(w)
	switch m.Op {
	case "drop-tells":
		c.Acts = slices.DeleteFunc(c.Acts, func(a ir.Act) bool { return a.ID == m.Target })
		c.Audience = slices.DeleteFunc(c.Audience, func(a ir.Audience) bool { return a.ActID == m.Target })
	case "drop-enter":
		e, s, _ := strings.Cut(m.Target, "@")
		inScene := map[string]bool{}
		for _, a := range c.Acts {
			inScene[a.ID] = a.SceneID == s
		}
		c.Presence = slices.DeleteFunc(c.Presence, func(p ir.Presence) bool { return p.EntityID == e && p.SceneID == s })
		c.Audience = slices.DeleteFunc(c.Audience, func(a ir.Audience) bool { return a.EntityID == e && inScene[a.ActID] })
	case "move-before":
		// Move that act 1 second before the source points of that information (m.Before, found by the harness in PG).
		// If the source is unknown, do not move it.
		if len(m.Before) == 0 {
			break
		}
		isolate(c, m.Target)
		c.TimeEdges = append(c.TimeEdges, ir.TimeEdge{From: origin(c), To: m.Target, Min: i64(0)})
		for _, s := range m.Before {
			c.TimeEdges = append(c.TimeEdges, ir.TimeEdge{From: m.Target, To: s, Min: i64(1)})
		}
	case "drop-setup":
		c.Setups = slices.DeleteFunc(c.Setups, func(s ir.Setup) bool { return s.ID == m.Target })
	case "drop-payoff":
		c.Payoffs = slices.DeleteFunc(c.Payoffs, func(p ir.Payoff) bool { return p.SetupID == m.Target })
	case "kill-early":
		// Move the death 1 second before that character's last act.
		last := index(c).lastAct(strings.TrimSuffix(m.Target, ".died"))
		isolate(c, m.Target)
		c.TimeEdges = append(c.TimeEdges, ir.TimeEdge{From: origin(c), To: m.Target, Min: i64(0)},
			ir.TimeEdge{From: m.Target, To: last, Min: i64(1)})
	case "shift-duration", "swap-order":
		// +Shift or flip one target edge (the edge of the first statement of its two endpoints) (the other constraints stay).
		from, to, _ := strings.Cut(m.Target, " ")
		for i, e := range c.TimeEdges {
			if e.From != from || e.To != to || e.Waived || e.TargetID == "" {
				continue
			}
			if m.Op == "swap-order" {
				// Flip: swap the endpoints, but scene/event boundaries go to the opposite end (`C before X`'s C.end→X.start becomes X.end→C.start,
				// i.e. `C after X`). A single point such as born→died is simply swapped.
				c.TimeEdges[i].From, c.TimeEdges[i].To = otherEnd(c, to), otherEnd(c, from)
			} else {
				c.TimeEdges[i].Min, c.TimeEdges[i].Max = sum(e.Min, i64(Shift)), sum(e.Max, i64(Shift))
			}
			break
		}
	case "flip-perceived":
		act, p, _ := strings.Cut(m.Target, " ")
		np := negate(c, p)
		for i := range c.Acts {
			if c.Acts[i].ID != act {
				continue
			}
			if c.Acts[i].PropID == p {
				c.Acts[i].PropID = np
			}
			for j, q := range c.Acts[i].With {
				if q == p {
					c.Acts[i].With[j] = np
				}
			}
		}
	}
	return c
}

func otherEnd(w *ir.Work, p string) string {
	for _, s := range w.Scenes {
		switch p {
		case s.StartPt:
			return s.EndPt
		case s.EndPt:
			return s.StartPt
		}
	}
	return p
}

func i64(v int64) *int64 { return &v }

func clone(w *ir.Work) *ir.Work {
	b, err := json.Marshal(w)
	if err != nil {
		panic(err) // an ir.Work can always be turned into JSON
	}
	var c ir.Work
	if err := json.Unmarshal(b, &c); err != nil {
		panic(err)
	}
	return &c
}

func origin(w *ir.Work) string {
	for _, p := range w.Points {
		if p.Kind == "origin" {
			return p.ID
		}
	}
	return ""
}

// isolate detaches every edge touching point pt and reconnects x→pt→y as x→y to preserve the order between the other points.
// Suppressed edges drop out of the solve, so they are not reconnected.
func isolate(w *ir.Work, pt string) {
	var in, out, keep []ir.TimeEdge
	for _, e := range w.TimeEdges {
		switch {
		case e.To == pt && e.From == pt:
		case e.To == pt:
			in = append(in, e)
		case e.From == pt:
			out = append(out, e)
		default:
			keep = append(keep, e)
		}
	}
	for _, a := range in {
		for _, b := range out {
			if a.Waived || b.Waived || a.From == b.To {
				continue
			}
			keep = append(keep, ir.TimeEdge{From: a.From, To: b.To, Min: sum(a.Min, b.Min), Max: sum(a.Max, b.Max), SrcID: a.SrcID})
		}
	}
	w.TimeEdges = keep
}

func sum(a, b *int64) *int64 {
	if a == nil || b == nil {
		return nil
	}
	return i64(*a + *b)
}

// negate is the fact ID of ¬p. If p is `not q`, q; if a `not p` fact already exists, that; otherwise create a new one.
func negate(w *ir.Work, p string) string {
	for _, f := range w.Facts {
		if f.ID == p && f.Def != nil && f.Def.Not != "" {
			return f.Def.Not
		}
	}
	for _, f := range w.Facts {
		if f.Def != nil && f.Def.Not == p {
			return f.ID
		}
	}
	id := w.Works[0].ID + "/not " + p
	w.Facts = append(w.Facts, ir.Fact{ID: id, WorkID: w.Works[0].ID, Text: "not " + p, Truth: "derived", Def: &ir.PropDef{Not: p}})
	return id
}

// Rename is a copy with the work ID changed (the `work/` prefix of every ID and work_id). Used when loading a mutant alongside the original.
func Rename(w *ir.Work, id string) *ir.Work {
	old := w.Works[0].ID
	b, err := json.Marshal(w)
	if err != nil {
		panic(err)
	}
	// IDs appear right after quotes and also inside compound propositions (`holds(w/…,w/…)`, `not w/…`).
	b = regexp.MustCompile(`(["(, ])`+regexp.QuoteMeta(old)+`/`).ReplaceAll(b, []byte(`${1}`+id+`/`))
	b = bytes.ReplaceAll(b, []byte(`"`+old+`"`), []byte(`"`+id+`"`))
	var c ir.Work
	if err := json.Unmarshal(b, &c); err != nil {
		panic(err)
	}
	return &c
}
