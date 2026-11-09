package pg_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/sema"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

// Warn if a fact text has a relative time embedded ("N days later" in Korean, "in N days"). An absolute date is quiet.
func TestRelativeTimeFact(t *testing.T) {
	pool := open(t)
	cases := []struct {
		fact string
		want int
	}{
		{`fact v "열이틀 뒤 투표가 열린다" true`, 1},
		{`fact v "투표는 10월 3일에 열린다" true`, 0},
		{`fact v "the vote opens in twelve days" true`, 1},
		{`fact v "사흘 뒤 문이 열린다" true`, 1},
		{`fact v "한 달 뒤 문이 열린다" true`, 1},
		{`fact v "서진이 상자를 확인한 뒤 자리를 떠났다" true`, 0},
		{`fact v "그 일 후 아무도 말하지 않았다" true`, 0},
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

// Warn if the same speaker tells a listener something they already believe (scene 3). Quiet if the listener doesn't know (c).
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


// Two true facts with the same predicate and first argument but different remaining arguments → warn. Quiet if one is not or the remaining arguments are the same.
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

// compileLoadFiles is like compileLoad but loads several files (cast in the first) as one work.
func compileLoadFiles(t *testing.T, pool *pgxpool.Pool, bodies ...string) string {
	t.Helper()
	id := fmt.Sprintf("t_%d", rand.Int63())
	var fs []*syntax.File
	for i, b := range bodies {
		if i == 0 {
			b = cast + b
		}
		f, diags := syntax.ParseFile(fmt.Sprintf("f%d.dieg", i), []byte(strings.ReplaceAll(b, "WK", id)))
		for _, d := range diags {
			if d.Severity == "error" {
				t.Fatalf("parse: %s", d)
			}
		}
		fs = append(fs, f)
	}
	w, ds := sema.Build(fs)
	for _, d := range ds {
		if d.Severity == "error" {
			t.Fatalf("compile: %s", d)
		}
	}
	cleanup(t, pool, id)
	if err := pg.Load(context.Background(), pool, w); err != nil {
		t.Fatal(err)
	}
	return id
}

const lensA = `fact lens_smashed "백영호가 거절하자 도경태가 직접 등실에 올라가 렌즈를 깨뜨려 등대 불을 꺼뜨렸고 그때 오른손을 베었다" true
scene 1
  enter a, b
  a tells b: lens_smashed
`
const lensB = `module t.s2
import t.s
fact lens_broke_in_fight "다투던 끝에 도경태가 뿌리친 오른손이 렌즈를 깨뜨렸고, 그 길로 등대 불이 꺼졌다" true
scene 2
  when after 1
  enter a, b
  a tells b: lens_broke_in_fight
`

// Two true facts that are text only, in different files with the same leading token and both narrated, warn a candidate conflict (B10).
// Quiet if in the same file, not narrated, or the leading tokens differ.
func TestFactConflictTextFacts(t *testing.T) {
	pool := open(t)
	id := compileLoadFiles(t, pool, lensA, lensB)
	got := msgs(lint(t, pool, id), "fact-conflict")
	if len(got) != 1 || !strings.Contains(got[0], "candidate conflict") {
		t.Errorf("cross-file text facts: %v, want one candidate conflict", got)
	}
	id = compileLoad(t, pool, lensA+`fact lens_broke_in_fight "다투던 끝에 도경태가 뿌리친 오른손이 렌즈를 깨뜨렸고, 그 길로 등대 불이 꺼졌다" true
scene 2
  when after 1
  enter a, b
  a tells b: lens_broke_in_fight
`)
	if got := of(lint(t, pool, id), "fact-conflict", id); len(got) != 0 {
		t.Errorf("same file: %v, want none", got)
	}
	id = compileLoadFiles(t, pool, lensA, `module t.s2
import t.s
fact lens_broke_in_fight "다투던 끝에 도경태가 뿌리친 오른손이 렌즈를 깨뜨렸고, 그 길로 등대 불이 꺼졌다" true
`)
	if got := of(lint(t, pool, id), "fact-conflict", id); len(got) != 0 {
		t.Errorf("unnarrated: %v, want none", got)
	}
	id = compileLoadFiles(t, pool, lensA, strings.ReplaceAll(lensB, "lens_broke", "tide_broke"))
	if got := of(lint(t, pool, id), "fact-conflict", id); len(got) != 0 {
		t.Errorf("different token: %v, want none", got)
	}
}

// An utterance with someone who doesn't know yet (c) mixed into the audience is not repetition (announcement, late revelation).
func TestRetellKnownMixedAudience(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, retellScenes+`scene 3
  when after 1
  enter a, b, c
  a tells b, c: p
`)
	if got := of(lint(t, pool, id), "retell-known", id); len(got) != 0 {
		t.Errorf("retell-known = %v, want none for a mixed audience", got)
	}
}

// retell-known exclusion cases: a doubt in between, a different speaker, a different quote, and sends are not repetition. The same words exactly are caught.
func TestRetellKnownExclusions(t *testing.T) {
	pool := open(t)
	for name, c := range map[string]struct {
		body string
		want int
	}{
		"plain repeat": {retellScenes + "scene 3\n  when after 1\n  enter a, b\n  a tells b: p\n", 1},
		"intervening doubt": {retellScenes + "scene 2\n  when after 1\n  enter b\n  b doubts p\nscene 3\n  when after 2\n  enter a, b\n  a tells b: p\n", 0},
		"different speaker": {retellScenes + "scene 3\n  when after 1\n  enter b, c\n  c tells b: p\n", 0},
		"unequal quotes":    {"scene 1\n  enter a, b\n  a tells b: p \"하나\"\nscene 3\n  when after 1\n  enter a, b\n  a tells b: p \"둘\"\n", 0},
		"sends":             {retellScenes + "scene 3\n  when after 1\n  enter a, m\n  a sends m to b with p\n", 0},
	} {
		id := compileLoad(t, pool, c.body)
		if got := of(lint(t, pool, id), "retell-known", id); len(got) != c.want {
			t.Errorf("%s: retell-known = %v, want %d", name, got, c.want)
		}
	}
}
