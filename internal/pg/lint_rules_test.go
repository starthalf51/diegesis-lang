package pg_test

import (
	"slices"
	"strings"
	"testing"
)

// fixture: bad 는 규칙을 위반하고(want = 그 규칙의 진단), good 은 같은 상황의 정상판(진단 0).
type fixture struct {
	bad  string
	want []string
	good string
}

var fixtures = map[string]fixture{
	"info-flow": {
		bad: `scene 1
  enter a, b
  a tells b: p
`,
		want: []string{"t.s/1#a1/warn"},
		good: `scene 1
  enter a, b, o
  o tells a: p
  a tells b: p
`,
	},
	"letter-unknown-info": {
		bad: `scene 1
  enter a
  a writes letter
`,
		want: []string{"t.s/1#a1/warn"},
		good: `scene 1
  enter a, o
  o tells a: p
  a writes letter
`,
	},
	"undelivered-message": {
		bad: `scene 1
  enter a, m, o
  o tells a: p
  a sends m to b with p
`,
		want: []string{"t.s/1#a2/warn"},
		good: `scene 1
  enter a, m, o
  o tells a: p
  a sends m to b with p
scene 2
  when after 1
  enter m, b
  m tells b: p
`,
	},
	"unpaid-promise": {
		bad: `scene 1
  setup s1 promise: p
`,
		want: []string{"t.s/s1/warn"},
		good: `scene 1
  setup s1 promise: p
  payoff s1
`,
	},
	"payoff-without-setup": {
		bad: `scene 1
  setup s1 hint: p
scene 2
  when before 1
  payoff s1
`,
		want: []string{"t.s/s1/warn"},
		good: `scene 1
  setup s1 hint: p
scene 2
  when after 1
  payoff s1
`,
	},
	"belief-contradiction": {
		bad: `scene 1
  enter a, b
  a sees: np
  a tells b: p
`,
		want: []string{"t.s/1#a2/warn"},
		good: `scene 1
  enter a, b
  a tells b: p
  a sees: np
`,
	},
	"dead-actor": {
		bad: `scene 1
  enter a
  a thinks: "x"
dies a when before 1
`,
		want: []string{"t.s/1#a1/warn"},
		good: `scene 1
  enter a
  a thinks: "x"
dies a when after 1
`,
	},
	"because-unbelieved": {
		bad: `scene 1
  enter a
  a does flee because p
`,
		want: []string{"t.s/1#a1/warn"},
		good: `scene 1
  enter a
  a sees: p
  a does flee because p
`,
	},
	"account-variance": {
		bad: `event ev "E"
account acc1 of ev by a
  a tells b: p
  a tells b: "x"
account acc2 of ev by b
  b tells a: np
`,
		want: []string{"t.s/acc1#a1/info", "t.s/acc2#a1/info"},
		good: `event ev "E"
account acc1 of ev by a
  a tells b: p
account acc2 of ev by b
  b tells a: p
`,
	},
	"time-cycle": {
		bad: `scene 1
  when after 2
scene 2
  when after 1
`,
		want: []string{"t.s/1/warn"}, // 대상 = 순환 간선을 만든 첫 `when` 문장(Task 11). src 는 가장 앞선 간선
		good: `scene 1
  when after 2
scene 2
`,
	},
	"inferred-deception": {
		bad: `a believes np
scene 1
  enter a
  a writes letter
`,
		want: []string{"t.s/1#a1/info"},
		good: `scene 1
  enter a, o
  o tells a: p
  a writes letter
`,
	},
}

func checkRule(t *testing.T, rule string) {
	pool := open(t)
	fx := fixtures[rule]
	bad := compileLoad(t, pool, fx.bad)
	if got := of(lint(t, pool, bad), rule, bad); !slices.Equal(got, fx.want) {
		t.Errorf("violating: %s = %v, want %v", rule, got, fx.want)
	}
	good := compileLoad(t, pool, fx.good)
	if got := of(lint(t, pool, good), rule, good); len(got) != 0 {
		t.Errorf("clean: %s = %v, want none", rule, got)
	}
}

