package syntax

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// sugar remembers what the frontmatter declared so that the same fact
// written again in a block is a dup-decl (checkSugarDups).
type sugar struct {
	fmEnd    int // last frontmatter line; nodes at or above it came from sugar
	chars    map[string]bool
	anyChars bool
	mentions map[string]bool
	rkeys    map[string]bool
}

var declMaps = map[string]string{"characters": "character", "groups": "group", "places": "place",
	"things": "thing", "factions": "faction", "facts": "fact"}

var yamlLine = regexp.MustCompile(`line (\d+)`)

// fm turns frontmatter into synthetic language lines (positioned at the
// frontmatter) and wraps the block lines, per the table in spec/grammar.ebnf.
type fm struct {
	p   *parser
	off int // yaml line + off = file line
}

func (g *fm) pos(n *yaml.Node) Pos { return Pos{n.Line + g.off, n.Column} }

func (g *fm) err(n *yaml.Node, rule, format string, args ...any) {
	g.p.errAt(g.pos(n), rule, format, args...)
}

func kw(v string, p Pos) tok { return tok{tWord, v, p} }

// toks lexes a scalar, or a sequence of scalars joined by commas.
func (g *fm) toks(n *yaml.Node) ([]tok, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		p := g.pos(n)
		if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
			p.Col++
		}
		return lexValue(n.Value, p.Line, p.Col, g.p.report)
	case yaml.SequenceNode:
		var out []tok
		for i, it := range n.Content {
			if i > 0 {
				out = append(out, tok{tPunct, ",", g.pos(it)})
			}
			ts, ok := g.toks(it)
			if !ok {
				return nil, false
			}
			out = append(out, ts...)
		}
		return out, true
	case yaml.AliasNode:
		return nil, false // reported by checkAnchors
	}
	g.err(n, "syntax", "expected a value or a list")
	return nil, false
}

// ln builds a synthetic line; ok=false drops it (a part failed to lex).
func (g *fm) ln(out *[]line, ind int, head tok, n *yaml.Node, extra ...tok) {
	ts := append([]tok{head}, extra...)
	if n != nil {
		more, ok := g.toks(n)
		if !ok {
			return
		}
		ts = append(ts, more...)
	}
	*out = append(*out, line{No: head.Line, Indent: ind, Toks: ts})
}

func items(n *yaml.Node) []*yaml.Node {
	if n.Kind == yaml.SequenceNode {
		return n.Content
	}
	return []*yaml.Node{n}
}

