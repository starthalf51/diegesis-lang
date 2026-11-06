package syntax

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var verbs = map[string]bool{"tells": true, "says": true, "hails": true, "asks": true, "lies": true,
	"whispers": true, "aside": true, "thinks": true, "sees": true, "shows": true, "reads": true,
	"writes": true, "sends": true, "orders": true, "decrees": true, "does": true}

var rkeys = map[string]bool{"pov": true, "pace": true, "tension": true, "focus": true, "omit": true, "show": true}

// words that end an act's idlist / start its next clause
var actStop = map[string]bool{"to": true, "with": true, "because": true, "src": true}

// scene sid [at ident] [in ident] [BLOCK(stmt)]
func (c *cur) scene() Node {
	s := &Scene{Pos: c.peek().Pos}
	c.k++
	s.ID = c.ident("scene id")
	if c.accept("at") {
		id := c.ident("place")
		s.At = &id
	}
	if c.accept("in") {
		id := c.ident("thread")
		s.In = &id
	}
	c.end()
	c.p.declare("scene", s.ID)
	s.Body = c.children((*cur).stmt)
	return s
}

// event ident [string] [BLOCK(stmt)]
func (c *cur) event() Node {
	e := &Event{Pos: c.peek().Pos}
	c.k++
	e.ID = c.ident("event id")
	e.Title, _ = c.str()
	c.end()
	c.p.declare("event", e.ID)
	e.Body = c.children((*cur).stmt)
	return e
}

// account ident of qname by ident [BLOCK(stmt)]
func (c *cur) account() Node {
	a := &Account{Pos: c.peek().Pos}
	c.k++
	a.ID = c.ident("account id")
	c.expect("of")
	a.Of = c.ident("event")
	c.expect("by")
	a.By = c.ident("source")
	c.end()
	c.p.declare("account", a.ID)
	a.Body = c.children((*cur).stmt)
	return a
}

// setup ident promise|hint|symbol : (prop | string)
func (c *cur) setup() Node {
	s := &Setup{Pos: c.peek().Pos}
	c.k++
	s.ID = c.ident("setup id")
	s.Kind = c.oneOf("setup kind", "promise", "hint", "symbol")
	c.expect(":")
	if txt, ok := c.str(); ok {
		s.Text = txt
	} else {
		s.Prop = c.prop()
	}
	c.end()
	c.p.declare("setup", s.ID)
	return s
}

func (c *cur) stmt() Node {
	t := c.peek()
	if t.Kind == tPragma {
		return c.pragma()
	}
	if t.Kind != tWord {
		c.fail("expected a statement, found %s", describe(t))
	}
	if c.isAttribute() {
		return c.attribute()
	}
	var n Node
	switch t.Val {
	case "when":
		n = c.when()
	case "lasts": // 장면·사건의 길이: start→end 간선 하나(AST 는 When{Op: "lasts"})
		c.k++
		w := &When{Pos: t.Pos, Op: "lasts"}
		lo, hi := c.durRange()
		w.Min, w.Max = &lo, &hi
		n = w
	case "enter", "exit", "mention":
		c.k++
		pr := &Presence{Pos: t.Pos, Op: t.Val}
		if t.Val == "exit" && c.accept("all") {
			pr.All = true
		} else {
			pr.IDs = c.idlist()
		}
		n = pr
	case "beat":
		c.k++
		b := &Beat{Pos: t.Pos, ID: c.ident("beat id")}
		c.expect(":")
		c.end()
		b.Body = c.children((*cur).stmt)
		return b
	case "render":
		c.k++
		c.end()
		r := &Render{Pos: t.Pos}
		for _, e := range c.children((*cur).rentry) {
			r.Entries = append(r.Entries, e.(*RenderEntry))
		}
		return r
	case "reveal":
		c.k++
		r := &Reveal{Pos: t.Pos, Prop: c.prop()}
		if c.accept("at") {
			r.At = c.tref()
		}
		n = r
	case "withhold":
		c.k++
		w := &Withhold{Pos: t.Pos, Prop: c.prop()}
		c.expect("until")
		w.Until = *c.tref()
		n = w
	case "setup":
		return c.setup()
	case "payoff":
		c.k++
		n = &Payoff{t.Pos, c.ident("setup reference")}
	case "discourse":
		c.k++
		c.expect("after")
		n = &Discourse{t.Pos, *c.tref()}
	case "parent":
		return c.parent()
	case "born", "dies":
		return c.life()
	default:
		next := c.peekN(1)
		switch {
		case t.Val == "role" && next != nil && next.Val == ":":
			return c.act()
		case next != nil && next.Kind == tWord && verbs[next.Val]:
			return c.act()
		case next != nil && next.Kind == tWord && (next.Val == "believes" || next.Val == "doubts"):
			return c.belief()
		case topOnly[t.Val] || declKinds[t.Val]:
			c.fail("%q is not allowed inside a block", t.Val)
		case next != nil && next.Kind == tWord:
			c.fail("unknown keyword %q or verb %q", t.Val, next.Val)
		default:
			c.fail("unknown keyword %q", t.Val)
		}
	}
	c.end()
	return n
}

