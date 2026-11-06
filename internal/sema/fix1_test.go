package sema

import (
	"slices"
	"testing"

	"diegesis/internal/ir"
)

func TestDiscourseNaturalOrder(t *testing.T) {
	w, diags := Build(parse(t, map[string]string{
		"cast.dieg":         cast,
		"book-10/a.dieg":    "module t.b10\nimport t.cast\nscene 10.1 at room\n",
		"book-2/a.dieg":     "module t.b2\nimport t.cast\nscene 1.10 at room\nscene 1.2 at room\n",
		"book-2/z-1.9.dieg": "module t.b2z\nimport t.cast\nscene 1.9 at room\n",
	}))
	noErrors(t, diags)
	ord := map[string]int{}
	for _, s := range w.Scenes {
		ord[s.ID] = s.DiscourseOrd
	}
	want := map[string]int{"t/t.b2/1.10": 1, "t/t.b2/1.2": 2, "t/t.b2z/1.9": 3, "t/t.b10/10.1": 4}
	for id, o := range want {
		if ord[id] != o {
			t.Errorf("discourse_ord %s = %d, want %d (all %v)", id, ord[id], o, ord)
		}
	}
	if !naturalLess("odyssey-2", "odyssey-13") || naturalLess("1.10", "1.2") == true || !naturalLess("1.2", "1.10") {
		t.Error("naturalLess wrong")
	}
}

func atEdge(t *testing.T, start, date string) (int64, int64) {
	t.Helper()
	w, diags := Build(parse(t, map[string]string{
		"c.dieg": "module t.c\nwork t \"T\"\n  start " + start + "\nplace room \"R\"\nscene 1 at room\n  when at " + date + "\n",
	}))
	noErrors(t, diags)
	for _, e := range w.TimeEdges {
		if e.From == "t/start" && e.To == "t/t.c/1.start" && e.Max != nil {
			return *e.Min, *e.Max
		}
	}
	t.Fatalf("no at edge in %+v", w.TimeEdges)
	return 0, 0
}

func TestAtDateNoLeap(t *testing.T) {
	const d = 86400
	cases := []struct {
		start, date string
		min, max    int64
	}{
		{"1", "1656", 1655 * 365 * d, 1656*365*d - 1},
		{"1000-01-01", "1000-02", 31 * d, 59*d - 1},
		{"1000-01-01", "1000-03-01", 59 * d, 60*d - 1},
		{"2000-03-01", "2004-03-01", 4 * 365 * d, 4*365*d + d - 1}, // no leap days
		{"1000-02-10", "1000-02", 0 - 9*d, 28*d - 9*d - 1},
	}
	for _, c := range cases {
		if lo, hi := atEdge(t, c.start, c.date); lo != c.min || hi != c.max {
			t.Errorf("start %s at %s = [%d,%d], want [%d,%d]", c.start, c.date, lo, hi, c.min, c.max)
		}
	}
}

func TestNoWork(t *testing.T) {
	_, diags := Build(parse(t, map[string]string{"a.dieg": "module t.a\nplace room \"R\"\n", "b.dieg": "module t.b\n"}))
	if len(diags) != 1 || diags[0].Rule != "no-work" || diags[0].Severity != "error" ||
		diags[0].File != "a.dieg" || diags[0].Line != 1 || diags[0].Col != 1 {
		t.Fatalf("diags = %v, want one no-work error at a.dieg:1:1", diags)
	}
}

func waivedEdges(w *ir.Work) []string {
	var out []string
	for _, e := range w.TimeEdges {
		if e.Waived {
			out = append(out, e.From+">"+e.To)
		}
	}
	return out
}

func TestWaiverEdgesOnlyOwnWhen(t *testing.T) {
	w, diags := build(t, `module t.s
import t.cast
scene X at room
  enter a
  #! allow offstage-speaker: "voice"
  a says: p
  #! allow belief-contradiction: "other rule"
  when after start
  #! allow time-cycle: "loop"
  born b when after X
  #! allow time-cycle: "loop"
  when before X
#! allow time-cycle: "loop"
dies a when after X
`)
	noErrors(t, diags)
	got := waivedEdges(w)
	want := []string{"t/t.s/X.end>t/t.cast/b.born", "t/t.s/X.end>t/t.s/X.start", "t/t.s/X.end>t/t.cast/a.died"}
	if !slices.Equal(got, want) {
		t.Errorf("waived edges = %v, want %v", got, want)
	}
}

func TestNestedGroupAudience(t *testing.T) {
	src := map[string]string{
		"cast.dieg": cast + "group crowd \"Crowd\"\n  members ws, c\n",
		"s.dieg": `module t.s
import t.cast
scene 1 at room
  enter a, crowd
  a whispers crowd: p
  w1 says: p
event e "off"
  a tells crowd: p
`}
	w, diags := Build(parse(t, src))
	noErrors(t, diags)
	cases := map[string][]string{
		"t/t.s/1#a1": {"crowd", "ws", "w1", "w2", "c"},
		"t/t.s/1#a2": {"a", "w2", "c"}, // w1's groups (ws, crowd) are not its audience
		"t/t.s/e#a1": {"crowd", "ws", "w1", "w2", "c"},
	}
	for act, want := range cases {
		if got := audience(w, act); !slices.Equal(got, want) {
			t.Errorf("%s audience = %v, want %v", act, got, want)
		}
	}
	var on []string
	for _, p := range w.Presence {
		on = append(on, p.EntityID[len("t/t.cast/"):])
	}
	if !slices.Equal(on, []string{"a", "crowd", "ws", "w1", "w2", "c"}) {
		t.Errorf("presence = %v", on)
	}
}
