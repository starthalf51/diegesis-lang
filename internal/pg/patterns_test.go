package pg_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// patternFixtures: per pattern one match (want = "target/span_from-span_to") + one non-match (the same situation with one condition removed).
// Loaded through the real path (syntax → sema → Load). The common declaration is lint_test's cast (o = knows oracle, p true, np = not p).
var patternFixtures = map[string]struct {
	match string
	want  []string
	none  string
}{
	"chekhov-gun": {
		match: `scene 1
  enter a
  setup gun promise: p
scene 2
  enter a
  payoff gun
`,
		want: []string{"t.s/gun/1-2"},
		none: `scene 1
  enter a
  setup gun hint: p
scene 2
  enter a
  payoff gun
`,
	},
	// State predicate: at the time of the prophecy b is alive (not true) and dead at the end of 2.
	"prophecy-fulfilled": {
		match: `fact b_dead = not alive(b)
scene 1
  enter o, a
  o tells a: b_dead
event e "b dies"
  when after 1
  when before 2
  dies b
scene 2
  enter a
`,
		want: []string{"t.s/1#a1/1-2"},
		none: `fact b_dead = not alive(b)
scene 1
  enter o, a
  o tells a: b_dead
scene 2
  enter a
`,
	},
	"dramatic-irony": {
		match: `scene 1
  enter a, b
  a aside: p
scene 2
  enter a, b
  a asks b: p
`,
		want: []string{"t.s/2/2-2"},
		none: `scene 1
  enter a, b
  a tells b: p
scene 2
  enter a, b
  a asks b: p
`,
	},
	"mystery": {
		match: `scene 1
  enter a, b
  withhold p until 3
  a sees: p
scene 2
  enter a, b
  b asks a: p
scene 3
  enter a, b
  a tells b: p
`,
		want: []string{"t.s/p/1-3"},
		none: `scene 1
  enter a, b
  withhold p until 3
  a sees: p
scene 2
  enter a, b
scene 3
  enter a, b
  a tells b: p
`,
	},
	"red-herring": {
		match: `fact q "Q" false
scene 1
  enter a, b
  a tells b: q
scene 2
  enter a, b
  b tells a: not q
`,
		want: []string{"t.s/q/1-2"},
		none: `fact q "Q" false
scene 1
  enter a, b
  a tells b: q
scene 2
  enter a, b
`,
	},
	"reversal-reveal": {
		match: `fact q "Q" false
scene 1
  enter a, b, c
  a tells b: q
  b does flee because q
scene 2
  enter b, c
  c tells b: not q
`,
		want: []string{"t.s/q/1-2"},
		none: `fact q "Q" false
scene 1
  enter a, b, c
  a tells b: q
scene 2
  enter b, c
  c tells b: not q
`,
	},
	// A lie → b believes and tells c → c acts on that belief. Non-match: the same chain but the source is not a lie.
	"deception-chain": {
		match: `fact q "Q" false
scene 1
  enter a, b
  a lies b: q
scene 2
  when after 1
  enter b, c
  b tells c: q
  c does flee because q
`,
		want: []string{"t.s/2#a2/1-2"},
		none: `fact q "Q" false
scene 1
  enter a, b
  a tells b: q
scene 2
  when after 1
  enter b, c
  b tells c: q
  c does flee because q
`,
	},
	"counter-deception": {
		match: `fact q "Q" false
scene 1
  enter a, b
  a lies b: q
  b doubts q
`,
		want: []string{"t.s/1#a1/1-1"},
		none: `fact q "Q" false
scene 1
  enter a, b
  a lies b: q
`,
	},
	// bookend is exercised by TestStructureBookend (its target is the work, whose ID is random); this entry only registers the file.
	"bookend": {},
	"recognition": {
		match: `fact who "the stranger is A" true tag identity
scene 1
  enter a, b
  a sees: who
scene 2
  enter a, c
  a tells c: who
`,
		want: []string{"t.s/who/1-2"},
		none: `fact who "the stranger is A" true tag identity
scene 1
  enter a, b
  a tells b: who
scene 2
  enter a, c
  a tells c: who
`,
	},
}

