package syntax

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goldenDir = "../../testdata/golden"

// SL_UPDATE_GOLDEN=1 rewrites *.ast.json.
func TestGoldenParse(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(goldenDir, "*.sl.md"))
	if len(files) < 3 {
		t.Fatalf("want ≥3 golden inputs, got %v", files)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, diags := ParseFile(filepath.Base(path), src)
			for _, d := range diags {
				t.Errorf("unexpected diag: %s", d)
			}
			got := Dump(f, true)
			want := strings.TrimSuffix(path, ".sl.md") + ".ast.json"
			if os.Getenv("SL_UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(want, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			exp, err := os.ReadFile(want)
			if err != nil {
				t.Fatalf("%v (run with SL_UPDATE_GOLDEN=1)", err)
			}
			if !bytes.Equal(bytes.ReplaceAll(exp, []byte("\r\n"), []byte("\n")), got) {
				t.Errorf("AST differs from %s:\n%s", want, got)
			}
		})
	}
}

func TestSugarEquivalence(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(goldenDir, "*.sl.md"))
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			sugar := parseNoDiag(t, path)
			plain := parseNoDiag(t, filepath.Join(goldenDir, "desugared", filepath.Base(path)))
			sugar.Status, sugar.Mode, sugar.Rendered = "", "", nil // metadata, not language
			sugar.Path, plain.Path = "", ""
			a, b := Dump(sugar, false), Dump(plain, false)
			if !bytes.Equal(a, b) {
				t.Errorf("sugar and desugared ASTs differ\nsugar:\n%s\nplain:\n%s", a, b)
			}
		})
	}
}

func parseNoDiag(t *testing.T, path string) *File {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, diags := ParseFile(path, src)
	for _, d := range diags {
		t.Errorf("%s", d)
	}
	return f
}

type want struct {
	Line, Col int
	Rule      string
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		want  []want
		check func(t *testing.T, f *File)
	}{
		{name: "bad indent", src: "```sl\nmodule m\nscene 1 at x\n   enter a\n```\n",
			want: []want{{4, 4, "indent"}}},
		{name: "tab indent", src: "```sl\nmodule m\nscene 1 at x\n\tenter a\n```\n",
			want: []want{{4, 1, "indent"}}},
		{name: "unknown keyword", src: "```sl\nmodule m\nscene 1 at x\n  frobnicate a\n```\n",
			want: []want{{4, 3, "syntax"}}},
		{name: "unknown top keyword", src: "```sl\nmodule m\nchapter one\n```\n",
			want: []want{{3, 1, "syntax"}}},
		{name: "dup-decl enter vs characters",
			src:  "---\nmodule: m\nscene: \"1\"\nlocation: x\ncharacters: [a, b]\n---\n```sl\nenter b\n```\n",
			want: []want{{8, 7, "dup-decl"}}},
		{name: "dup-decl exit all vs characters",
			src:  "---\nmodule: m\nscene: \"1\"\ncharacters: [a]\n---\n```sl\nexit all\n```\n",
			want: []want{{7, 1, "dup-decl"}}},
		{name: "dup-decl scene header vs scene key",
			src:  "---\nmodule: m\nscene: \"1\"\nlocation: x\n---\n```sl\nscene 1 at x\n  enter a\n```\n",
			want: []want{{7, 1, "dup-decl"}}},
		{name: "dup-decl pov",
			src:  "---\nmodule: m\nscene: \"1\"\npov: a\n---\n```sl\nrender\n  pov b\n```\n",
			want: []want{{8, 3, "dup-decl"}}},
		{name: "dup-decl module",
			src:  "---\nmodule: m\n---\n```sl\nmodule m\n```\n",
			want: []want{{5, 1, "dup-decl"}}},
		{name: "dup-decl same id twice",
			src:  "```sl\nmodule m\nplace heath\nplace heath \"a heath\"\n```\n",
			want: []want{{4, 7, "dup-decl"}}},
		{name: "scene 1.10 stays a string",
			src:  "---\nmodule: m\nscene: 1.10\n---\n",
			want: nil,
			check: func(t *testing.T, f *File) {
				if s, ok := f.Items[0].(*Scene); !ok || s.ID.Name != "1.10" {
					t.Errorf("scene id = %#v, want 1.10", f.Items[0])
				}
			}},
		{name: "no is not false", src: "---\nmodule: m\nplaces: {no: Norway}\n---\n",
			check: func(t *testing.T, f *File) {
				if d, ok := f.Items[0].(*Decl); !ok || d.ID.Name != "no" || d.Names[0] != "Norway" {
					t.Errorf("decl = %#v", f.Items[0])
				}
			}},
		{name: "yaml anchor", src: "---\nmodule: m\nscene: \"1\"\ncharacters: &c [a]\n---\n",
			want: []want{{4, 13, "yaml-anchor"}}},
		{name: "yaml alias", src: "---\nmodule: m\nscene: \"1\"\ncharacters: &c [a]\nmentions: *c\n---\n",
			want: []want{{4, 13, "yaml-anchor"}, {5, 11, "yaml-anchor"}}},
		{name: "yaml merge key", src: "---\nmodule: m\nplaces:\n  heath:\n    <<: {name: X}\n---\n",
			want: []want{{5, 5, "yaml-anchor"}}},
		{name: "unknown key", src: "---\nmodule: m\ncolour: red\n---\n",
			want: []want{{3, 1, "unknown-key"}}},
		{name: "unterminated fence", src: "```sl\nmodule m\n",
			want: []want{{1, 1, "unterminated-fence"}}},
		{name: "unterminated frontmatter", src: "---\nmodule: m\n",
			want: []want{{1, 1, "unterminated-fence"}}},
		{name: "bad duration", src: "```sl\nmodule m\nscene 2 at x\n  when after 1 by 0d..1w\n```\n",
			want: []want{{4, 23, "syntax"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, diags := ParseFile("t.sl.md", []byte(c.src))
			var got []want
			for _, d := range diags {
				got = append(got, want{d.Line, d.Col, d.Rule})
				if d.File != "t.sl.md" || d.Severity != "error" || d.Msg == "" {
					t.Errorf("incomplete diag %+v", d)
				}
			}
			if len(got) != len(c.want) {
				t.Fatalf("diags = %v, want %v (%v)", got, c.want, diags)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("diag %d = %v, want %v (%s)", i, got[i], c.want[i], diags[i].Msg)
				}
			}
			if c.check != nil {
				c.check(t, f)
			}
		})
	}
}

