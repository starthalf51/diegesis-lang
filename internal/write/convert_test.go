package write

import (
	"strings"
	"testing"
)

func TestConvertIdea(t *testing.T) {
	text := "# Title\n# by X\n\nFirst para\nline two.\n\n\nSecond para.\n"
	got, err := ConvertIdea("T", text)
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
	if _, err := ConvertIdea("T", strings.Repeat("가", 12001)); err == nil || !strings.Contains(err.Error(), "too long for one pass") {
		t.Errorf("want too-long error, got %v", err)
	}
}
