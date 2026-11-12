package write

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var chapterRE = regexp.MustCompile(`^(CHAPTER \d+|Epilogue)`)

// paras returns n unique paragraphs of about size runes each, tagged id-k.
func paras(id string, n, size int) []string {
	var ps []string
	for k := 1; k <= n; k++ {
		tag := fmt.Sprintf("%s-%d ", id, k)
		ps = append(ps, tag+strings.Repeat("가", size-len([]rune(tag))))
	}
	return ps
}

func TestSplitBook(t *testing.T) {
	one, two, three := paras("one", 3, 200), paras("two", 40, 500), paras("three", 2, 100)
	text := "Front matter, dropped.\n\nCHAPTER 1. Loomings.\n\n" + strings.Join(one, "\n\n") +
		"\n\nCHAPTER 2. The Carpet-Bag.\n\n" + strings.Join(two, "\n\n") +
		"\n\nCHAPTER 3. The Spouter-Inn.\n\n" + strings.Join(three, "\n\n") + "\n"
	parts, err := SplitBook(text, chapterRE, 12000)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 4 {
		t.Fatalf("got %d parts, want 4: %+v", len(parts), parts)
	}
	want := [][2]int{{1, 0}, {2, 1}, {2, 2}, {3, 0}}
	all := map[string]bool{}
	for _, p := range append(append(one, two...), three...) {
		all[p] = true
	}
	for i, p := range parts {
		if p.Chapter != want[i][0] || p.Seq != want[i][1] {
			t.Errorf("part %d: chapter %d seq %d, want %v", i, p.Chapter, p.Seq, want[i])
		}
		if n := len([]rune(p.Text)); n > 12000 {
			t.Errorf("part %d: %d runes > 12000", i, n)
		}
		for _, q := range strings.Split(p.Text, "\n\n") {
			if !all[q] {
				t.Errorf("part %d: %.20q… is not a whole source paragraph", i, q)
			}
		}
	}
	if parts[1].Title != "CHAPTER 2. The Carpet-Bag." || strings.Contains(parts[0].Text, "Front matter") {
		t.Errorf("title %q / front matter kept: %.40q", parts[1].Title, parts[0].Text)
	}
	if got := strings.Count(parts[1].Text+"\n\n"+parts[2].Text, "two-"); got != 40 {
		t.Errorf("chapter 2 parts hold %d of 40 paragraphs", got)
	}

	ep, err := SplitBook("CHAPTER 1. A\n\nx\n\nEpilogue\n\ny\n", chapterRE, 12000)
	if err != nil || len(ep) != 2 || ep[1].Chapter != 0 || PartStem(ep[1]) != "epilogue" || PartStem(ep[0]) != "ch001" ||
		PartStem(Part{Chapter: 32, Seq: 2}) != "ch032b" {
		t.Errorf("epilogue / stems: %+v %v", ep, err)
	}
	in, err := ParseChapters("1-2,5,epilogue")
	if err != nil || !in(1) || !in(2) || in(3) || !in(5) || !in(0) {
		t.Errorf("ParseChapters: %v", err)
	}
	if _, err := ParseChapters("3-1"); err == nil {
		t.Error("ParseChapters accepted 3-1")
	}
}

