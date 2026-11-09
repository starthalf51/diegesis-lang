package pg_test

import (
	"encoding/json"
	"slices"
	"testing"
)

// Injury site shoulder vs leg → warn 1. The target is the later assertion. Re-asserting the same value and the same value as the background (declaration block) are not conflicts.
// attr inside an account is account-variance's job, so it is left out. Suppression attaches to the target statement like other rules.
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

// An assertion of age 30, with a student age (20) assertion a few months earlier → warn (target = the later-in-time 30).
// Without bounds, precedes alone catches the age decreasing.
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

// 20 at t, 30 at t+10y → 0. An age increasing with no bounds, an error within 1 year, an alias id (same fact), and a difference in integer notation are also 0.
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

// It judges with only one bound too (fix 1). Upper bound only: between an undated scene (≥ origin) and a year-1600 scene (epoch = 1600) it is
// within 1 year → 20 → 30 is a conflict. Lower bound only: 10y or more later → 20 → 21 is a conflict. With neither bound, only decrease (Age test above).
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

// Giving a scene length an upper bound with lasts creates hi even between two undated scenes (Task 4, Hamlet).
// Between 1 → 2 ≤ 1d + 30d + 1d → 20 → 30 is a conflict. 3 → 4 with no lasts has no upper bound, so 0 findings.
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

// An object's X.at changing across scenes is a move, not a conflict (object-location uses the most recent value). A general key is still a conflict.
func TestAttributeConflictAtIsMove(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `place hall "H"
place yard "Y"
thing box "B"
scene 1 at hall
  attr box.at = hall
  attr box.color = red
scene 2 at yard
  when after 1
  attr box.at = yard
  attr box.color = blue
`)
	want := []string{"t.s/2#attr2/warn"}
	if got := of(lint(t, pool, id), "attribute-conflict", id); !slices.Equal(got, want) {
		t.Errorf("attribute-conflict = %v, want %v", got, want)
	}
}
