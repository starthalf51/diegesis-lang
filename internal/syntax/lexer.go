package syntax

import (
	"strings"
	"unicode"
)

type tokKind int

const (
	tWord   tokKind = iota // identifiers, numbers, ids, durations, dates, refs
	tString                // "…" (Val is unescaped)
	tPunct                 // : , ( ) = ..
	tPragma                // #!
)

type tok struct {
	Kind tokKind
	Val  string
	Pos
	End int // column just after the token (strings include their quotes)
}

// line is one logical source line: indentation level and tokens.
type line struct {
	No     int
	Indent int // spaces
	Toks   []tok
	Bad    bool // lexing failed (already reported); parser skips it and its children
}

func wordStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '@'
}

func wordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_' || r == '?'
}

// lexLine tokenizes one code line. It returns ok=false for blank and
// comment-only lines. Errors are reported through diag; a line with a lexing
// error yields no tokens (the parser skips it).
func lexLine(l srcLine, diag func(Diag)) (line, bool) {
	rs := []rune(l.Text)
	ind := 0
	for ind < len(rs) && (rs[ind] == ' ' || rs[ind] == '\t') {
		ind++
	}
	if ind == len(rs) || rs[ind] == '#' && (ind+1 == len(rs) || rs[ind+1] != '!') {
		return line{}, false
	}
	if strings.ContainsRune(string(rs[:ind]), '\t') {
		diag(Diag{Line: l.No, Col: 1, Rule: "indent", Msg: "tab in indentation; use 2 spaces"})
		return line{No: l.No, Indent: ind, Bad: true}, true
	}
	toks, ok := lexRunes(rs, ind, l.No, 1, diag)
	if !ok {
		return line{No: l.No, Indent: ind, Bad: true}, true
	}
	return line{No: l.No, Indent: ind, Toks: toks}, true
}

// lexValue tokenizes a frontmatter scalar whose first rune sits at line:col.
func lexValue(s string, ln, col int, diag func(Diag)) ([]tok, bool) {
	return lexRunes([]rune(s), 0, ln, col, diag)
}

// lexRunes scans rs from index i; col0 is the column of rs[0].
func lexRunes(rs []rune, i, ln, col0 int, diag func(Diag)) ([]tok, bool) {
	var toks []tok
	fail := func(at int, msg string) ([]tok, bool) {
		diag(Diag{Line: ln, Col: col0 + at, Rule: "syntax", Msg: msg})
		return nil, false
	}
	for i < len(rs) {
		r := rs[i]
		p := Pos{ln, col0 + i}
		switch {
		case r == ' ' || r == '\t':
			i++
		case r == '#':
			if i+1 < len(rs) && rs[i+1] == '!' {
				toks = append(toks, tok{tPragma, "#!", p, p.Col + 2})
				i += 2
				continue
			}
			return toks, true
		case r == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(rs) && rs[j] != '"'; j++ {
				if rs[j] == '\\' {
					if j+1 < len(rs) && (rs[j+1] == '"' || rs[j+1] == '\\') {
						j++
					} else {
						return fail(j, `bad escape in string (only \" and \\)`)
					}
				}
				b.WriteRune(rs[j])
			}
			if j == len(rs) {
				return fail(i, "unterminated string")
			}
			toks = append(toks, tok{tString, b.String(), p, col0 + j + 1})
			i = j + 1
		case r == '.' && i+1 < len(rs) && rs[i+1] == '.':
			toks = append(toks, tok{tPunct, "..", p, p.Col + 2})
			i += 2
		case strings.ContainsRune(":,()=", r):
			toks = append(toks, tok{tPunct, string(r), p, p.Col + 1})
			i++
		case wordStart(r):
			j := i + 1
			for j < len(rs) {
				if wordChar(rs[j]) {
					j++
				} else if strings.ContainsRune(".-/", rs[j]) && j+1 < len(rs) && wordChar(rs[j+1]) {
					j += 2
				} else {
					break
				}
			}
			toks = append(toks, tok{tWord, string(rs[i:j]), p, col0 + j})
			i = j
		default:
			return fail(i, "unexpected character "+string(r))
		}
	}
	return toks, true
}