func TestSplitBookNoMatch(t *testing.T) {
	_, err := SplitBook("no headings here\n\nat all\n", chapterRE, 12000)
	if err == nil || !strings.Contains(err.Error(), "no chapter headings match") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func partReply(stem, decl string) string {
	return "=== FILE: " + stem + ".dieg.md ===\n---\nmodule: moby." + stem + "\nimport: [moby.book]\n---\n\n```sl\n" + decl +
		"\nscene s1\n  ishmael does look src 1\n```\n" + End + "\n"
}

func TestConvertBookRefs(t *testing.T) {
	dir := t.TempDir()
	parts := []Part{{Chapter: 1, Title: "CHAPTER 1.", Text: "Call me Ishmael."}, {Chapter: 2, Title: "CHAPTER 2.", Text: "I stuffed a shirt."}}
	f := &fakeLLM{replies: []string{
		partReply("ch001", `character ishmael "Ishmael"`),
		partReply("ch002", `character ishmael "Ishmael"`), // redeclares: the checker loop must catch it
		partReply("ch002", `place inn "Spouter-Inn"`),
	}}
	none := func(context.Context, string) ([]Diag, error) { return nil, nil }
	unclean, err := ConvertBook(ctx, f, none, "moby", "Moby-Dick", dir, parts, testRefs(t), Opts{Rounds: 3})
	if err != nil || unclean != 0 {
		t.Fatalf("unclean=%d err=%v", unclean, err)
	}
	if len(f.prompts) != 3 {
		t.Fatalf("%d prompts, want 3", len(f.prompts))
	}
	if strings.Contains(f.prompts[0], "character ishmael") {
		t.Error("first part's prompt already lists ishmael")
	}
	for _, want := range []string{"character ishmael (moby.ch001)", "reuse these IDs, do not redeclare", "import: [moby.book, moby.ch001]"} {
		if !strings.Contains(f.prompts[1], want) {
			t.Errorf("second part's prompt lacks %q", want)
		}
	}
	if !strings.Contains(f.prompts[2], `"ishmael" is already declared in module moby.ch001`) {
		t.Error("redeclaration not fed back as a diagnostic")
	}
	for _, n := range []string{"book.dieg.md", "ch001.dieg.md", "ch002.dieg.md"} {
		if _, err := os.Stat(filepath.Join(dir, "book", n)); err != nil {
			t.Error(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(dir, "summary.tsv"))
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 3 ||
		lines[0] != "chapter\tseq\tclean\trounds\tseconds\ttokens" || !strings.HasPrefix(lines[2], "2\t0\ttrue\t2\t") {
		t.Errorf("summary.tsv:\n%s", b)
	}
	// a second run skips the parts already written
	f2 := &fakeLLM{replies: []string{"junk"}}
	if _, err := ConvertBook(ctx, f2, none, "moby", "Moby-Dick", dir, parts, testRefs(t), Opts{Rounds: 1}); err != nil || len(f2.prompts) != 0 {
		t.Errorf("resume called the model %d times, err %v", len(f2.prompts), err)
	}
}

func TestSplitBookHeadingNumbers(t *testing.T) {
	parts, err := SplitBook("CHAPTER 3. C\n\nx\n\nCHAPTER 7. G\n\ny\n\nEpilogue\n\nz\n", chapterRE, 12000)
	if err != nil || len(parts) != 3 || parts[0].Chapter != 3 || parts[1].Chapter != 7 || parts[2].Chapter != 0 {
		t.Fatalf("chapters should come from the heading numbers: %+v %v", parts, err)
	}
}

// A Gutenberg contents list ends with "Epilogue" and is followed by front matter: that front matter would become a
// second epilogue, so SplitBook refuses the text instead of dropping either one.
func TestSplitBookContentsList(t *testing.T) {
	text := "CONTENTS\n\nCHAPTER 1. Loomings.\n\nCHAPTER 2. The Carpet-Bag.\n\nCHAPTER 3. The Spouter-Inn.\n\nEpilogue\n\n" +
		"ETYMOLOGY.\n\nEXTRACTS.\n\nCHAPTER 1. Loomings.\n\nCall me Ishmael.\n\nCHAPTER 2. The Carpet-Bag.\n\nb\n\n" +
		"CHAPTER 3. The Spouter-Inn.\n\nc\n\nEpilogue\n\nThe drama's done.\n"
	_, err := SplitBook(text, chapterRE, 12000)
	if err == nil || !strings.Contains(err.Error(), `"epilogue" twice`) || !strings.Contains(err.Error(), "front matter") {
		t.Fatalf("want a duplicate-epilogue error, got %v", err)
	}
	// with the front matter cut (Task 2's preparation) the same text splits cleanly
	parts, err := SplitBook(text[strings.Index(text, "EXTRACTS.\n\n")+len("EXTRACTS.\n\n"):], chapterRE, 12000)
	if err != nil || len(parts) != 4 || parts[0].Chapter != 1 || parts[3].Text != "The drama's done." {
		t.Fatalf("%+v %v", parts, err)
	}
}

func TestPartCheckerModule(t *testing.T) {
	book, dir := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dir, "ch001.dieg.md"), []byte("---\nmodule: moby.chapter1\n---\n"), 0o644)
	none := func(context.Context, string) ([]Diag, error) { return nil, nil }
	ds, err := partChecker(none, book, "ch001.dieg.md", "moby.ch001", nil)(ctx, dir)
	if err != nil || len(ds) != 1 || ds[0].Severity != "error" || !strings.Contains(ds[0].Msg, "module: moby.ch001") {
		t.Fatalf("want a module mismatch error, got %v %v", ds, err)
	}
	os.WriteFile(filepath.Join(dir, "ch001.dieg.md"), []byte("---\nmodule: moby.ch001\n---\n"), 0o644)
	if ds, err := partChecker(none, book, "ch001.dieg.md", "moby.ch001", nil)(ctx, dir); err != nil || len(ds) != 0 {
		t.Fatalf("matching module: %v %v", ds, err)
	}
}