func TestRule_info_flow(t *testing.T)            { checkRule(t, "info-flow") }
func TestRule_letter_unknown_info(t *testing.T)  { checkRule(t, "letter-unknown-info") }
func TestRule_undelivered_message(t *testing.T)  { checkRule(t, "undelivered-message") }
func TestRule_unpaid_promise(t *testing.T)       { checkRule(t, "unpaid-promise") }
func TestRule_payoff_without_setup(t *testing.T) { checkRule(t, "payoff-without-setup") }
func TestRule_belief_contradiction(t *testing.T) { checkRule(t, "belief-contradiction") }
func TestRule_dead_actor(t *testing.T)           { checkRule(t, "dead-actor") }
func TestRule_because_unbelieved(t *testing.T)   { checkRule(t, "because-unbelieved") }
func TestRule_account_variance(t *testing.T)     { checkRule(t, "account-variance") }
func TestRule_time_cycle(t *testing.T)           { checkRule(t, "time-cycle") }
func TestRule_inferred_deception(t *testing.T)   { checkRule(t, "inferred-deception") }

// 순서가 정해지지 않은 출처(형제 장면)는 warn 이 아니라 info(Review Focus 2).
// writes / because 는 각자의 규칙이 맡고 info-flow 는 겹쳐 내지 않는다.
func TestInfoFlowMaybeAndPartition(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, o
  o tells a: p
scene 2
  enter a, b
  a tells b: p
  a writes letter
  a does flee because p
`)
	ds := lint(t, pool, id)
	for rule, want := range map[string][]string{
		"info-flow":           {"t.s/2#a1/info"},
		"letter-unknown-info": {"t.s/2#a2/info"},
		"because-unbelieved":  {"t.s/2#a3/info"},
	} {
		if got := of(ds, rule, id); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", rule, got, want)
		}
	}
}

// 선후가 정해지지 않은 payoff 는 info, 전달(형제 장면)도 info.
func TestMaybeOrderIsInfo(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, m, o
  o tells a: p
  setup s1 hint: p
  a sends m to b with p
scene 2
  enter m, b
  payoff s1
  m tells b: p
`)
	ds := lint(t, pool, id)
	for rule, want := range map[string][]string{
		"payoff-without-setup": {"t.s/s1/info"},
		"undelivered-message":  {"t.s/1#a2/info"},
	} {
		if got := of(ds, rule, id); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", rule, got, want)
		}
	}
}

// 기록이 셋 이상이면 같은 행위·명제가 빠진 기록마다 따로 나오지 않고 한 건으로 모인다(Task 10).
// 빠진 기록과 반대로 적은 기록은 따로 센다(종류가 다르다).
func TestAccountVarianceOnePerProp(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `event ev "E"
account acc1 of ev by a
  a tells b: p
account acc2 of ev by b
  b tells a: "x"
account acc3 of ev by b
  b tells a: "x"
account acc4 of ev by b
  b tells a: np
`)
	ds := lint(t, pool, id)
	want := []string{"t.s/acc1#a1/info", "t.s/acc1#a1/info", "t.s/acc4#a1/info", "t.s/acc4#a1/info"}
	if got := of(ds, "account-variance", id); !slices.Equal(got, want) {
		t.Errorf("account-variance = %v, want %v", got, want)
	}
	var msg string
	for _, d := range ds {
		if d.Rule == "account-variance" && d.TargetID == id+"/t.s/acc1#a1" && strings.Contains(d.Message, "absent") {
			msg = d.Message
		}
	}
	if !strings.Contains(msg, "acc2") || !strings.Contains(msg, "acc3") {
		t.Errorf("collapsed message should name every account lacking p: %q", msg)
	}
}
