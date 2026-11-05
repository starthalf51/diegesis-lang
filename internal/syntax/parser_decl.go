package syntax

import "strings"

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
	for _, w := range strings.Fields("when enter exit mention beat render reveal withhold setup payoff discourse " +
		"parent born dies all not to with because src major minor extra") {
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
