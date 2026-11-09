package pg_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/write"
)

func lineWith(s string, subs ...string) string {
	for _, l := range strings.Split(s, "\n") {
		ok := true
		for _, x := range subs {
			ok = ok && strings.Contains(l, x)
		}
		if ok {
			return l
		}
	}
	return ""
}

func TestStateDigestMacbeth(t *testing.T) {
	pool := open(t)
	w, id := corpusIR(t, "macbeth")
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	d, err := write.StateDigest(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(d); n > 8000 {
		t.Errorf("digest is %d chars, cap 8000", n)
	}
	if lineWith(d, "duncan", "사망") == "" {
		t.Errorf("no duncan death line:\n%s", d)
	}
	if lineWith(d, "banquo_kings") == "" {
		t.Errorf("no banquo_kings setup:\n%s", d)
	}
	_, after, ok := strings.Cut(d, "## 거짓 믿음")
	sec, _, _ := strings.Cut(after, "\n## ")
	if !ok || !strings.Contains(sec, "\n- ") {
		t.Errorf("no false belief:\n%s", d)
	}
	for _, liar := range []string{"- lady_macbeth: murder_plot", "- macbeth: murder_plot"} {
		if l := lineWith(d, liar); l != "" {
			t.Errorf("liar listed as deceived: %s", l)
		}
	}
	if strings.Contains(d, "## 사실") {
		t.Error("digest repeats the cast's fact list")
	}
	if testing.Verbose() {
		t.Log(d)
	}
	d2, _ := write.StateDigest(context.Background(), pool, id)
	if d2 != d {
		t.Error("digest is not deterministic")
	}
}

// Review Focus 4: a withheld fact not yet revealed by the last scene carries [독자 비공개].
func TestStateDigestWithheld(t *testing.T) {
	pool := open(t)
	// s9 is not written yet: a time reference must resolve, so it stands as an offstage event (never shown).
	id := compileLoad(t, pool, `scene s1
  enter a, b
  withhold p until s9
  a tells b: p
scene s2
  enter a
scene s3
  enter a
event s9 "later"
`)
	d, err := write.StateDigest(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if lineWith(d, "- p", "[독자 비공개]") == "" {
		t.Errorf("p is not marked hidden:\n%s", d)
	}
}

// The liar is not deceived by their own lie, even after hearing it back.
func TestStateDigestLiarNotDeceived(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene s1
  enter a, b
  a lies b: np
  b says a: np
`)
	d, err := write.StateDigest(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if lineWith(d, "- b: p", "← a 의 기만") == "" {
		t.Errorf("b's false belief missing:\n%s", d)
	}
	if l := lineWith(d, "- a: p"); l != "" {
		t.Errorf("liar listed as deceived: %s", l)
	}
	if d2, _ := write.StateDigest(context.Background(), pool, id); d2 != d {
		t.Error("digest is not deterministic")
	}
}

// Offstage events (never shown) share discourse_ord 0; the digest's "previous scene" lookup must only count shown scenes.
func TestDigestTwoOffstageEvents(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene s1
  enter a, b
  a tells b: p
event e1 "first"
event e2 "second"
`)
	var sid string
	if err := pool.QueryRow(context.Background(), `SELECT id FROM scene WHERE work_id=$1 AND shown ORDER BY discourse_ord LIMIT 1`, id).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pg.Digest(context.Background(), pool, id, sid); err != nil {
		t.Fatal(err)
	}
}
