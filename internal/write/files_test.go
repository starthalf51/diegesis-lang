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
		"case dup":   h("A.md", "x\n") + h("a.md", "y\n"),
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

func TestParseFilesSentinelAndFences(t *testing.T) {
	sl := "# a\n```sl\nenter x\n```\n"
	want := map[string]string{"a.dieg.md": sl, "b.md": "b\n"}
	cases := map[string]string{
		"chatter after END":                     "=== FILE: a.dieg.md ===\n" + sl + "=== FILE: b.md ===\nb\n=== END ===\nHope this helps!\n",
		"fenced with END":                       "```text\n=== FILE: a.dieg.md ===\n" + sl + "=== FILE: b.md ===\nb\n=== END ===\n```\n",
		"fenced, no END":                        "Here you go:\n```\n=== FILE: a.dieg.md ===\n" + sl + "=== FILE: b.md ===\nb\n```\n",
		"fenced last file has sl block, no END": "```markdown\n=== FILE: b.md ===\nb\n=== FILE: a.dieg.md ===\n" + sl + "```\n",
		"plain, no END":                         "=== FILE: a.dieg.md ===\n" + sl + "=== FILE: b.md ===\nb\n",
	}
	for name, text := range cases {
		got, err := ParseFiles(text)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %q err %v", name, got, err)
		}
	}
	// A last file that ends with its own closing fence keeps it.
	got, err := ParseFiles("=== FILE: a.dieg.md ===\n" + sl)
	if err != nil || got["a.dieg.md"] != sl {
		t.Errorf("own fence stripped: %q %v", got, err)
	}
}

func TestWriteFilesRefusesDangerousDirs(t *testing.T) {
	cwd, _ := os.Getwd()
	repo := t.TempDir()
	os.Mkdir(filepath.Join(repo, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo, "keep.md"), []byte("keep"), 0o644)
	for _, d := range []string{"", ".", "/", filepath.VolumeName(cwd) + string(filepath.Separator), cwd, filepath.Dir(cwd), repo} {
		if err := WriteFiles(d, map[string]string{"x.md": "x\n"}); err == nil {
			t.Errorf("%q: want refusal", d)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "keep.md")); err != nil {
		t.Error("repo dir was emptied")
	}
}

func TestResetRun(t *testing.T) {
	out := t.TempDir()
	for _, p := range []string{"novel/s1.md", "rounds/r9/x.md", "story/keep.md"} {
		os.MkdirAll(filepath.Dir(filepath.Join(out, p)), 0o755)
		os.WriteFile(filepath.Join(out, p), []byte("x"), 0o644)
	}
	if err := ResetRun(out); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]bool{"novel": false, "rounds": false, "story/keep.md": true} {
		if _, err := os.Stat(filepath.Join(out, p)); (err == nil) != want {
			t.Errorf("%s exists=%t want %t", p, err == nil, want)
		}
	}
	if err := ResetRun(""); err == nil {
		t.Error("empty out: want refusal")
	}
}

func TestParseFilesRejectsDeleteBundleAllowsIt(t *testing.T) {
	text := "=== FILE: a.dieg.md ===\nx\n=== DELETE: b.dieg.md ===\n" + End
	if _, err := ParseFiles(text); err == nil || !strings.Contains(err.Error(), "DELETE") {
		t.Errorf("ParseFiles accepted a DELETE bundle: %v", err)
	}
	files, dels, err := ParseBundle(text)
	if err != nil || len(files) != 1 || len(dels) != 1 || dels[0] != "b.dieg.md" {
		t.Errorf("%v %v %v", files, dels, err)
	}
	for _, bad := range []string{"=== DELETE: ../x ===\n", "=== DELETE: /x.md ===\n", "=== DELETE: x.txt ===\n",
		"=== FILE: a.md ===\nx\n=== DELETE: a.md ===\n"} {
		if _, _, err := ParseBundle(bad + End); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
