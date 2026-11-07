package pg_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"diegesis/internal/pg"
	"diegesis/internal/write"
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