func (p *parser) frontmatter(pr parts, code []line) []line {
	p.sugar = sugar{fmEnd: pr.fmEnd, chars: map[string]bool{}, mentions: map[string]bool{}, rkeys: map[string]bool{}}
	texts := make([]string, len(pr.fm))
	for i, l := range pr.fm {
		texts[i] = l.Text
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(texts, "\n")), &doc); err != nil {
		ln := 0
		if m := yamlLine.FindStringSubmatch(err.Error()); m != nil {
			ln, _ = strconv.Atoi(m[1])
		}
		p.errAt(Pos{ln + pr.fmStart, 1}, "syntax", "frontmatter: %v", err)
		return code
	}
	if len(doc.Content) == 0 {
		return code
	}
	g := &fm{p: p, off: pr.fmStart}
	root := doc.Content[0]
	g.checkAnchors(root)
	if root.Kind != yaml.MappingNode {
		g.err(root, "syntax", "frontmatter must be a mapping")
		return code
	}
	var head, decls, whens, dates, disc, enter, mention, rnd, tail []line
	var scene, loc, thread, date, tim, chars, pov, render *yaml.Node
	var needScene []*yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		k, v := root.Content[i], root.Content[i+1]
		switch k.Value {
		case "module":
			g.ln(&head, 0, kw("module", g.pos(k)), v)
		case "import":
			for _, it := range items(v) {
				g.ln(&head, 0, kw("import", g.pos(k)), it)
			}
		case "scene":
			scene = k
		case "location":
			loc, needScene = k, append(needScene, k)
		case "thread":
			thread, needScene = k, append(needScene, k)
		case "when":
			needScene = append(needScene, k)
			for _, it := range items(v) {
				g.ln(&whens, 2, kw("when", g.pos(k)), it)
			}
		case "date":
			date, needScene = k, append(needScene, k)
		case "time":
			tim, needScene = k, append(needScene, k)
		case "discourse":
			needScene = append(needScene, k)
			g.ln(&disc, 2, kw("discourse", g.pos(k)), v)
		case "characters":
			if v.Kind == yaml.MappingNode {
				g.decls(&decls, "character", v)
				continue
			}
			chars, needScene = k, append(needScene, k)
			g.ln(&enter, 2, kw("enter", g.pos(k)), v)
			p.sugar.anyChars = true
			for _, it := range items(v) {
				p.sugar.chars[it.Value] = true
			}
		case "mentions":
			needScene = append(needScene, k)
			g.ln(&mention, 2, kw("mention", g.pos(k)), v)
			for _, it := range items(v) {
				p.sugar.mentions[it.Value] = true
			}
		case "pov":
			pov, needScene = k, append(needScene, k)
		case "render":
			render, needScene = k, append(needScene, k)
		case "status":
			p.f.Status = v.Value
		case "mode":
			p.f.Mode = v.Value
		case "rendered":
			p.f.Rendered = map[string]string{}
			for j := 0; j+1 < len(v.Content); j += 2 {
				p.f.Rendered[v.Content[j].Value] = v.Content[j+1].Value
			}
		case "<<":
			// reported by checkAnchors
		default:
			if kind, ok := declMaps[k.Value]; ok {
				g.decls(&decls, kind, v)
				continue
			}
			g.err(k, "unknown-key", "unknown frontmatter key %q", k.Value)
		}
	}
	value := func(k *yaml.Node) *yaml.Node { // value node of key k
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i] == k {
				return root.Content[i+1]
			}
		}
		return nil
	}
	head = append(head, decls...)
	if scene == nil {
		for _, k := range needScene {
			g.err(k, "syntax", "frontmatter key %q requires scene", k.Value)
		}
		return append(head, code...)
	}
	var hdr []line
	g.ln(&hdr, 0, kw("scene", g.pos(scene)), value(scene))
	if len(hdr) == 1 {
		if loc != nil {
			more, ok := g.toks(value(loc))
			if ok {
				hdr[0].Toks = append(append(hdr[0].Toks, kw("at", g.pos(loc))), more...)
			}
		}
		if thread != nil {
			more, ok := g.toks(value(thread))
			if ok {
				hdr[0].Toks = append(append(hdr[0].Toks, kw("in", g.pos(thread))), more...)
			}
		}
	}
	if date != nil {
		var t []tok
		if tim != nil {
			t, _ = g.toks(value(tim))
		}
		g.ln(&dates, 2, kw("when", g.pos(date)), value(date), kw("at", g.pos(date)))
		if len(dates) == 1 {
			dates[0].Toks = append(dates[0].Toks, t...)
		}
	} else if tim != nil {
		g.err(tim, "syntax", "frontmatter key \"time\" requires date")
	}
	if pov != nil || render != nil {
		first := pov
		if first == nil {
			first = render
		}
		rnd = append(rnd, line{No: g.pos(first).Line, Indent: 2, Toks: []tok{kw("render", g.pos(first))}})
		if pov != nil {
			p.sugar.rkeys["pov"] = true
			g.ln(&rnd, 4, kw("pov", g.pos(pov)), value(pov))
		}
		if render != nil {
			rv := value(render)
			if rv.Kind != yaml.MappingNode {
				g.err(rv, "syntax", "render must be a mapping")
			}
			for j := 0; rv.Kind == yaml.MappingNode && j+1 < len(rv.Content); j += 2 {
				rk := rv.Content[j]
				if p.sugar.rkeys[rk.Value] {
					g.err(rk, "dup-decl", "render key %q is given twice", rk.Value)
					continue
				}
				p.sugar.rkeys[rk.Value] = true
				g.ln(&rnd, 4, kw(rk.Value, g.pos(rk)), rv.Content[j+1])
			}
		}
	}
	if chars != nil {
		tail = append(tail, line{No: g.pos(chars).Line, Indent: 2,
			Toks: []tok{kw("exit", g.pos(chars)), kw("all", g.pos(chars))}})
	}
	// The block becomes the scene body: shift it under the synthetic header.
	var body []line
	inHdr := false
	for _, l := range code {
		if l.Indent == 0 {
			inHdr = false
			if !l.Bad && l.Toks[0].Kind == tWord {
				switch v := l.Toks[0].Val; {
				case (v == "module" || v == "import") && len(body) == 0:
					head = append(head, l)
					continue
				case v == "scene":
					p.errAt(l.Toks[0].Pos, "dup-decl", "scene is already declared in frontmatter (line %d)", g.pos(scene).Line)
					inHdr = true
					continue
				}
			}
		}
		if !inHdr {
			l.Indent += 2
		}
		body = append(body, l)
	}
	out := append(head, hdr...)
	for _, part := range [][]line{whens, dates, disc, enter, mention, body, rnd, tail} {
		out = append(out, part...)
	}
	return out
}

