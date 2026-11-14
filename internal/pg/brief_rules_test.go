package pg_test

import (
	"slices"
	"strings"
	"testing"
)

// brief-term-unshown, brief-card-missing, brief-place-unseen report data gaps only in a work that uses the card
// conventions; the same work without any convention attribute reports none of them.
func TestBriefGapRules(t *testing.T) {
	pool := open(t)
	body := "term t1 \"T1\" = \"g1\"\nterm t2 \"T2\" = \"g2\"\nplace room1 \"R1\"\nplace room2 \"R2\"\n" +
		"ATTRS" +
		"scene 1 at room1\n  enter a, b\n  render\n    explain t1, t2\nscene 2 at room2\n  enter a\n"
	attrs := "attr a.role = \"문지기\"\nattr room1.look = \"낡은 문\"\nattr t1.shown_by = p\n"
	rules := []string{"brief-card-missing", "brief-place-unseen", "brief-term-unshown"}
	id := compileLoad(t, pool, strings.Replace(body, "ATTRS", attrs, 1))
	ds := lint(t, pool, id)
	want := map[string][]string{
		"brief-card-missing": {"t.s/1/info"},
		"brief-place-unseen": {"t.s/2/info"},
		"brief-term-unshown": {"t.s/1/info"},
	}
	for _, r := range rules {
		if got := of(ds, r, id); !slices.Equal(got, want[r]) {
			t.Errorf("with conventions: %s got %v, want %v", r, got, want[r])
		}
	}
	for _, d := range ds {
		if d.Rule == "brief-card-missing" && !strings.Contains(d.Message, "B") {
			t.Errorf("card-missing names the wrong character: %s", d.Message)
		}
		if d.Rule == "brief-term-unshown" && !strings.Contains(d.Message, "T2") {
			t.Errorf("term-unshown names the wrong term: %s", d.Message)
		}
	}
	id = compileLoad(t, pool, strings.Replace(body, "ATTRS", "", 1))
	ds = lint(t, pool, id)
	for _, r := range rules {
		if got := of(ds, r, id); len(got) != 0 {
			t.Errorf("without conventions: %s got %v, want none", r, got)
		}
	}
}
