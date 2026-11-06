package syntax

import (
	"bytes"
	"regexp"
	"strings"
)

// srcLine is one raw source line with its 1-based file line number.
type srcLine struct {
	No   int
	Text string
}

// split separates a .dieg.md file into frontmatter, language-block lines and
// prose. A file ending in .dieg (not .dieg.md) is all language.
type parts struct {
	fm      []srcLine // YAML lines between the --- fences
	fmStart int       // line of the opening --- (0 = no frontmatter)
	fmEnd   int       // line of the closing ---
	code    []srcLine
	prose   []Node
}

var beatAnchor = regexp.MustCompile(`^\s*<!--\s*beat:\s*([^\s>]+)\s*-->\s*$`)

func split(path string, src []byte, diag func(Diag)) parts {
	src = bytes.TrimPrefix(src, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	raw := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	lines := make([]srcLine, len(raw))
	for i, t := range raw {
		lines[i] = srcLine{i + 1, t}
	}
	var p parts
	if strings.HasSuffix(path, ".dieg") {
		p.code = lines
		return p
	}
	i := 0
	if len(lines) > 0 && strings.TrimRight(lines[0].Text, " \t") == "---" {
		p.fmStart = 1
		for i = 1; i < len(lines); i++ {
			if t := strings.TrimRight(lines[i].Text, " \t"); t == "---" || t == "..." {
				break
			}
			p.fm = append(p.fm, lines[i])
		}
		if i == len(lines) {
			diag(Diag{Line: 1, Col: 1, Rule: "unterminated-fence", Msg: "frontmatter is not closed with ---"})
		}
		p.fmEnd = i + 1
		i++
	}
	var prose []srcLine
	flush := func() {
		for len(prose) > 0 && strings.TrimSpace(prose[0].Text) == "" {
			prose = prose[1:]
		}
		for len(prose) > 0 && strings.TrimSpace(prose[len(prose)-1].Text) == "" {
			prose = prose[:len(prose)-1]
		}
		if len(prose) > 0 {
			texts := make([]string, len(prose))
			for k, l := range prose {
				texts[k] = l.Text
			}
			p.prose = append(p.prose, &Prose{Pos{prose[0].No, 1}, strings.Join(texts, "\n")})
		}
		prose = nil
	}
	for ; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimRight(l.Text, " \t") == "```sl" {
			flush()
			open := l.No
			for i++; i < len(lines) && strings.TrimRight(lines[i].Text, " \t") != "```"; i++ {
				p.code = append(p.code, lines[i])
			}
			if i == len(lines) {
				diag(Diag{Line: open, Col: 1, Rule: "unterminated-fence", Msg: "```sl block is not closed with ```"})
			}
			continue
		}
		if m := beatAnchor.FindStringSubmatch(l.Text); m != nil {
			flush()
			p.prose = append(p.prose, &BeatAnchor{Pos{l.No, strings.Index(l.Text, "<") + 1}, m[1]})
			continue
		}
		prose = append(prose, l)
	}
	flush()
	return p
}

// IsSource 는 Diegesis 소스 파일(.dieg.md)인지 판정한다. 확장자 인식은 여기 한 곳뿐이다.
func IsSource(path string) bool { return strings.HasSuffix(path, ".dieg.md") }
