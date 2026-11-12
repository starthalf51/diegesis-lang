package syntax

import (
	"fmt"
	"sort"
)

// ParseFile parses one .dieg.md file (or a .dieg file, which is all language)
// into an AST. It never stops at the first error: every line that fails is
// reported and parsing resumes at the next line.
func ParseFile(path string, src []byte) (*File, []Diag) {
	p := &parser{path: path, seen: map[string]bool{}}
	pr := split(path, src, p.report)
	p.f = &File{Pos: Pos{1, 1}, Path: path, Prose: pr.prose}
	var code []line
	for _, l := range pr.code {
		if ln, ok := lexLine(l, p.report); ok {
			code = append(code, ln)
		}
	}
	if pr.fmStart > 0 {
		code = p.frontmatter(pr, code)
	}
	p.lines = code
	p.f.Items = p.body(0, p.top)
	p.checkSugarDups()
	if p.f.Module == nil {
		p.report(Diag{Line: 1, Col: 1, Rule: "syntax", Msg: "missing module declaration"})
	}
	for i := range p.diags {
		p.diags[i].File, p.diags[i].Severity = path, "error"
	}
	sort.SliceStable(p.diags, func(i, j int) bool {
		a, b := p.diags[i], p.diags[j]
		return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
	})
	return p.f, p.diags
}

type parser struct {
	path  string
	f     *File
	lines []line
	i     int
	diags []Diag
	seen  map[string]bool // namespace:id for dup-decl
	sugar sugar
}

type bail struct{}

func (p *parser) report(d Diag) { p.diags = append(p.diags, d) }

func (p *parser) errAt(pos Pos, rule, format string, args ...any) {
	p.report(Diag{Line: pos.Line, Col: pos.Col, Rule: rule, Msg: fmt.Sprintf(format, args...)})
}

// declare records an id in a namespace and reports dup-decl on repeats.
func (p *parser) declare(ns string, id Ident) {
	if p.seen[ns+":"+id.Name] {
		p.errAt(id.Pos, "dup-decl", "%s %q is declared twice", ns, id.Name)
	}
	p.seen[ns+":"+id.Name] = true
}

// body parses the lines at indentation ind, stopping at a shallower line.
// Each line goes through fn; a failed line skips its deeper children.
func (p *parser) body(ind int, fn func(*cur) Node) []Node {
	var out []Node
	for p.i < len(p.lines) {
		l := p.lines[p.i]
		switch {
		case l.Bad:
			p.i++
			p.skipDeeper(l.Indent)
			continue
		case l.Indent < ind:
			return out
		case l.Indent > ind:
			p.errAt(l.Toks[0].Pos, "indent", "unexpected indentation (%d spaces, expected %d)", l.Indent, ind)
			p.skipDeeper(ind)
			continue
		}
		p.i++
		if n := p.line(l, fn); n != nil {
			out = append(out, n)
		} else {
			p.skipDeeper(ind)
		}
	}
	return out
}

func (p *parser) skipDeeper(ind int) {
	for p.i < len(p.lines) && p.lines[p.i].Indent > ind {
		p.i++
	}
}

func (p *parser) line(l line, fn func(*cur) Node) (n Node) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bail); !ok {
				panic(r)
			}
			n = nil
		}
	}()
	return fn(&cur{p: p, l: l})
}

// cur is a token cursor over one line.
type cur struct {
	p *parser
	l line
	k int
}

// children parses the indented block under the current line.
func (c *cur) children(fn func(*cur) Node) []Node { return c.p.body(c.l.Indent+2, fn) }

func (c *cur) peek() *tok { return c.peekN(0) }

func (c *cur) peekN(n int) *tok {
	if c.k+n < len(c.l.Toks) {
		return &c.l.Toks[c.k+n]
	}
	return nil
}

func (c *cur) here() Pos {
	if t := c.peek(); t != nil {
		return t.Pos
	}
	last := c.l.Toks[len(c.l.Toks)-1] // end of line: point at the last character of the line
	return Pos{last.Line, max(last.Col, last.End-1)}
}

func (c *cur) fail(format string, args ...any) {
	c.p.errAt(c.here(), "syntax", format, args...)
	panic(bail{})
}

