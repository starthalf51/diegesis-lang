package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// judgeHead is Plan 3's checklist (tools/bench/judge.go) with the serial's episode headers.
const judgeHead = `# 과제: 두 연재 소설의 연속성 오류 판정

같은 아이디어로 쓴 한국어 연재 소설 두 편(X, Y)이 있다. 각 소설 안에서 **객관적인 연속성 오류**만 찾는다.
문체·재미·완성도는 판정하지 않는다. 두 소설을 서로 비교하지 않는다(서로 다른 이야기다).

## 체크리스트 (이 일곱 가지만)
1. knowledge — 인물이 그 시점에 알 수 없는 것을 안다(본 적도 들은 적도 없는 사실을 말하거나 그에 따라 행동한다)
2. absent-actor — 죽었거나 그 자리에 없는 인물이 말하거나 행동한다
3. timeline — 시각·순서·알리바이가 서로 모순된다
4. attribute — 나이·이름·관계·물건·장소 등 속성이 앞뒤로 다르다
5. unpaid-setup — 분명히 심어 둔 복선(물건·약속·단서)이 끝까지 회수되지 않는다
6. unsetup-payoff — 앞에서 심지 않은 것이 결정적 해결에 쓰인다
7. secret — 비밀·거짓말이 드러나는 방식이 앞뒤로 맞지 않는다(이미 드러난 것을 모르는 척하거나, 드러나지 않았는데 아는 척한다)

## 규칙
- 오류마다 소설 원문을 **그대로** 인용한다(한 글자도 바꾸지 않는다). 모순이면 서로 부딪히는 두 곳을 모두 인용한다.
- 해석으로 설명이 되는 것(인물의 의도적 거짓말, 서술상 생략, 독자에게 감춘 것)은 오류가 아니다.
- 확신이 없으면 넣지 않는다.

## 수정 반영 일관성 (추가 항목)
두 소설은 연재 도중 아래 수정 요청을 한 번씩 받아 이미 반영한 완성본이다. 각 소설에서 수정 요청마다, 그 요청이 **영향을 받는 모든 화**에 일관되게 반영됐는지
화별로 표시한다. applied=true 는 그 화에 요청과 어긋나는 내용이 남아 있지 않다는 뜻이고, false 는 남아 있다는 뜻이다(note 에 원문 인용). 영향 없는 화는 넣지 않는다.

`

const judgeTail = `
## 출력 (JSON 만, 다른 글 없이)
{"X": [{"category": "...", "scene": 3, "quotes": ["...", "..."], "why": "..."}], "Y": [...],
 "revision": {"X": [{"revision": 1, "episodes": [{"ep": 1, "applied": true, "note": ""}]}], "Y": [...]}}

`

var sceneSep = "\n* * *\n"

// judgeMaxChars: above this many characters a single judge prompt is split into two halves of episodes
// (12 episodes x 2 arms x ~5000 characters is about 120k, so the full run normally fits in one call).
var judgeMaxChars = 150000

type claim struct {
	Category string   `json:"category"`
	Scene    any      `json:"scene"`
	Quotes   []string `json:"quotes"`
	Why      string   `json:"why"`
}

type revJudge struct {
	Revision int `json:"revision"`
	Episodes []struct {
		Ep      int    `json:"ep"`
		Applied bool   `json:"applied"`
		Note    string `json:"note"`
	} `json:"episodes"`
}

func norm(s string) string {
	s = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'").Replace(s)
	return strings.Join(strings.Fields(s), "")
}

// judgeOutcome: Status "ok" or "unknown: <why>". resid[arm][revision] = episodes marked applied=false; a revision
// the judge did not mark for an arm has no entry, which reads "unknown", never 0.
type judgeOutcome struct {
	Status, stop string
	resid        map[string]map[int]int
}

func (j *judgeOutcome) residual(arm string, rev int) any {
	if j == nil || j.resid == nil {
		return "unknown"
	}
	if n, ok := j.resid[arm][rev]; ok {
		return n
	}
	return "unknown"
}

