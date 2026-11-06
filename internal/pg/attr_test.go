package pg_test

import (
	"encoding/json"
	"slices"
	"testing"
)

// 부상 부위 shoulder vs leg → warn 1. 대상은 나중 단언. 같은 값 재단언·배경(선언 블록)과 같은 값은 충돌 아님.
// account 안의 attr 는 account-variance 몫이라 빠진다. 억제는 다른 규칙처럼 대상 문장에 붙는다.
func TestAttributeConflictStatic(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `character d "D"
  attr d.wound = shoulder
scene 1
  attr d.wound = shoulder
scene 2
  when after 1
  attr d.wound = leg
scene 3
  when after 2
  #! allow attribute-conflict: "the narrator misremembers"
  attr d.wound = hip
event ev "E"
account acc1 of ev by a
  attr d.wound = arm
`)
	ds := lint(t, pool, id)
	want := []string{"t.s/2#attr1/warn", "t.s/3#attr1/warn/waived"}
	if got := of(ds, "attribute-conflict", id); !slices.Equal(got, want) {
		t.Fatalf("attribute-conflict = %v, want %v", got, want)
	}
	for _, d := range ds {
		if d.Rule != "attribute-conflict" || d.TargetID != id+"/t.s/2#attr1" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(d.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data["value"] != "leg" || data["other_value"] != "shoulder" || data["src_id"] == nil || data["other_src_id"] == nil {
			t.Errorf("data = %v", data)
		}
		if d.Line != 19 || d.Col != 3 {
			t.Errorf("position = %d:%d, want the scene 2 attr line", d.Line, d.Col)
		}
	}
}

// 30세 단언, 그보다 몇 달 앞에서 학생 나이(20) 단언 → warn(대상 = 시간상 나중인 30세).
// 경계가 없으면 precedes 만으로 나이가 줄어드는 것을 잡는다.
func TestAttributeConflictAge(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  when at 1600-06
  attr a.age = 30
scene 2
  when at 1600-02
  attr a.age = 20
scene 3
  attr b.age = 30
scene 4
  when after 3
  attr b.age = 29
`)
	want := []string{"t.s/1#attr1/warn", "t.s/4#attr1/warn"}
	if got := of(lint(t, pool, id), "attribute-conflict", id); !slices.Equal(got, want) {
		t.Errorf("attribute-conflict = %v, want %v", got, want)
	}
}

// 20세 at t, 30세 at t+10y → 0. 경계 없이 나이가 느는 것, 1년 이내 오차, 별칭 id(같은 사실), 정수 표기 차이도 0.
func TestAttributeNoConflictAcrossTime(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `fact q = p
scene 1
  when at 1590
  attr a.age = 20
  attr a.oath = p
  attr a.rank = 7
scene 2
  when at 1600
  attr a.age = 30
  attr a.oath = q
  attr a.rank = 07
scene 3
  attr b.age = 20
scene 4
  when after 3
  attr b.age = 45
scene 5
  when after 3 by 0d..10d
  attr b.age = 21
`)
	if got := of(lint(t, pool, id), "attribute-conflict", id); len(got) != 0 {
		t.Errorf("attribute-conflict = %v, want none", got)
	}
}

// 한쪽 경계만 있어도 판정한다(fix 1). 위 경계만: 날짜 없는 장면(≥ origin)과 1600 년 장면(epoch = 1600) 사이는
// 1년 안 → 20 → 30 은 충돌. 아래 경계만: 10y 이상 뒤 → 20 → 21 은 충돌. 양쪽 다 없으면 감소만(위 Age 테스트).
func TestAttributeAgeOneSided(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  attr a.age = 20
scene 2
  when at 1600
  attr a.age = 30
scene 3
  attr b.age = 20
scene 4
  when after 3 by 10y..20y
  attr b.age = 21
scene 5
  attr c.age = 20
scene 6
  when after 5
  attr c.age = 60
`)
	want := []string{"t.s/2#attr1/warn", "t.s/4#attr1/warn"}
	if got := of(lint(t, pool, id), "attribute-conflict", id); !slices.Equal(got, want) {
		t.Errorf("attribute-conflict = %v, want %v", got, want)
	}
}

// lasts 로 장면 길이에 상한을 주면 날짜 없는 두 장면 사이에도 hi 가 생긴다(Task 4, 햄릿).
// 1 → 2 사이 ≤ 1d + 30d + 1d → 20 → 30 은 충돌. lasts 가 없는 3 → 4 는 상한이 없어 0건.
func TestAttributeAgeLasts(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  lasts 0d..1d
  attr a.age = 20
scene 2
  when after 1 by 0d..30d
  lasts 0d..1d
  attr a.age = 30
scene 3
  attr b.age = 20
scene 4
  when after 3 by 0d..30d
  attr b.age = 30
`)
	want := []string{"t.s/2#attr1/warn"}
	if got := of(lint(t, pool, id), "attribute-conflict", id); !slices.Equal(got, want) {
		t.Errorf("attribute-conflict = %v, want %v", got, want)
	}
}
