package write

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseFilesRoundTrip(t *testing.T) {
	in := map[string]string{"a.dieg.md": "# a\nline\n", "sub/b.md": "b\n", "c.yaml": "k: v\n"}
	dir := t.TempDir()
	if err := WriteFiles(dir, in); err != nil {
		t.Fatal(err)
	}
	text, err := PackFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "=== FILE: a.dieg.md ===\n") {
		t.Errorf("not sorted / wrong header: %q", text)
	}
	out, err := ParseFiles(text)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("got %v", out)
	}
}

func TestParseFilesRejects(t *testing.T) {
	h := func(p, body string) string { return "=== FILE: " + p + " ===\n" + body }
	cases := map[string]string{
		"no header":  "just text\n",
		"dotdot":     h("../x.dieg.md", "x\n"),
		"abs":        h("/abs.dieg.md", "x\n"),
		"drive":      h(`C:\x`, "x\n"),
		"empty":      h("a.dieg.md", "\n  \n"),
		"duplicate":  h("a.dieg.md", "x\n") + h("a.dieg.md", "y\n"),
		"extension":  h("a.txt", "x\n"),
		"inner dots": h("a/../../b.md", "x\n"),
	}
	for name, text := range cases {
		if _, err := ParseFiles(text); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestWriteFilesStaysInDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "out")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "stale.md"), []byte("old"), 0o644)
	if err := WriteFiles(dir, map[string]string{"x/y.dieg.md": "y\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.md")); err == nil {
		t.Error("dir not emptied")
	}
	if err := WriteFiles(dir, map[string]string{"../esc.md": "x\n"}); err == nil {
		t.Error("escaping path accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "esc.md")); err == nil {
		t.Error("file escaped dir")
	}
}