// matches collects one pattern's matches as "target (without work ID)/span_from-span_to" (a NULL span is "-").
func matches(ms []pg.Match, pattern, work string) []string {
	var out []string
	for _, m := range ms {
		if m.Pattern == pattern {
			out = append(out, fmt.Sprintf("%s/%s-%s", strings.TrimPrefix(m.TargetID, work+"/"), ord(m.SpanFrom), ord(m.SpanTo)))
		}
	}
	slices.Sort(out)
	return out
}

func ord(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func runPatterns(t *testing.T, body string) ([]pg.Match, string) {
	t.Helper()
	pool := open(t)
	id := compileLoad(t, pool, body)
	ms, err := pg.Patterns(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return ms, id
}

func checkPattern(t *testing.T, id string) {
	t.Helper()
	fx, ok := patternFixtures[id]
	if !ok {
		t.Fatalf("no fixture for %s", id)
	}
	ms, w := runPatterns(t, fx.match)
	if got := matches(ms, id, w); !slices.Equal(got, fx.want) {
		t.Errorf("match fixture: %s = %v, want %v", id, got, fx.want)
	}
	ms, w = runPatterns(t, fx.none)
	if got := matches(ms, id, w); len(got) != 0 {
		t.Errorf("non-matching fixture: %s = %v, want none", id, got)
	}
}

func TestPattern_chekhov_gun(t *testing.T)        { checkPattern(t, "chekhov-gun") }
func TestPattern_prophecy_fulfilled(t *testing.T) { checkPattern(t, "prophecy-fulfilled") }
func TestPattern_dramatic_irony(t *testing.T)     { checkPattern(t, "dramatic-irony") }
func TestPattern_mystery(t *testing.T)            { checkPattern(t, "mystery") }
func TestPattern_red_herring(t *testing.T)        { checkPattern(t, "red-herring") }
func TestPattern_reversal_reveal(t *testing.T)    { checkPattern(t, "reversal-reveal") }
func TestPattern_deception_chain(t *testing.T)    { checkPattern(t, "deception-chain") }
func TestPattern_counter_deception(t *testing.T)  { checkPattern(t, "counter-deception") }
func TestPattern_recognition(t *testing.T)        { checkPattern(t, "recognition") }

// Every pattern file has a fixture, and the header is in the right format.
func TestPatternLibrary(t *testing.T) {
	ps, err := pg.PatternDefs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != len(patternFixtures) {
		t.Errorf("%d pattern files, %d fixtures", len(ps), len(patternFixtures))
	}
	for _, p := range ps {
		if _, ok := patternFixtures[p.ID]; !ok {
			t.Errorf("pattern %s has no fixture", p.ID)
		}
	}
}

// Coder-type prophecy: already true at the time of the prophecy (role holding) but the listener does not know (undetermined) → a non-prophet asserts it again when it is true.
// If that person repeats what they heard from the prophecy (the source is the prophet), it is not confirmation.
func TestPatternProphecyConfirmed(t *testing.T) {
	ms, w := runPatterns(t, `role chief "Chief"
  held a from start
fact a_chief = holds(chief, a)
b believes a_chief
scene 1
  enter o, a
  o tells a: a_chief
  exit o
  enter b
  b tells a: a_chief
`)
	if got, want := matches(ms, "prophecy-fulfilled", w), []string{"t.s/1#a1/1-1"}; !slices.Equal(got, want) {
		t.Errorf("confirmed = %v, want %v", got, want)
	}
	ms, w = runPatterns(t, `role chief "Chief"
  held a from start
fact a_chief = holds(chief, a)
scene 1
  enter o, a, b
  o tells a: a_chief
  b tells a: a_chief
`)
	if got := matches(ms, "prophecy-fulfilled", w); len(got) != 0 {
		t.Errorf("echo of the prophecy counted as confirmation: %v", got)
	}
}

// Author declaration: with no match, lint-path warn pattern-missing (suppressible); with a match, no diagnostic.
// `at <scene>` counts only matches touching that scene.
func TestPatternMissing(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `pattern red-herring
#! allow pattern-missing: "fixture"
pattern mystery
pattern chekhov-gun
pattern chekhov-gun at 3
scene 1
  enter a
  setup gun promise: p
scene 2
  enter a
  payoff gun
scene 3
  enter a
`)
	got := of(lint(t, pool, id), "pattern-missing", id)
	want := []string{"t.s#mystery/warn/waived", "t.s#red-herring/warn", "t.s/3#chekhov-gun/warn"}
	if !slices.Equal(got, want) {
		t.Errorf("pattern-missing = %v, want %v", got, want)
	}
}

// A belief definitely withdrawn by doubt is not knowledge (not even maybe): a character who heard and then doubted is the object of irony in the next scene.
func TestIronyAfterDoubt(t *testing.T) {
	ms, w := runPatterns(t, `scene 1
  enter a, b
  a aside: p
scene 2
  when after 1
  enter a, b
  a tells b: p
  b doubts p
scene 3
  when after 2
  enter a, b
  a asks b: p
`)
	if got, want := matches(ms, "dramatic-irony", w), []string{"t.s/2/2-2", "t.s/3/3-3"}; !slices.Equal(got, want) {
		t.Errorf("dramatic-irony = %v, want %v", got, want)
	}
}

// Fix round 1: lint's partial run (declared patterns only) does not overwrite the full result (the pattern_result view).
func TestPatternResultSurvivesLint(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `fact q "Q" false
pattern chekhov-gun
scene 1
  enter a, b
  setup gun promise: p
  a tells b: q
scene 2
  enter a, b
  payoff gun
  b tells a: not q
`)
	ms, err := pg.Patterns(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	lint(t, pool, id)
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pattern_result WHERE work_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if len(ms) < 2 || n != len(ms) {
		t.Errorf("pattern_result after lint = %d rows, want the full run's %d (≥ 2)", n, len(ms))
	}
}

// Fix round 1: a character who merely has no record but did not touch the fact is not counted in the irony (c did not hear the whisper).
func TestIronyOnlyEngagedCharacters(t *testing.T) {
	ms, w := runPatterns(t, `scene 1
  enter a, b
  a aside: p
scene 2
  when after 1
  enter a, b, c
  a whispers b: p
`)
	var who []string
	for _, m := range ms {
		if m.Pattern == "dramatic-irony" {
			var d struct {
				Irony []struct{ Character string } `json:"irony"`
			}
			if err := json.Unmarshal(m.Data, &d); err != nil {
				t.Fatal(err)
			}
			for _, x := range d.Irony {
				who = append(who, strings.TrimPrefix(x.Character, w+"/"))
			}
		}
	}
	if want := []string{"t.s/b"}; !slices.Equal(who, want) {
		t.Errorf("irony characters = %v, want %v", who, want)
	}
}

// Fix round 1: confirmation must be made to someone who heard the prophecy. Revealing the truth among people unrelated to the prophecy is not realization.
func TestProphecyConfirmationNeedsTheHearer(t *testing.T) {
	ms, w := runPatterns(t, `role chief "Chief"
  held a from start
fact a_chief = holds(chief, a)
b believes a_chief
scene 1
  enter o, a
  o tells a: a_chief
  exit o, a
  enter b, c
  b tells c: a_chief
`)
	if got := matches(ms, "prophecy-fulfilled", w); len(got) != 0 {
		t.Errorf("confirmation to someone who never heard the prophecy counted: %v", got)
	}
}

// enacts (performance/re-enactment): everyone on stage is audience but it is not an assertion and makes no belief.
// b who watched the re-enactment in scene 2 still does not know p in scene 3 → irony in both scenes (engaged = the audience of enacts).
func TestEnactsIsNotBelief(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  a aside: p
scene 2
  when after 1
  enter a, b
  a enacts: p
scene 3
  when after 2
  enter a, b
  a asks b: p
`)
	var n, aud int
	if err := pool.QueryRow(context.Background(), `SELECT
	    (SELECT count(*) FROM believes WHERE work_id = $1 AND entity_id = $1 || '/t.s/b'),
	    (SELECT count(*) FROM audience au JOIN act a ON a.id = au.act_id WHERE a.work_id = $1 AND a.verb = 'enacts' AND au.entity_id = $1 || '/t.s/b')`,
		id).Scan(&n, &aud); err != nil {
		t.Fatal(err)
	}
	if n != 0 || aud != 1 {
		t.Errorf("b believes rows = %d (want 0), enacts audience b = %d (want 1)", n, aud)
	}
	ms, err := pg.Patterns(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := matches(ms, "dramatic-irony", id), []string{"t.s/2/2-2", "t.s/3/3-3"}; !slices.Equal(got, want) {
		t.Errorf("dramatic-irony = %v, want %v", got, want)
	}
}
