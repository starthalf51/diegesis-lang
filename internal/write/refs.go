package write

import (
	"embed"
	"fmt"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

// refs/ holds byte-for-byte copies of repo files (TestRefsInSync keeps them in step):
// spec/grammar.ebnf, README.md, corpus/macbeth/cast.dieg.md, corpus/macbeth/act1/1.3.dieg.md.
//
//go:embed refs
var refsFS embed.FS

// Refs is the reference material every coding prompt carries.
type Refs struct {
	Grammar, Keywords string
	Examples          []string
}

// LoadRefs reads the embedded copies. Keywords is the README "## 키워드" section plus a one-line summary of
// each lint rule (from the rule headers), so the model knows what the checker will say.
func LoadRefs() (Refs, error) {
	read := func(n string) (string, error) {
		b, err := refsFS.ReadFile("refs/" + n)
		return strings.ReplaceAll(string(b), "\r\n", "\n"), err
	}
	var r Refs
	var err error
	if r.Grammar, err = read("grammar.ebnf"); err != nil {
		return r, err
	}
	readme, err := read("README.md")
	if err != nil {
		return r, err
	}
	i := strings.Index(readme, "\n## 키워드\n")
	if i < 0 {
		return r, fmt.Errorf("refs/README.md: no \"## 키워드\" section")
	}
	kw := readme[i+1:]
	if j := strings.Index(kw[1:], "\n## "); j >= 0 {
		kw = kw[:j+2]
	}
	rs, err := pg.Rules(rules.FS)
	if err != nil {
		return r, err
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(kw) + "\n\n## Lint rules (the checker's warnings)\n\n")
	for _, x := range rs {
		fmt.Fprintf(&sb, "- `%s` (%s): %s\n", x.ID, x.Severity, x.Summary)
	}
	r.Keywords = sb.String()
	for _, n := range []string{"cast.dieg.md", "1.3.dieg.md"} {
		s, err := read(n)
		if err != nil {
			return r, err
		}
		r.Examples = append(r.Examples, s)
	}
	return r, nil
}