func (c *cur) rentry() Node {
	t := c.peek()
	if t.Kind != tWord || !rkeys[t.Val] {
		c.fail("unknown render key %s", describe(t))
	}
	c.k++
	e := &RenderEntry{Pos: t.Pos, Key: t.Val}
	for c.peek() != nil {
		v := c.peek()
		if v.Kind != tWord && v.Kind != tString {
			c.fail("unexpected %s in render value", describe(v))
		}
		e.Values = append(e.Values, v.Val)
		c.k++
		c.accept(",")
	}
	if len(e.Values) == 0 {
		c.fail("render %s needs a value", t.Val)
	}
	return e
}

// actor verb [action] [idlist] [to idlist] [with proplist] [: [prop]] [string] [because prop] [src ref]
func (c *cur) act() Node {
	a := &Act{Pos: c.peek().Pos}
	a.Actor, a.Role = c.actor()
	a.Verb = c.word("verb").Val
	if a.Verb == "does" {
		a.Action = c.word("action").Val
	}
	if t := c.peek(); t != nil && t.Kind == tWord && !actStop[t.Val] {
		a.Objects = c.idlist()
	}
	if c.accept("to") {
		a.To = c.idlist()
	}
	if c.accept("with") {
		a.With = c.proplist()
	}
	if c.accept(":") {
		if t := c.peek(); t != nil && t.Kind == tWord && !actStop[t.Val] {
			a.Prop = c.prop()
		}
	}
	a.Quote, _ = c.str()
	if c.accept("because") {
		a.Because = c.prop()
	}
	if c.accept("src") {
		if s, ok := c.str(); ok {
			a.Src = s
		} else {
			a.Src = c.word("source reference").Val
		}
	}
	c.end()
	return a
}

// actor = ident | "role" ":" ident
func (c *cur) actor() (Ident, bool) {
	if c.is("role") && c.peekN(1) != nil && c.peekN(1).Val == ":" {
		c.k += 2
		return c.ident("role id"), true
	}
	return c.ident("actor"), false
}

// when (after|before) tref [by dur..dur] | at date [ident] | during tref
func (c *cur) when() *When {
	w := &When{Pos: c.peek().Pos}
	c.k++
	w.Op = c.oneOf("time relation", "after", "before", "at", "during")
	switch w.Op {
	case "at":
		w.Date = c.word("date").Val
		if t := c.peek(); t != nil && t.Kind == tWord {
			w.Time = c.word("time of day").Val
		}
	case "during":
		w.Ref = c.tref()
	default:
		w.Ref = c.tref()
		if c.accept("by") {
			lo, hi := c.durRange()
			w.Min, w.Max = &lo, &hi
		}
	}
	return w
}

func (c *cur) idlist() []Ident {
	ids := []Ident{c.ident("name")}
	for c.accept(",") {
		ids = append(ids, c.ident("name"))
	}
	return ids
}

// prop = ["not"] ident ["(" [prop {"," prop}] ")"]
func (c *cur) prop() *Prop {
	p := &Prop{Pos: c.here()}
	if c.accept("not") {
		p.Not = true
	}
	p.Name = c.word("proposition").Val
	if c.accept("(") {
		p.Call = true
		if !c.is(")") {
			p.Args = c.proplist()
		}
		c.expect(")")
	}
	return p
}

func (c *cur) proplist() []*Prop {
	ps := []*Prop{c.prop()}
	for c.accept(",") {
		ps = append(ps, c.prop())
	}
	return ps
}

func (c *cur) tref() *Tref {
	t := c.word("time reference")
	return &Tref{t.Pos, strings.HasPrefix(t.Val, "@"), strings.TrimPrefix(t.Val, "@")}
}

var durRe = regexp.MustCompile(`^([0-9]+)([ydhm])$`)

func (c *cur) dur() Dur {
	t := c.word("duration")
	m := durRe.FindStringSubmatch(t.Val)
	if m == nil {
		c.k--
		c.fail("bad duration %q (want <n>y|d|h|m)", t.Val)
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	unit := map[string]int64{"y": 365 * 86400, "d": 86400, "h": 3600, "m": 60}[m[2]]
	if err != nil || n > math.MaxInt64/unit {
		c.k--
		c.fail("duration %q out of range", t.Val)
	}
	return Dur{t.Pos, n, m[2], n * unit}
}

func (c *cur) durRange() (Dur, Dur) {
	lo := c.dur()
	c.expect("..")
	return lo, c.dur()
}

// belief: <who> believes|doubts prop [because tref] (장면 안, 또는 최상위 배경 지식)
func (c *cur) belief() Node {
	t, next := c.peek(), c.peekN(1)
	c.k += 2
	b := &Belief{Pos: t.Pos, Who: Ident{t.Pos, t.Val}, Verb: next.Val, Prop: c.prop()}
	if c.accept("because") {
		b.Because = c.tref()
	}
	c.end()
	return b
}