func TestRecoverManyErrors(t *testing.T) {
	src := "```sl\nmodule m\nscene 1 at x\n  enter a\n  frobnicate b\n  a tells b: \"unterminated\n" +
		"  exit all\n      enter c\n  exit a\nplace heath\n```\n"
	_, diags := ParseFile("t.sl.md", []byte(src))
	if len(diags) != 3 {
		t.Fatalf("want 3 diags, got %d: %v", len(diags), diags)
	}
	for i, line := range []int{5, 6, 8} {
		if diags[i].Line != line {
			t.Errorf("diag %d line = %d, want %d", i, diags[i].Line, line)
		}
	}
}

// Every production in spec/grammar.ebnf, including the language additions.
const coverage = `module macbeth.cast
import macbeth.common

work macbeth "The Tragedy of Macbeth"
  unit day
  start 1040-08
  name ko "맥베스의 비극"

character macbeth "Macbeth" major
  name ko "맥베스"
character cawdor "the elder Thane of Cawdor" minor   # 무대에 한 번도 안 나온다
group witches "the Weird Sisters" major
  members witch1, witch2, witch3
  knows oracle
character athena "Athena"
  knows omniscient
place heath "a heath"
distance camp heath 0d..1d
role thane_of_cawdor "Thane of Cawdor", "Cawdor"
  held cawdor until @1.2.decree
  held macbeth from @1.2.decree
role chancellor "승상"
  shared
  held cao_cao from start until @chibi.end
fact cawdor_traitor "Cawdor aided Norway against Duncan" true
fact king_hereafter = holds(king, macbeth)
artifact macbeth_letter "Macbeth's letter"
  contains holds(thane_of_cawdor, macbeth), king_hereafter
  "They met me in the day of success"
thread scotland "Scotland"
parent adam of seth
parent noah of shem, ham, japheth
born seth when after @adam.born by 130y..130y
dies methuselah
#! allow time-cycle: "5:32는 첫아들 기준으로 읽는 것이 일반적"
born shem when after @noah.born by 500y..500y
setup kings promise: "Banquo's line"

event macbeth_writes "Macbeth writes to his wife"
  when after 1.3.ross_news
  when before 1.5
  macbeth writes macbeth_letter

scene 45.1 at camp in wei
  when at 208-11 dawn
  when during 44.9
  when before 46.1 by 1h..12h
  discourse after 44.9
  enter 曹操, 蔣幹
  mention zhou_yu
  role:chancellor tells 蔣幹: commanders_traitors src rtk.45.12
  曹操 does execute cai_mao, zhang_yun because commanders_traitors
  蔣幹 lies 曹操: not loyal(cai_mao) "they plot against you" src "45:3"
  odysseus whispers eurycleia: beggar_is_odysseus
  eurycleia sees odysseus: beggar_is_odysseus
  曹操 doubts letter_genuine because @45.1.theft
  reveal letter_forged at @46.2
  withhold letter_forged until 46.2
  payoff macbeth.act1/banquo_kings
  exit 蔣幹
  exit all
  render
    pov 曹操 close
    show "the burning ships"
    omit letter.full_text

account mark of peter_denies by mark
  when during 14.66
  maid asks peter: knows(peter, jesus)
`

func TestParseCoverage(t *testing.T) {
	f, diags := ParseFile("cast.sl", []byte(coverage))
	for _, d := range diags {
		t.Errorf("%s", d)
	}
	if f.Module == nil || f.Module.Name != "macbeth.cast" || len(f.Imports) != 1 {
		t.Fatalf("header = %+v %+v", f.Module, f.Imports)
	}
	kinds := map[string]int{}
	for _, it := range f.Items {
		kinds[strings.TrimPrefix(fmt.Sprintf("%T", it), "*syntax.")]++
	}
	for k, n := range map[string]int{"Work": 1, "Decl": 11, "Distance": 1, "Parent": 2, "Life": 3,
		"Pragma": 1, "Setup": 1, "Event": 1, "Scene": 1, "Account": 1} {
		if kinds[k] != n {
			t.Errorf("%s count = %d, want %d (all %v)", k, kinds[k], n, kinds)
		}
	}
	var sc *Scene
	for _, it := range f.Items {
		if s, ok := it.(*Scene); ok {
			sc = s
		}
	}
	act := sc.Body[6].(*Act)
	if !act.Role || act.Actor.Name != "chancellor" || act.Src != "rtk.45.12" {
		t.Errorf("role act = %+v", act)
	}
	does := sc.Body[7].(*Act)
	if does.Action != "execute" || len(does.Objects) != 2 || does.Because.Name != "commanders_traitors" {
		t.Errorf("does act = %+v", does)
	}
}