// runJudge builds the blind prompt for episodes 1..cut of both arms (X/Y at random, kept in judge/mapping.txt),
// calls once (twice when the prompt is too large: eps 1..mid, mid+1..cut), and writes judge/quotes.md and
// judge/revision.md. It never fails the run: on any problem the raw output stays in judge/ and the marks are "unknown".
func runJudge(ctx context.Context, t *tracker) judgeOutcome {
	c := t.c
	fail := func(f string, a ...any) judgeOutcome {
		return judgeOutcome{Status: "unknown: " + fmt.Sprintf(f, a...)}
	}
	if t.cut == 0 {
		return fail("no episode completed by both arms")
	}
	dir := filepath.Join(c.out, "judge")
	ea, eb := make([]string, t.cut+1), make([]string, t.cut+1)
	for e := 1; e <= t.cut; e++ {
		ea[e], eb[e] = aEpText(c, e), t.bEpText(e)
	}
	x, y := "A", "B"
	if rand.IntN(2) == 1 {
		x, y = "B", "A"
	}
	if err := write(filepath.Join(dir, "mapping.txt"), "X="+x+"\nY="+y); err != nil {
		return fail("%v", err)
	}
	armOf := map[string]string{"X": x, "Y": y}
	var reqs strings.Builder
	for k, after := range c.revAfter {
		if t.revDone[k] {
			fmt.Fprintf(&reqs, "수정 %d (%d화 뒤에 받음): %s\n", k+1, after, revReqs[k])
		}
	}
	text := func(arm string, lo, hi int) string {
		src := ea
		if arm == "B" {
			src = eb
		}
		var b strings.Builder
		for e := lo; e <= hi; e++ {
			fmt.Fprintf(&b, "### 화 %d\n\n%s\n\n", e, src[e])
		}
		return b.String()
	}
	build := func(lo, hi int) string {
		part := ""
		if lo != 1 || hi != t.cut {
			part = fmt.Sprintf("\n(이 요청은 전체 %d화 중 %d~%d화만 담는다. 수정 반영 표시는 이 화들에 대해서만 한다.)\n", t.cut, lo, hi)
		}
		return judgeHead + reqs.String() + part + judgeTail + "\n## 소설 X\n\n" + text(armOf["X"], lo, hi) + "\n## 소설 Y\n\n" + text(armOf["Y"], lo, hi)
	}
	ranges := [][2]int{{1, t.cut}}
	if utf8.RuneCountInString(build(1, t.cut)) > judgeMaxChars && t.cut >= 2 {
		ranges = [][2]int{{1, t.cut / 2}, {t.cut/2 + 1, t.cut}}
	}
	m, f, err := meter(c.judge, dir)
	if err != nil {
		return fail("%v", err)
	}
	defer f.Close()
	out := judgeOutcome{Status: "ok", resid: map[string]map[int]int{"A": {}, "B": {}}}
	var q, r strings.Builder
	q.WriteString("# Judge findings vs. quote check (automatic)\n\nOnly checks whether the quotes appear verbatim in the source text. Whether a finding is right is decided by a person.\n")
	r.WriteString("# Revision consistency (marked by the judge, unverified)\n")
	src := map[string]string{"A": norm(text("A", 1, t.cut)), "B": norm(text("B", 1, t.cut))}
	for i, rg := range ranges {
		prompt := build(rg[0], rg[1])
		if err := write(filepath.Join(dir, fmt.Sprintf("prompt%d.txt", i+1)), prompt); err != nil {
			return fail("%v", err)
		}
		raw, err := call(ctx, m, fmt.Sprintf("judge-%d", i+1), prompt)
		if err != nil {
			return fail("%v", err)
		}
		if err := write(filepath.Join(dir, fmt.Sprintf("run%d.txt", i+1)), raw); err != nil {
			return fail("%v", err)
		}
		j := strings.TrimSpace(raw)
		if a, b := strings.Index(j, "{"), strings.LastIndex(j, "}"); a >= 0 && b > a {
			j = j[a : b+1]
		}
		var keys map[string]json.RawMessage
		var got struct {
			X, Y     []claim
			Revision map[string][]revJudge
		}
		if err := json.Unmarshal([]byte(j), &keys); err != nil {
			return fail("judge output %d: %v (raw in judge/run%d.txt)", i+1, err, i+1)
		}
		if keys["X"] == nil || keys["Y"] == nil {
			return fail("judge output %d lacks X or Y (raw in judge/run%d.txt)", i+1, i+1)
		}
		if err := json.Unmarshal([]byte(j), &got); err != nil {
			return fail("judge output %d: %v (raw in judge/run%d.txt)", i+1, err, i+1)
		}
		for _, xy := range []string{"X", "Y"} {
			a, cl := armOf[xy], got.X
			if xy == "Y" {
				cl = got.Y
			}
			fmt.Fprintf(&q, "\n## Part %d: %s (= %s) — %d findings\n", i+1, xy, a, len(cl))
			for n, c := range cl {
				fmt.Fprintf(&q, "\n%s%d. [%s] episode/scene %v — %s\n", a, n+1, c.Category, c.Scene, c.Why)
				for _, qu := range c.Quotes {
					mark := "absent"
					if strings.Contains(src[a], norm(qu)) {
						mark = "present"
					}
					fmt.Fprintf(&q, "   - (%s) %s\n", mark, qu)
				}
			}
			fmt.Fprintf(&r, "\n## Part %d: %s (= %s)\n", i+1, xy, a)
			for _, rv := range got.Revision[xy] {
				bad := 0
				for _, e := range rv.Episodes {
					if !e.Applied {
						bad++
						fmt.Fprintf(&r, "- revision %d, episode %d: not applied — %s\n", rv.Revision, e.Ep, e.Note)
					}
				}
				out.resid[a][rv.Revision] += bad
				fmt.Fprintf(&r, "- revision %d: %d episodes marked, %d residual mismatches\n", rv.Revision, len(rv.Episodes), bad)
			}
		}
	}
	if err := write(filepath.Join(dir, "revision.md"), r.String()); err != nil {
		return fail("%v", err)
	}
	if err := write(filepath.Join(dir, "quotes.md"), q.String()); err != nil {
		return fail("%v", err)
	}
	return out
}

