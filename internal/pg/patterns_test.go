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

// patternFixtures: 패턴마다 매치 1건(want = "대상/span_from-span_to") + 비매치 1건(같은 상황에서 조건 하나를 뺀 것).
// 실제 경로(syntax → sema → Load)로 적재한다. 공통 선언은 lint_test 의 cast(o = knows oracle, p 참, np = not p).
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
	// 상태 술어: 예언 시점에 b 는 살아 있고(참이 아님) 2 의 끝에는 죽어 있다.
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
	// 거짓말 → b 가 믿고 c 에게 전한다 → c 가 그 믿음으로 행동한다. 비매치: 같은 사슬인데 출처가 거짓말이 아니다.
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

// matches 는 한 패턴의 매치를 "대상(작품 ID 뺀 것)/span_from-span_to" 로 모은다(NULL 구간은 "-").
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

// 모든 패턴 파일에 픽스처가 있고, 헤더가 형식대로다.
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

// 코더형 예언: 예언 시점에 이미 참이지만(역할 보유) 청자는 모른다(미정) → 예언자가 아닌 사람이 참일 때 다시 단언한다.
// 그 사람이 예언에서 들은 것을 되풀이하면(출처가 예언자) 확인이 아니다.
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

// 작가 선언: 매치가 없으면 lint 경로의 warn pattern-missing(억제 가능), 매치가 있으면 진단 없음.
// `at <scene>` 은 그 장면에 걸친 매치만 센다.
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

// 의심으로 확실히 거둬진 믿음은 앎이 아니다(maybe 도 아니다): 들었다가 의심한 인물은 다음 장면에서 아이러니의 대상이다.
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

// 수정 1회차: lint 의 부분 실행(선언된 패턴만)이 전체 결과(pattern_result 뷰)를 덮지 않는다.
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

// 수정 1회차: 기록이 없을 뿐 그 사실에 닿지 않은 인물은 아이러니에 세지 않는다(c 는 귓속말을 듣지 못했다).
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

// 수정 1회차: 확인은 예언을 들은 사람에게 해야 한다. 예언과 무관한 사람들 사이에서 참을 밝히는 것은 실현이 아니다.
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

// enacts(공연·재연): 무대 위 모두가 청중이지만 단언이 아니라 믿음을 만들지 않는다.
// 장면 2 의 재연을 본 b 는 장면 3 에서도 p 를 모른다 → 두 장면 모두 아이러니(engaged = enacts 의 청중).
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
