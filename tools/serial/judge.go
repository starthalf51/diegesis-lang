package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"diegesis/internal/render"
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

// novelA joins a/eps/e01..n.md under "### 화 e" headers.
func novelA(c cfg) (string, error) {
	var b strings.Builder
	for e := 1; e <= c.eps; e++ {
		s, err := os.ReadFile(epPath(c.out, e))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "### 화 %d\n\n%s\n\n", e, strings.TrimSpace(string(s)))
	}
	return b.String(), nil
}

// novelB regroups b/novel.md (one "## <title>" block per scene, discourse order) into episodes of perEp scenes
// separated like A's, with the titles dropped so they do not tell the arms apart. A failed scene shifts the
// grouping; that is B's real output and stays in.
func novelB(c cfg) (string, error) {
	s, err := os.ReadFile(filepath.Join(c.out, "b", "novel.md"))
	if err != nil {
		return "", err
	}
	var scenes []string
	for _, l := range strings.Split(string(s), "\n") {
		if strings.HasPrefix(l, "## ") {
			scenes = append(scenes, "")
		} else if len(scenes) > 0 {
			scenes[len(scenes)-1] += l + "\n"
		}
	}
	var b strings.Builder
	for i, sc := range scenes {
		if i%perEp == 0 {
			fmt.Fprintf(&b, "### 화 %d\n\n", i/perEp+1)
		} else {
			b.WriteString(sceneSep)
		}
		b.WriteString(strings.TrimSpace(sc) + "\n")
		if i%perEp == perEp-1 || i == len(scenes)-1 {
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

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

// judgeResult: Residual[arm][revision] = episodes marked applied=false.
type judgeResult struct{ Residual map[string]map[int]int }

func norm(s string) string {
	s = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'").Replace(s)
	return strings.Join(strings.Fields(s), "")
}

// runJudge builds the blind prompt (X/Y at random, kept in judge/mapping.txt), calls once, and writes
// judge/quotes.md (claims with quote found/not) and judge/revision.md (per-episode revision marks).
func runJudge(ctx context.Context, t *tracker) (judgeResult, error) {
	c := t.c
	res := judgeResult{map[string]map[int]int{"A": {}, "B": {}}}
	dir := filepath.Join(c.out, "judge")
	na, err := novelA(c)
	if err != nil {
		return res, err
	}
	nb, err := novelB(c)
	if err != nil {
		return res, err
	}
	x := "A"
	if rand.IntN(2) == 1 {
		x = "B"
	}
	nx, ny, y := na, nb, "B"
	if x == "B" {
		nx, ny, y = nb, na, "A"
	}
	var reqs strings.Builder
	for k, after := range c.revAfter {
		fmt.Fprintf(&reqs, "수정 %d (%d화 뒤에 받음): %s\n", k+1, after, revReqs[k])
	}
	prompt := judgeHead + reqs.String() + judgeTail + "\n## 소설 X\n\n" + nx + "\n## 소설 Y\n\n" + ny
	if err := write(filepath.Join(dir, "mapping.txt"), "X="+x+"\nY="+y); err != nil {
		return res, err
	}
	if err := write(filepath.Join(dir, "prompt.txt"), prompt); err != nil {
		return res, err
	}
	if err := t.check(); err != nil {
		return res, err
	}
	m, f, err := meter(c.judge, dir)
	if err != nil {
		return res, err
	}
	defer f.Close()
	text, err := call(ctx, m, "judge", prompt)
	if err != nil {
		return res, err
	}
	if err := write(filepath.Join(dir, "run1.txt"), text); err != nil {
		return res, err
	}
	j := strings.TrimSpace(text)
	if i, k := strings.Index(j, "{"), strings.LastIndex(j, "}"); i >= 0 && k > i {
		j = j[i : k+1]
	}
	var got struct {
		X, Y     []claim
		Revision map[string][]revJudge
	}
	if err := json.Unmarshal([]byte(j), &got); err != nil {
		return res, fmt.Errorf("judge output: %w", err)
	}
	armOf := map[string]string{"X": x, "Y": y}
	src := map[string]string{"A": norm(na), "B": norm(nb)}
	var q, r strings.Builder
	q.WriteString("# 판정 지적과 인용 대조 (자동)\n\n인용이 원문에 그대로 있는지만 본다. 지적이 맞는지는 사람이 가린다.\n")
	r.WriteString("# 수정 반영 일관성 (판정자 표시, 미검증)\n")
	for _, xy := range []string{"X", "Y"} {
		a, cl := armOf[xy], got.X
		if xy == "Y" {
			cl = got.Y
		}
		fmt.Fprintf(&q, "\n## %s (= %s) — %d건\n", xy, a, len(cl))
		for i, c := range cl {
			fmt.Fprintf(&q, "\n%s%d. [%s] 화/장면 %v — %s\n", a, i+1, c.Category, c.Scene, c.Why)
			for _, qu := range c.Quotes {
				mark := "없음"
				if strings.Contains(src[a], norm(qu)) {
					mark = "있음"
				}
				fmt.Fprintf(&q, "   - (%s) %s\n", mark, qu)
			}
		}
		fmt.Fprintf(&r, "\n## %s (= %s)\n", xy, a)
		for _, rv := range got.Revision[xy] {
			var bad []string
			for _, e := range rv.Episodes {
				if !e.Applied {
					bad = append(bad, strconv.Itoa(e.Ep))
					fmt.Fprintf(&r, "- 수정 %d, %d화: 미반영 — %s\n", rv.Revision, e.Ep, e.Note)
				}
			}
			res.Residual[a][rv.Revision] = len(bad)
			fmt.Fprintf(&r, "- 수정 %d: 표시된 화 %d개, 잔여 불일치 %d개 [%s]\n", rv.Revision, len(rv.Episodes), len(bad), strings.Join(bad, ","))
		}
	}
	if err := write(filepath.Join(dir, "revision.md"), r.String()); err != nil {
		return res, err
	}
	return res, write(filepath.Join(dir, "quotes.md"), q.String())
}

// fake answers every model call for -dry and the test, keyed on the prompt's title.
type fake struct{ eps int }

func (f fake) Complete(_ context.Context, _, user string) (render.Result, error) {
	t := "가짜 화. 종이 울렸다.\n* * *\n둘째 장면이다."
	switch {
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
		t = "```json\n" + `{"X": [{"category": "timeline", "scene": 1, "quotes": ["종이 울렸다.", "지어낸 인용"], "why": "가짜"}], "Y": [],
"revision": {"X": [{"revision": 1, "episodes": [{"ep": 1, "applied": true}, {"ep": 2, "applied": false, "note": "남음"}]}], "Y": [{"revision": 1, "episodes": [{"ep": 1, "applied": false, "note": "x"}, {"ep": 2, "applied": false, "note": "y"}]}]}}` + "\n```"
	}
	return render.Result{Text: t, StopReason: "end_turn", Usage: render.Usage{Input: 2, Output: 10, CacheRead: 100, CacheWrite: 50}}, nil
}

// fakeDieg stands in for the dieg CLI: 3 metered calls per invocation into b/calls.jsonl and a novel.md that
// write creates (2 scenes), continue extends (2 scenes) and revise edits (scene 1 rewritten).
func fakeDieg(ctx context.Context, out string, args []string) error {
	dir := filepath.Join(out, "b")
	m, f, err := meter(fake{}, dir)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range []string{"code-r1", "render-s1-a1", "extract-s1-a1"} {
		if _, err := m.Label(l).Complete(ctx, "", ""); err != nil {
			return err
		}
	}
	p := filepath.Join(dir, "novel.md")
	old, _ := os.ReadFile(p)
	s := string(old)
	switch args[0] {
	case "write", "continue":
		s += "## 첫 장면\n\n가짜 장면이다. 종이 울렸다.\n\n## 둘째 장면\n\n가짜 장면이다.\n\n"
	case "revise":
		s = strings.Replace(s, "가짜 장면이다. 종이 울렸다.", "수정된 장면이다.", 1)
	}
	return os.WriteFile(p, []byte(s), 0o644)
}
