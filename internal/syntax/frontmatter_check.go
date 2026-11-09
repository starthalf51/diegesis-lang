package syntax

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// checkYAML reports anchors, aliases, merge keys (yaml-anchor) and repeated
// mapping keys (dup-decl, at the second key). Desugaring skips the repeats.
func (g *fm) checkYAML(n *yaml.Node) {
	if n.Anchor != "" {
		g.err(n, "yaml-anchor", "YAML anchors (&%s) are not allowed", n.Anchor)
	}
	if n.Kind == yaml.AliasNode {
		g.err(n, "yaml-anchor", "YAML aliases (*%s) are not allowed", n.Value)
		return
	}
	seen := map[string]bool{}
	for i, c := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 {
			if c.Value == "<<" && c.Tag == "!!merge" {
				g.err(c, "yaml-anchor", "YAML merge keys (<<) are not allowed")
			} else if seen[c.Value] {
				g.err(c, "dup-decl", "frontmatter key %q is given twice", c.Value)
			}
			seen[c.Value] = true
		}
		g.checkYAML(c)
	}
}

// single reports whether scalar n sits on one source line. Block scalars and
// multi-line plain/quoted scalars are rejected so that positions stay exact.
func (g *fm) single(n *yaml.Node) bool {
	ok := true
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		ok = false
	} else if i := n.Line - 1; i >= 0 && i < len(g.src) {
		rs := []rune(g.src[i])
		rest := ""
		if n.Column-1 <= len(rs) {
			rest = string(rs[n.Column-1:])
		}
		switch {
		case n.Style&yaml.DoubleQuotedStyle != 0:
			// ponytail: values with escapes are only checked for a raw newline
			if strings.ContainsAny(n.Value, "\"\\\n\t") {
				ok = !strings.Contains(n.Value, "\n")
			} else {
				ok = strings.HasPrefix(rest, `"`+n.Value+`"`)
			}
		case n.Style&yaml.SingleQuotedStyle != 0:
			ok = strings.HasPrefix(rest, "'"+strings.ReplaceAll(n.Value, "'", "''")+"'")
		default:
			ok = strings.HasPrefix(rest, n.Value)
		}
	}
	if !ok {
		g.err(n, "syntax", "frontmatter values must be single-line")
	}
	return ok
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
			if n.pos().Line <= s.fmEnd {
				continue // synthesized from the frontmatter itself
			}
			switch x := n.(type) {
			case *Beat:
				walk(x.Body)
			case *When:
				if s.hasTime && x.Op != "lasts" { // lasts is a length, not a time of day
					p.errAt(x.Pos, "dup-decl", "the scene's time is already given in frontmatter (when/date/time)")
				}
			case *Discourse:
				if s.hasDiscourse {
					p.errAt(x.Pos, "dup-decl", "discourse is already given in frontmatter")
				}
			case *Presence:
				if x.All && s.anyChars {
					p.errAt(x.Pos, "dup-decl", "exit all is implied by frontmatter characters")
				}
				for _, id := range x.IDs {
					if x.Op == "enter" && s.chars[id.Name] || x.Op == "mention" && s.mentions[id.Name] {
						p.errAt(id.Pos, "dup-decl", "%s %q is already in frontmatter", x.Op, id.Name)
					}
				}
			case *Render:
				for _, e := range x.Entries {
					if s.rkeys[e.Key] {
						p.errAt(e.Pos, "dup-decl", "render %s is already in frontmatter", e.Key)
					}
				}
			}
		}
	}
	for _, it := range p.f.Items {
		if sc, ok := it.(*Scene); ok {
			walk(sc.Body) // the sugar scene header itself is at a frontmatter line
		}
	}
}