// fakeJudge selects a judge failure for tests: "garbage", "noY" (no Y key), "norev" (no revision marks for Y).
var fakeJudge string

// fakeReplanBad makes the fake replan answer unparsable.
var fakeReplanBad bool

// fakeDiegCalls is how many metered calls one fake dieg invocation makes.
var fakeDiegCalls = 3

// fakeSkip names a scene (e.g. "e2b") the fake dieg does not render, like a scene that failed verification.
var fakeSkip string

// fakeDiegArgs records every fake dieg command line (for the test).
var fakeDiegArgs [][]string

// fake answers every model call for -dry and the test, keyed on the prompt's title.
type fake struct{ eps int }

func (f fake) Complete(_ context.Context, _, user string) (render.Result, error) {
	t := "가짜 화. 종이 울렸다.\n* * *\n둘째 장면이다."
	switch {
	case strings.Contains(user, "화별 방향 재생성"):
		t = ""
		for i := 1; i <= f.eps; i++ {
			t += fmt.Sprintf("%d화: 재계획 방향 %d\n", i, i)
		}
		if fakeReplanBad {
			t = "방향을 못 정하겠다"
		}
	case strings.Contains(user, "시리즈 계획"):
		t = ""
		for i := 1; i <= f.eps; i++ {
			t += fmt.Sprintf("%d화: 가짜 방향 %d\n", i, i)
		}
	case strings.Contains(user, "고르기"):
		t = "[1,2]"
	case strings.Contains(user, "누적 요약 갱신"):
		t = "가짜 요약"
	case strings.Contains(user, "연속성 오류 판정"):
		y := `[{"revision": 1, "episodes": [{"ep": 1, "applied": false, "note": "x"}, {"ep": 2, "applied": false, "note": "y"}]}]`
		if fakeJudge == "norev" {
			y = `[]`
		}
		t = "```json\n" + `{"X": [{"category": "timeline", "scene": 1, "quotes": ["종이 울렸다.", "지어낸 인용"], "why": "가짜"}], "Y": [],
"revision": {"X": [{"revision": 1, "episodes": [{"ep": 1, "applied": true}, {"ep": 2, "applied": false, "note": "남음"}]}], "Y": ` + y + `}}` + "\n```"
		switch fakeJudge {
		case "garbage":
			t = "판정 불가"
		case "noY":
			t = `{"X": []}`
		}
	}
	return render.Result{Text: t, StopReason: "end_turn", Usage: render.Usage{Input: 2, Output: 10, CacheRead: 100, CacheWrite: 50}}, nil
}

// fakeDieg stands in for the dieg CLI: 3 metered calls per invocation into b/calls.jsonl and scene files in
// b/novel/ that write creates (e1a, e1b), continue adds (e<N>a, e<N>b) and revise edits (e1a rewritten).
func fakeDieg(ctx context.Context, out string, args []string) error {
	fakeDiegArgs = append(fakeDiegArgs, args)
	dir := filepath.Join(out, "b")
	m, f, err := meter(fake{}, dir)
	if err != nil {
		return err
	}
	defer f.Close()
	labels := []string{"code-r1", "render-s1-a1", "extract-s1-a1"}
	for i := 0; i < fakeDiegCalls; i++ {
		l := labels[min(i, 2)]
		if i > 2 {
			l = labels[1+i%2]
		}
		if _, err := m.Label(l).Complete(ctx, "", ""); err != nil {
			return err
		}
	}
	put := func(id, s string) error {
		if id == fakeSkip {
			return nil
		}
		return write(filepath.Join(dir, "novel", id+".ko-haera.md"), s)
	}
	switch args[0] {
	case "write", "continue":
		ep := "1"
		if args[0] == "continue" {
			ep = args[3] // continue <dir> --ep N --brief X
			if err := write(filepath.Join(dir, "argv", ep+".txt"), args[5]); err != nil {
				return err
			}
		}
		if err := put("e"+ep+"a", "가짜 장면이다. 종이 울렸다. "+strings.Repeat("가", 10*len(ep))); err != nil {
			return err
		}
		return put("e"+ep+"b", "가짜 장면이다. "+ep)
	case "revise":
		return put("e1a", "수정된 장면이다.")
	}
	return nil
}
