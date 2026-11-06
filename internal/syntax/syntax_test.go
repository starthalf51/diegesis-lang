package syntax

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goldenDir = "../../testdata/golden"

// SL_UPDATE_GOLDEN=1 rewrites *.ast.json.
func TestGoldenParse(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(goldenDir, "*.sil.md"))
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
			want := strings.TrimSuffix(path, ".sil.md") + ".ast.json"
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
	files, _ := filepath.Glob(filepath.Join(goldenDir, "*.sil.md"))
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
	cases := []errCase{
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
	runCases(t, cases)
}

type errCase struct {
	name  string
	src   string
	want  []want
	check func(t *testing.T, f *File)
}

func runCases(t *testing.T, cases []errCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, diags := ParseFile("t.sil.md", []byte(c.src))
			var got []want
			for _, d := range diags {
				got = append(got, want{d.Line, d.Col, d.Rule})
				if d.File != "t.sil.md" || d.Severity != "error" || d.Msg == "" {
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
	_, diags := ParseFile("t.sil.md", []byte(src))
	if len(diags) != 3 {
		t.Fatalf("want 3 diags, got %d: %v", len(diags), diags)
	}
	for i, line := range []int{5, 6, 8} {
		if diags[i].Line != line {
			t.Errorf("diag %d line = %d, want %d", i, diags[i].Line, line)
		}
	}
}
