package pg_test

import "testing"

// 사실 텍스트에 상대 시간("N일 뒤", "in N days")이 박혀 있으면 경고한다. 절대 날짜는 조용하다.
func TestRelativeTimeFact(t *testing.T) {
	pool := open(t)
	cases := []struct {
		fact string
		want int
	}{
		{`fact v "열이틀 뒤 투표가 열린다" true`, 1},
		{`fact v "투표는 10월 3일에 열린다" true`, 0},
		{`fact v "the vote opens in twelve days" true`, 1},
	}
	for _, c := range cases {
		id := compileLoad(t, pool, c.fact+"\n")
		if got := of(lint(t, pool, id), "relative-time-fact", id); len(got) != c.want {
			t.Errorf("%s: relative-time-fact = %v, want %d", c.fact, got, c.want)
		}
	}
}

const retellScenes = `scene 1
  enter a, b
  a tells b: p
`

// 청자가 이미 믿는 사실을 같은 화자가 다시 알려 주면 경고한다(장면 3). 모르는 청자(c)면 조용하다.
func TestRetellKnown(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, retellScenes+`scene 3
  when after 1
  enter a, b
  a tells b: p
`)
	if got := of(lint(t, pool, id), "retell-known", id); len(got) != 1 || got[0] != "t.s/3#a1/warn" {
		t.Errorf("retell-known = %v, want scene 3 only", got)
	}
	id = compileLoad(t, pool, retellScenes+`scene 3
  when after 1
  enter a, c
  a tells c: p
`)
	if got := of(lint(t, pool, id), "retell-known", id); len(got) != 0 {
		t.Errorf("retell-known = %v, want none for a new listener", got)
	}
}


// 같은 술어·같은 첫 인자인데 나머지 인자가 다른 참 사실 둘 → 경고. 하나가 not 이거나 나머지 인자가 같으면 조용하다.
func TestFactConflict(t *testing.T) {
	pool := open(t)
	cases := []struct {
		name, body string
		want       int
	}{
		{"conflict", "fact f1 = broke(lens, a)\nfact f2 = broke(lens, b)\n", 1},
		{"one negated", "fact f1 = broke(lens, a)\nfact f2 = not broke(lens, b)\n", 0},
		{"alias of same", "fact f1 = broke(lens, a)\nfact f2 = f1\n", 0},
		{"same args", "fact f1 = broke(lens, a)\nfact f2 = broke(lens, a)\n", 0},
	}
	for _, c := range cases {
		id := compileLoad(t, pool, "thing lens \"L\"\n"+c.body)
		if got := of(lint(t, pool, id), "fact-conflict", id); len(got) != c.want {
			t.Errorf("%s: fact-conflict = %v, want %d", c.name, got, c.want)
		}
	}
}