func describe(t *tok) string {
	if t == nil {
		return "end of line"
	}
	if t.Kind == tString {
		return "string"
	}
	return fmt.Sprintf("%q", t.Val)
}

// is reports whether the next token is the word or punctuation v.
func (c *cur) is(v string) bool {
	t := c.peek()
	return t != nil && (t.Kind == tWord || t.Kind == tPunct) && t.Val == v
}

func (c *cur) accept(v string) bool {
	if c.is(v) {
		c.k++
		return true
	}
	return false
}

func (c *cur) expect(v string) {
	if !c.accept(v) {
		c.fail("expected %q, found %s", v, describe(c.peek()))
	}
}

func (c *cur) word(what string) tok {
	t := c.peek()
	if t == nil || t.Kind != tWord {
		c.fail("expected %s, found %s", what, describe(t))
	}
	c.k++
	return *t
}

func (c *cur) ident(what string) Ident {
	t := c.word(what)
	return Ident{t.Pos, t.Val}
}

func (c *cur) oneOf(what string, vals ...string) string {
	if t := c.peek(); t != nil && t.Kind == tWord {
		for _, v := range vals {
			if t.Val == v {
				c.k++
				return v
			}
		}
	}
	c.fail("expected %s %v, found %s", what, vals, describe(c.peek()))
	return ""
}

func (c *cur) str() (string, bool) {
	if t := c.peek(); t != nil && t.Kind == tString {
		c.k++
		return t.Val, true
	}
	return "", false
}

func (c *cur) end() {
	if t := c.peek(); t != nil {
		c.fail("unexpected %s", describe(t))
	}
}

var declKinds = map[string]bool{"character": true, "group": true, "place": true, "thing": true,
	"faction": true, "role": true, "fact": true, "artifact": true, "thread": true, "term": true}

var topOnly = map[string]bool{"module": true, "import": true, "work": true, "scene": true,
	"event": true, "account": true, "distance": true}

// top parses one top-level line.
func (p *parser) top(c *cur) Node {
	t := c.peek()
	if t.Kind == tPragma {
		return c.pragma()
	}
	f := p.f
	if c.isPattern() {
		return c.patternDecl()
	}
	if c.isAttribute() {
		return c.attribute()
	}
	if t.Kind == tWord {
		switch t.Val {
		case "module":
			c.k++
			id := c.ident("module name")
			c.end()
			if f.Module != nil {
				p.errAt(t.Pos, "dup-decl", "module is declared twice")
			} else if len(f.Items) > 0 || len(f.Imports) > 0 {
				p.errAt(t.Pos, "syntax", "module must be the first statement")
			} else {
				f.Module = &Module{t.Pos, id.Name}
			}
			return nil
		case "import":
			c.k++
			id := c.ident("module name")
			c.end()
			if len(f.Items) > 0 {
				p.errAt(t.Pos, "syntax", "import must come before declarations")
				return nil
			}
			p.declare("import", id)
			f.Imports = append(f.Imports, &Import{t.Pos, id.Name})
			return nil
		case "work":
			return c.work()
		case "scene":
			return c.scene()
		case "event":
			return c.event()
		case "account":
			return c.account()
		case "setup":
			return c.setup()
		case "distance":
			return c.distance()
		case "parent":
			return c.parent()
		case "born", "dies":
			return c.life()
		}
		if declKinds[t.Val] {
			return c.decl()
		}
		if next := c.peekN(1); next != nil && next.Kind == tWord && (next.Val == "believes" || next.Val == "doubts") {
			return c.belief() // background knowledge: believed from the start
		}
	}
	c.fail("unknown keyword %s at top level", describe(t))
	return nil
}

// pragma: #! allow ruleid : "reason"
func (c *cur) pragma() Node {
	pos := c.peek().Pos
	c.k++
	c.expect("allow")
	rule := c.word("rule id")
	c.expect(":")
	reason, ok := c.str()
	if !ok {
		c.fail("expected reason string, found %s", describe(c.peek()))
	}
	c.end()
	return &Pragma{pos, rule.Val, reason}
}
