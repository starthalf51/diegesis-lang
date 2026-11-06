package syntax

import (
	"strconv"
	"strings"
)

// work ident [string] [BLOCK(wattr)]
func (c *cur) work() Node {
	w := &Work{Pos: c.peek().Pos}
	c.k++
	w.ID = c.ident("work id")
	w.Title, _ = c.str()
	c.end()
	c.p.declare("work", w.ID)
	w.Attrs = c.children(func(c *cur) Node {
		t := c.peek()
		switch {
		case t.Kind == tPragma:
			return c.pragma()
		case c.accept("unit"):
			u := &Unit{t.Pos, c.word("unit").Val}
			c.end()
			return u
		case c.accept("start"):
			s := &Start{t.Pos, c.word("date").Val}
			c.end()
			return s
		case c.is("name"):
			return c.name()
		}
		c.fail("unknown work attribute %s", describe(t))
		return nil
	})
	return w
}

// reserved cannot be entity ids: they start statements or clauses (spec/grammar.ebnf).
var reserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields("when lasts enter exit mention beat render reveal withhold setup payoff discourse " +
		"parent born dies all not to with because src major minor extra enacts") {
		reserved[w] = true
	}
}

// kind ident [string {"," string}] [tier] ["=" prop] [bool] [BLOCK(attr)]
func (c *cur) decl() Node {
	d := &Decl{Pos: c.peek().Pos, Kind: c.peek().Val}
	c.k++
	d.ID = c.ident(d.Kind + " id")
	if reserved[d.ID.Name] {
		c.p.errAt(d.ID.Pos, "syntax", "reserved word used as id: %q", d.ID.Name)
	}
	if s, ok := c.str(); ok {
		d.Names = append(d.Names, s)
		for c.accept(",") {
			s, ok := c.str()
			if !ok {
				c.fail("expected name string after \",\"")
			}
			d.Names = append(d.Names, s)
		}
	}
	if t := c.peek(); t != nil && (t.Val == "major" || t.Val == "minor" || t.Val == "extra") {
		d.Tier = c.word("tier").Val
	}
	if c.accept("=") {
		d.Def = c.prop()
	}
	if c.is("true") || c.is("false") {
		d.Value = c.word("bool").Val
	}
	if c.accept("tag") {
		d.Tag = c.word("tag name").Val
	}
	c.end()
	c.p.declare("decl", d.ID)
	d.Attrs = c.children((*cur).attr)
	return d
}

func (c *cur) attr() Node {
	t := c.peek()
	if t.Kind == tPragma {
		return c.pragma()
	}
	if c.isAttribute() {
		return c.attribute()
	}
	if s, ok := c.str(); ok {
		c.end()
		return &Text{t.Pos, s}
	}
	var n Node
	switch t.Val {
	case "name":
		return c.name()
	case "members":
		c.k++
		n = &Members{t.Pos, c.idlist()}
	case "knows":
		c.k++
		n = &Knows{t.Pos, c.oneOf("knowledge source", "oracle", "omniscient")}
	case "held":
		c.k++
		h := &Held{Pos: t.Pos}
		h.Holder, h.Role = c.actor()
		if c.accept("from") {
			h.From = c.tref()
			if c.accept("until") {
				h.Until = c.tref()
			}
		} else {
			c.expect("until")
			h.Until = c.tref()
		}
		n = h
	case "shared":
		c.k++
		n = &Shared{t.Pos}
	case "contains":
		c.k++
		n = &Contains{t.Pos, c.proplist()}
	case "believes", "doubts": // 선언한 인물·그룹의 배경 지식(Who 는 sema 가 채운다)
		c.k++
		n = &Belief{Pos: t.Pos, Verb: t.Val, Prop: c.prop()}
	default:
		c.fail("unknown attribute %s", describe(t))
	}
	c.end()
	return n
}

// name lang "text"
func (c *cur) name() Node {
	pos := c.peek().Pos
	c.k++
	lang := c.word("language code").Val
	s, ok := c.str()
	if !ok {
		c.fail("expected name string, found %s", describe(c.peek()))
	}
	c.end()
	return &Name{pos, lang, s}
}

// distance a b dur..dur
func (c *cur) distance() Node {
	d := &Distance{Pos: c.peek().Pos}
	c.k++
	d.A, d.B = c.ident("place"), c.ident("place")
	d.Min, d.Max = c.durRange()
	c.end()
	return d
}

// parent a of b, c
func (c *cur) parent() Node {
	n := &Parent{Pos: c.peek().Pos}
	c.k++
	n.Parent = c.ident("parent")
	c.expect("of")
	n.Children = c.idlist()
	c.end()
	return n
}

// born|dies x [when …]
func (c *cur) life() Node {
	n := &Life{Pos: c.peek().Pos, Kind: c.peek().Val}
	c.k++
	n.Who = c.ident("character")
	if c.is("when") {
		n.When = c.when()
	}
	c.end()
	return n
}

// isAttribute: "attr" starts an attribute unless it is an actor or believer named attr (attr tells b …).
func (c *cur) isAttribute() bool {
	if !c.is("attr") {
		return false
	}
	n := c.peekN(1)
	return n == nil || n.Kind != tWord || !verbs[n.Val] && n.Val != "believes" && n.Val != "doubts"
}

// attr <entity>.<key> = <int | string | id> [src (word | string)]
func (c *cur) attribute() Node {
	a := &Attribute{Pos: c.peek().Pos}
	c.k++
	t := c.word("<entity>.<key>")
	i := strings.LastIndex(t.Val, ".")
	if i <= 0 || i == len(t.Val)-1 {
		c.k--
		c.fail("expected <entity>.<key>, found %q", t.Val)
	}
	a.Entity, a.Key = Ident{t.Pos, t.Val[:i]}, t.Val[i+1:]
	c.expect("=")
	if s, ok := c.str(); ok {
		a.Kind, a.Value = "str", s
	} else {
		v := c.word("value (integer, string or id)").Val
		a.Kind, a.Value = "id", v
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			a.Kind, a.Value = "int", strconv.FormatInt(n, 10) // 07 ≡ 7
		}
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

// isPattern: "pattern" starts a declaration unless it is an actor or believer named pattern.
func (c *cur) isPattern() bool {
	n := c.peekN(1)
	return c.is("pattern") && n != nil && n.Kind == tWord && !verbs[n.Val] && n.Val != "believes" && n.Val != "doubts"
}

// pattern <id> [at <scene>]
func (c *cur) patternDecl() Node {
	n := &PatternDecl{Pos: c.peek().Pos}
	c.k++
	n.ID = c.ident("pattern id")
	if c.accept("at") {
		at := c.ident("scene")
		n.At = &at
	}
	c.end()
	return n
}
