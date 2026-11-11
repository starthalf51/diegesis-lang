package write

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertIdea(t *testing.T) {
	text := "# Title\n# by X\n\nFirst para\nline two.\n\n\nSecond para.\n"
	got, _, err := ConvertIdea("T", text)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"¶1 First para\nline two.", "¶2 Second para.", "src <문단번호>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "by X") || strings.Contains(got, "¶3") {
		t.Errorf("header kept or extra paragraph:\n%s", got)
	}
	if _, _, err := ConvertIdea("T", strings.Repeat("가", 12001)); err == nil || !strings.Contains(err.Error(), "too long for one pass") {
		t.Errorf("want too-long error, got %v", err)
	}
}

func TestSrcCoverageAndChecker(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "ep01"), 0o755)
	os.WriteFile(filepath.Join(dir, "ep01", "s1.dieg.md"), []byte("a does x src 1\nb does y src 2\nc does z src 2\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cast.dieg.md"), []byte("work w \"T\"\n"), 0o644)
	if n := SrcCoverage(dir); n != 2 {
		t.Fatalf("SrcCoverage = %d, want 2", n)
	}
	none := func(context.Context, string) ([]Diag, error) { return nil, nil }
	if ds, _ := CoverageChecker(none, 10)(context.Background(), dir); len(ds) != 0 {
		t.Errorf("2 of 10 paragraphs (a fifth) should pass: %v", ds)
	}
	ds, _ := CoverageChecker(none, 11)(context.Background(), dir)
	if len(ds) != 1 || ds[0].Severity != "error" || ds[0].Rule != "coverage" {
		t.Errorf("2 of 11 paragraphs should be a coverage error: %v", ds)
	}
}

func TestConvertIdeaWhitespaceLines(t *testing.T) {
	_, n, err := ConvertIdea("T", "One.\n \t \nTwo.\n\t\nThree.\n")
	if err != nil || n != 3 {
		t.Errorf("paragraphs = %d, %v; want 3", n, err)
	}
}
