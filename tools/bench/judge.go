package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// judgeHead is the checklist from examples/idea-to-novel/prompts/judge-build.sh, verbatim.
const judgeHead = `# 과제: 두 소설의 연속성 오류 판정

같은 아이디어로 쓴 한국어 소설 두 편(X, Y)이 있다. 각 소설 안에서 **객관적인 연속성 오류**만 찾는다.
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

## 출력 (JSON 만, 다른 글 없이)
{"X": [{"category": "...", "scene": 3, "quotes": ["...", "..."], "why": "..."}], "Y": [...]}

`

// novelA joins a/scenes/s1..n.md under "### 장면 i" headers, as judge-build.sh does.
func novelA(out string, n int) (string, error) {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		s, err := os.ReadFile(filepath.Join(out, "a", "scenes", fmt.Sprintf("s%d.md", i)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "### 장면 %d\n\n%s\n\n", i, strings.TrimSpace(string(s)))
	}
	return b.String(), nil
}

// novelB re-heads b/novel.md (discourse order, "## <title>" per scene) as "### 장면 i" so titles do not tell the arms apart.
func novelB(out string) (string, error) {
	s, err := os.ReadFile(filepath.Join(out, "b", "novel.md"))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	i := 0
	for _, l := range strings.Split(string(s), "\n") {
		if strings.HasPrefix(l, "## ") {
			i++
			fmt.Fprintf(&b, "### 장면 %d", i)
		} else {
			b.WriteString(l)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

type claim struct {
	Category string   `json:"category"`
	Scene    any      `json:"scene"`
	Quotes   []string `json:"quotes"`
	Why      string   `json:"why"`
}

// norm drops whitespace and unifies quote marks so a verbatim quote matches across line wraps.
func norm(s string) string {
	s = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'").Replace(s)
	return strings.Join(strings.Fields(s), "")
}

// RunJudge builds the blind prompt (X/Y at random, kept in judge/mapping.txt), calls once (reusing judge/run1.txt
// if present), and writes judge/quotes.md: every claim under its real arm with each quote marked found or not.
func RunJudge(ctx context.Context, c render.Completer, n int, out string) error {
	dir := filepath.Join(out, "judge")
	na, err := novelA(out, n)
	if err != nil {
		return err
	}
	nb, err := novelB(out)
	if err != nil {
		return err
	}
	raw := filepath.Join(dir, "run1.txt")
	text := ""
	if b, err := os.ReadFile(raw); err == nil {
		text = string(b)
	} else {
		x := "A"
		if rand.IntN(2) == 1 {
			x = "B"
		}
		nx, ny := na, nb
		if x == "B" {
			nx, ny = nb, na
		}
		prompt := judgeHead + "\n## 소설 X\n\n" + nx + "\n## 소설 Y\n\n" + ny
		y := map[string]string{"A": "B", "B": "A"}[x]
		if err := write(filepath.Join(dir, "mapping.txt"), "X="+x+"\nY="+y); err != nil {
			return err
		}
		if err := write(filepath.Join(dir, "prompt.txt"), prompt); err != nil {
			return err
		}
		m, f, err := meter(c, dir)
		if err != nil {
			return err
		}
		defer f.Close()
		if text, err = call(ctx, m, "judge", prompt); err != nil {
			return err
		}
		if err := write(raw, text); err != nil {
			return err
		}
	}
	mp, err := os.ReadFile(filepath.Join(dir, "mapping.txt"))
	if err != nil {
		return err
	}
	arm := map[string]string{}
	for _, l := range strings.Fields(string(mp)) {
		if k, v, ok := strings.Cut(l, "="); ok {
			arm[k] = v
		}
	}
	j := strings.TrimSpace(text)
	if i, k := strings.Index(j, "{"), strings.LastIndex(j, "}"); i >= 0 && k > i {
		j = j[i : k+1]
	}
	var got map[string][]claim
	if err := json.Unmarshal([]byte(j), &got); err != nil {
		return fmt.Errorf("judge output: %w", err)
	}
	src := map[string]string{"A": norm(na), "B": norm(nb)}
	var b strings.Builder
	b.WriteString("# 판정 지적과 인용 대조 (자동)\n\n인용이 원문에 그대로 있는지만 본다. 지적이 맞는지는 verification.md 에서 사람이 가린다.\n")
	for _, xy := range []string{"X", "Y"} {
		a := arm[xy]
		fmt.Fprintf(&b, "\n## %s (= %s) — %d건\n", xy, a, len(got[xy]))
		for i, cl := range got[xy] {
			fmt.Fprintf(&b, "\n%s%d. [%s] 장면 %v — %s\n", a, i+1, cl.Category, cl.Scene, cl.Why)
			for _, q := range cl.Quotes {
				mark := "없음"
				if strings.Contains(src[a], norm(q)) {
					mark = "있음"
				}
				fmt.Fprintf(&b, "   - (%s) %s\n", mark, q)
			}
		}
	}
	return write(filepath.Join(dir, "quotes.md"), b.String())
}

// fake answers every call for -dry and the test: a judge prompt gets one claim against X, anything else a stub.
type fake struct{}

func (fake) Complete(_ context.Context, _, user string) (render.Result, error) {
	t := "가짜 장면이다. 종이 울렸다."
	if strings.Contains(user, "연속성 오류 판정") {
		t = "```json\n" + `{"X": [{"category": "timeline", "scene": 1, "quotes": ["종이 울렸다.", "지어낸 인용"], "why": "가짜"}], "Y": []}` + "\n```"
	}
	return render.Result{Text: t, StopReason: "end_turn", Usage: render.Usage{Input: 2, Output: 10, CacheRead: 100, CacheWrite: 50}}, nil
}

// fakeB stands in for `dieg write`: one code round and two scenes of one render + one extract each.
func fakeB(ctx context.Context, _, out string, _ []string) error {
	dir := filepath.Join(out, "b")
	m, f, err := meter(fake{}, dir)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range []string{"code-r1", "render-s1-a1", "extract-s1-a1", "render-s2-a1", "extract-s2-a1"} {
		if _, err := m.Label(l).Complete(ctx, "", ""); err != nil {
			return err
		}
	}
	if err := write(filepath.Join(dir, "novel.md"), "## 첫 장면\n\n가짜 장면이다. 종이 울렸다.\n\n## 둘째 장면\n\n가짜 장면이다."); err != nil {
		return err
	}
	return write(filepath.Join(out, "b.log"), "write: rounds=1 clean=true scenes=2 failed=1[s2] calls=5 tokens in=10 out=50 cache_read=500 cache_write=250 → b")
}