// decls desugars a declaration map: id: "Name" | {name, tier, def, value, members, knows, names}.
func (g *fm) decls(out *[]line, kind string, m *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		g.err(m, "syntax", "%s declarations must be a mapping", kind)
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		idN, v := m.Content[i], m.Content[i+1]
		var names, tier, def, val []tok
		var attrs []line
		ok := true
		str := func(n *yaml.Node) tok {
			p := g.pos(n)
			if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
				p.Col++
			}
			return tok{tString, n.Value, p}
		}
		switch v.Kind {
		case yaml.ScalarNode:
			if v.Tag != "!!null" && v.Value != "" {
				names = []tok{str(v)}
			}
		case yaml.MappingNode:
			for j := 0; j+1 < len(v.Content); j += 2 {
				fk, fv := v.Content[j], v.Content[j+1]
				var good bool
				switch fk.Value {
				case "name":
					for n, it := range items(fv) {
						if n > 0 {
							names = append(names, tok{tPunct, ",", g.pos(it)})
						}
						names = append(names, str(it))
					}
					good = true
				case "tier":
					tier, good = g.toks(fv)
				case "value":
					val, good = g.toks(fv)
				case "def":
					def, good = g.toks(fv)
					def = append([]tok{{tPunct, "=", g.pos(fk)}}, def...)
				case "members", "knows":
					g.ln(&attrs, 2, kw(fk.Value, g.pos(fk)), fv)
					good = true
				case "names":
					for n := 0; n+1 < len(fv.Content); n += 2 {
						lk := fv.Content[n]
						attrs = append(attrs, line{No: g.pos(lk).Line, Indent: 2,
							Toks: []tok{kw("name", g.pos(lk)), kw(lk.Value, g.pos(lk)), str(fv.Content[n+1])}})
					}
					good = true
				case "<<":
					good = true
				default:
					g.err(fk, "unknown-key", "unknown %s field %q", kind, fk.Value)
					good = true
				}
				ok = ok && good
			}
		default:
			g.err(v, "syntax", "expected a name or a mapping for %s %q", kind, idN.Value)
			ok = false
		}
		id, idOK := g.toks(idN)
		if !ok || !idOK {
			continue
		}
		ts := append([]tok{kw(kind, g.pos(idN))}, id...)
		for _, part := range [][]tok{names, tier, def, val} {
			ts = append(ts, part...)
		}
		*out = append(*out, line{No: g.pos(idN).Line, Toks: ts})
		*out = append(*out, attrs...)
	}
}

func (g *fm) checkAnchors(n *yaml.Node) {
	if n.Anchor != "" {
		g.err(n, "yaml-anchor", "YAML anchors (&%s) are not allowed", n.Anchor)
	}
	if n.Kind == yaml.AliasNode {
		g.err(n, "yaml-anchor", "YAML aliases (*%s) are not allowed", n.Value)
		return
	}
	for i, c := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 && c.Value == "<<" && c.Tag == "!!merge" {
			g.err(c, "yaml-anchor", "YAML merge keys (<<) are not allowed")
		}
		g.checkAnchors(c)
	}
}

// checkSugarDups reports block statements that repeat frontmatter facts.
func (p *parser) checkSugarDups() {
	s := p.sugar
	if s.fmEnd == 0 {
		return
	}
	var walk func([]Node)
	walk = func(ns []Node) {
		for _, n := range ns {
			switch x := n.(type) {
			case *Scene:
				walk(x.Body)
			case *Beat:
				walk(x.Body)
			case *Presence:
				if x.Line <= s.fmEnd {
					continue
				}
				if x.All && s.anyChars {
					p.errAt(x.Pos, "dup-decl", "exit all is implied by frontmatter characters")
				}
				for _, id := range x.IDs {
					if x.Op == "enter" && s.chars[id.Name] || x.Op == "mention" && s.mentions[id.Name] {
						p.errAt(id.Pos, "dup-decl", "%s %q is already in frontmatter", x.Op, id.Name)
					}
				}
			case *Render:
				if x.Line <= s.fmEnd {
					continue
				}
				for _, e := range x.Entries {
					if s.rkeys[e.Key] {
						p.errAt(e.Pos, "dup-decl", "render %s is already in frontmatter", e.Key)
					}
				}
			}
		}
	}
	walk(p.f.Items)
}
