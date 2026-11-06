package mutate

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"diegesis/internal/ir"
)

// Apply 는 w 의 복사본에 m 을 적용한다(w 는 바꾸지 않는다). 대상이 없으면 복사본 그대로다.
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
		// 그 행위를 그 정보의 출처 시점들(m.Before, 하네스가 PG 에서 찾는다)보다 1초 앞으로 옮긴다.
		// 출처를 모르면 옮기지 않는다.
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
		// 죽음을 그 인물의 마지막 행위보다 1초 앞으로 옮긴다.
		last := index(c).lastAct(strings.TrimSuffix(m.Target, ".died"))
		isolate(c, m.Target)
		c.TimeEdges = append(c.TimeEdges, ir.TimeEdge{From: origin(c), To: m.Target, Min: i64(0)},
			ir.TimeEdge{From: m.Target, To: last, Min: i64(1)})
	case "shift-duration", "swap-order":
		// 대상 간선(그 두 끝점의 첫 문장 간선) 하나를 +Shift 하거나 뒤집는다(나머지 제약은 그대로 둔다).
		from, to, _ := strings.Cut(m.Target, " ")
		for i, e := range c.TimeEdges {
			if e.From != from || e.To != to || e.Waived || e.TargetID == "" {
				continue
			}
			if m.Op == "swap-order" {
				// 뒤집기: 끝점을 맞바꾸되 장면·사건의 경계는 반대쪽 끝으로(`C before X` 의 C.end→X.start 는 X.end→C.start,
				// 곧 `C after X`). born→died 같은 한 시점은 그대로 맞바꾼다.
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
		panic(err) // ir.Work 는 언제나 JSON 이 된다
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

// isolate 는 시점 pt 에 닿는 간선을 모두 떼고, x→pt→y 를 x→y 로 이어 다른 시점끼리의 순서를 지킨다.
// 억제된 간선은 풀이에서 빠지므로 잇지 않는다.
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

// negate 는 ¬p 의 fact ID. p 가 `not q` 면 q, 이미 있는 `not p` fact 면 그것, 아니면 새로 만든다.
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

// Rename 은 작품 ID 를 바꾼 복사본(모든 ID 의 `work/` 접두사와 work_id). 변이본을 원본과 함께 적재할 때 쓴다.
func Rename(w *ir.Work, id string) *ir.Work {
	old := w.Works[0].ID
	b, err := json.Marshal(w)
	if err != nil {
		panic(err)
	}
	// ID 는 따옴표 바로 뒤에도, 복합 명제 안(`holds(w/…,w/…)`, `not w/…`)에도 있다.
	b = regexp.MustCompile(`(["(, ])`+regexp.QuoteMeta(old)+`/`).ReplaceAll(b, []byte(`${1}`+id+`/`))
	b = bytes.ReplaceAll(b, []byte(`"`+old+`"`), []byte(`"`+id+`"`))
	var c ir.Work
	if err := json.Unmarshal(b, &c); err != nil {
		panic(err)
	}
	return &c
}
