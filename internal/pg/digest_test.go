package pg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// The box is in the hall, and in episode 1 a saw a private fact (secret). Look at the summary for scene 2.
const digestBody = `place hall "홀"
thing box "녹슨 상자"
  name ko "녹슨 상자"
  attr box.at = hall
fact secret "비밀문장XYZ" true
scene 1
  enter a, b
  withhold secret until 3
  a sees: secret
  a sees box
scene 2
  enter a, b
  a sees box
scene 3
  enter a, b
  a tells b: secret
`

func digestFor(t *testing.T) string {
	t.Helper()
	pool := open(t)
	id := compileLoad(t, pool, digestBody)
	d, _, err := pg.Digest(context.Background(), pool, id, "2")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func digestOf(t *testing.T, body, scene string) (text, state string) {
	t.Helper()
	pool := open(t)
	id := compileLoad(t, pool, body)
	text, state, err := pg.Digest(context.Background(), pool, id, scene)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// If the setup planted or paid off in this scene is a private fact, its source text must not appear in the summary (negative propositions included).
func TestDigestOmitsWithheldSetup(t *testing.T) {
	for _, prop := range []string{"hid", "nhid"} {
		body := `fact hid "숨김설정ABC" true
fact nhid = not hid
scene 1
  enter a, b
scene 2
  enter a, b
  withhold hid until 3
  setup s2 hint: ` + prop + `
  a sees: hid
scene 3
  enter a, b
  a tells b: hid
`
		text, _ := digestOf(t, body, "2")
		if strings.Contains(text, "숨김설정ABC") || strings.Contains(text, "복선") {
			t.Errorf("%s: withheld setup leaked:\n%s", prop, text)
		}
	}
}

const stateBody = `place hall "홀"
place yard "마당"
thing box "녹슨 상자"
  name ko "녹슨 상자"
  attr box.at = LOC
fact secret "비밀문장XYZ" true
scene 1
  enter a, b
  ACT
scene 2
  enter a, b
  a sees box
`

// The string for the state hash holds only the state section: it stays the same when the act mentions in earlier scenes change, and differs when an object's location attribute changes.
func TestDigestStateOnlyPart(t *testing.T) {
	mk := func(loc, act string) (string, string) {
		return digestOf(t, strings.NewReplacer("LOC", loc, "ACT", act).Replace(stateBody), "2")
	}
	t0, s0 := mk("hall", "a sees box")
	t1, s1 := mk("hall", "b sees box")
	if t0 == t1 {
		t.Fatalf("fixture: act change should change the full text:\n%s", t0)
	}
	if s0 != s1 {
		t.Errorf("recent-act change altered the state part:\n%s\n---\n%s", s0, s1)
	}
	if _, s2 := mk("yard", "a sees box"); s0 == s2 {
		t.Errorf("location change did not alter the state part:\n%s", s0)
	}
	if strings.Contains(s0, "이미 일어난") {
		t.Errorf("state part holds the acts section:\n%s", s0)
	}
}

func TestDigestSections(t *testing.T) {
	d := digestFor(t)
	if !strings.HasPrefix(d, "## 이 시점의 세계 상태 (어기지 말 것)") {
		t.Errorf("heading: %q", d)
	}
	if !strings.Contains(d, "녹슨 상자(box): at = ") {
		t.Errorf("no thing location attribute:\n%s", d)
	}
	if strings.Contains(d, "독자가 이미 아는") || strings.Contains(d, "독자에게 이미 서술된") {
		t.Errorf("reader-knows section must be dropped:\n%s", d)
	}
}

func TestDigestOmitsWithheld(t *testing.T) {
	if d := digestFor(t); strings.Contains(d, "비밀문장XYZ") {
		t.Errorf("withheld fact text leaked:\n%s", d)
	}
}

// An object moved in a later scene: the earlier scene's summary shows only the old location, the moved scene's summary only the new location (no future-value leak).
func TestDigestAttrTimeFilter(t *testing.T) {
	body := `place hall "홀"
place yard "마당"
thing box "녹슨 상자"
  name ko "녹슨 상자"
  attr box.at = hall
scene 1 at hall
  enter a, b
  a sees box
scene 2 at yard
  when after 1
  enter a, b
  attr box.at = yard
  a sees box
`
	_, s1 := digestOf(t, body, "1")
	if !strings.Contains(s1, "at = 홀") || strings.Contains(s1, "마당(") || strings.Contains(s1, "at = 마당") {
		t.Errorf("scene 1 leaks the later location:\n%s", s1)
	}
	_, s2 := digestOf(t, body, "2")
	if !strings.Contains(s2, "at = 마당") || strings.Contains(s2, "at = 홀") {
		t.Errorf("scene 2 must show only the latest location:\n%s", s2)
	}
}
