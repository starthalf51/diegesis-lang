package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// PlanPrompt asks for one line of direction per episode. Its result is the brief both arms get; the call is
// recorded under plan/ and charged to neither arm.
func PlanPrompt(idea string, n int) string {
	return fmt.Sprintf("# 과제: 시리즈 계획\n\n아래 아이디어로 한국어 웹소설을 총 %d화(화당 장면 %d개)로 연재한다. 화마다 그 화의 방향을 한 줄(60자 안팎)로 정한다.\n"+
		"비밀, 일부 인물만 믿는 거짓말, 누가 언제 무엇을 아는가가 화를 거치며 쌓이고 마지막 화에서 회수되도록 짠다.\n\n## 아이디어\n%s\n\n"+
		"## 출력 (다른 글 없이, 정확히 %d줄)\n1화: ...\n2화: ...\n", n, perEp, strings.TrimSpace(idea), n)
}

var briefRE = regexp.MustCompile(`(?m)^\s*(\d+)\s*화\s*[:：.]\s*(.+?)\s*$`)

// parseBriefs returns briefs[1..n]; any missing episode is an error (a half plan would silently skew one arm).
func parseBriefs(text string, n int) ([]string, error) {
	b := make([]string, n+1)
	for _, m := range briefRE.FindAllStringSubmatch(text, -1) {
		if i, _ := strconv.Atoi(m[1]); i >= 1 && i <= n && b[i] == "" {
			b[i] = m[2]
		}
	}
	for i := 1; i <= n; i++ {
		if b[i] == "" {
			return nil, fmt.Errorf("plan: no brief for episode %d", i)
		}
	}
	return b, nil
}

func runPlan(ctx context.Context, t *tracker) ([]string, error) {
	c := t.c
	if err := t.check(); err != nil {
		return nil, err
	}
	m, f, err := meter(c.a, filepath.Join(c.out, "plan"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	text, err := call(ctx, m, "plan", PlanPrompt(c.idea, c.eps))
	if err != nil {
		return nil, err
	}
	if err := write(filepath.Join(c.out, "plan", "plan.md"), text); err != nil {
		return nil, err
	}
	return parseBriefs(text, c.eps)
}

const aHead = "## 연재 규칙\n- 화당 장면 %d개. 장면 사이는 한 줄 `* * *` 로 나눈다. 해라체 서술, 화당 5000자 안팎. 제목·머리말·설명 없이 산문만 출력한다.\n" +
	"- 인물의 이름·나이·관계·물건·시각, 누가 무엇을 아는가는 처음 정한 대로 지킨다. 요약의 `확정된 수정` 은 개요보다 우선한다.\n"

func OutlinePrompt(idea string, briefs []string) string {
	var b strings.Builder
	b.WriteString("# 과제: 연재 소설 — 개요\n\n너는 이 소설을 한 화씩 직접 쓸 것이다. 먼저 짧은 개요를 쓴다. 이후 화를 쓸 때마다 이 개요, 누적 요약, 직전 화만 보게 된다.\n\n")
	fmt.Fprintf(&b, aHead, perEp)
	b.WriteString("\n## 아이디어\n" + strings.TrimSpace(idea) + "\n\n## 화별 방향\n")
	for i := 1; i < len(briefs); i++ {
		fmt.Fprintf(&b, "%d화: %s\n", i, briefs[i])
	}
	b.WriteString("\n## 개요에 담을 것 (1500자 안팎, 마크다운)\n- 등장인물: 이름, 나이, 관계, 비밀\n- 사건의 진상과 시각표\n- 심어 두고 회수할 물건·단서\n- 화마다 두세 줄\n\n개요만 출력한다.\n")
	return b.String()
}

func EpisodePrompt(idea, outline, summary, prev, brief string, ep, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 과제: 연재 소설 %d/%d화를 쓴다\n\n", ep, n)
	fmt.Fprintf(&b, aHead, perEp)
	b.WriteString("\n## 아이디어\n" + strings.TrimSpace(idea) + "\n\n## 개요\n\n" + strings.TrimSpace(outline) + "\n")
	if ep > 1 {
		b.WriteString("\n## 누적 요약\n\n" + strings.TrimSpace(summary) + "\n")
		fmt.Fprintf(&b, "\n## 직전 화 (%d화)\n\n%s\n", ep-1, strings.TrimSpace(prev))
	}
	fmt.Fprintf(&b, "\n## 지시\n%d화를 쓴다. 이 화의 방향: %s\n", ep, brief)
	return b.String()
}

func SummaryPrompt(summary, request string, eps map[int]string) string {
	var b strings.Builder
	b.WriteString("# 과제: 연재 누적 요약 갱신\n\n아래 새로 쓴 화(들)를 반영해 누적 요약을 갱신한다. 요약은 다음 화를 쓰는 데 필요한 것만 담는다: 인물(이름·나이·관계), 시각표, " +
		"심은 물건·단서, 누가 무엇을 아는가(믿는 거짓말 포함, 독자에게 아직 숨긴 것은 표시), 지금까지의 줄거리. 1500자 안팎.\n")
	if request != "" {
		b.WriteString("요약 맨 위 `확정된 수정` 항목에 아래 수정 요청을 한 줄로 누적 기록한다(이미 있으면 유지).\n\n## 수정 요청\n" + request + "\n")
	}
	b.WriteString("\n## 기존 요약\n\n" + strings.TrimSpace(summary) + "\n")
	var ks []int
	for k := range eps {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	for _, k := range ks {
		fmt.Fprintf(&b, "\n## 새로 쓴 %d화\n\n%s\n", k, strings.TrimSpace(eps[k]))
	}
	b.WriteString("\n갱신된 요약만 출력한다.\n")
	return b.String()
}

func PickPrompt(request, summary string, upto int) string {
	return fmt.Sprintf("# 과제: 수정 요청이 영향을 주는 화 고르기\n\n지금까지 1~%d화를 썼다. 아래 수정 요청을 일관되게 반영하려면 다시 써야 하는 화를 모두 고른다. 영향이 없는 화는 넣지 않는다.\n\n"+
		"## 수정 요청\n%s\n\n## 누적 요약\n\n%s\n\n## 출력 (다른 글 없이)\n다시 쓸 화 번호의 JSON 배열. 예: [1,3]\n", upto, request, strings.TrimSpace(summary))
}

func RewritePrompt(idea, outline, summary, request, old, prev, brief string, ep int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 과제: 연재 소설 %d화를 수정 요청에 맞게 다시 쓴다\n\n", ep)
	fmt.Fprintf(&b, aHead, perEp)
	b.WriteString("\n## 수정 요청 (개요·요약과 충돌하면 이것이 우선)\n" + request + "\n\n## 개요\n\n" + strings.TrimSpace(outline) + "\n\n## 누적 요약\n\n" + strings.TrimSpace(summary) + "\n")
	if prev != "" {
		fmt.Fprintf(&b, "\n## 직전 화 (%d화, 수정 반영본)\n\n%s\n", ep-1, strings.TrimSpace(prev))
	}
	fmt.Fprintf(&b, "\n## 기존 %d화\n\n%s\n\n## 지시\n%d화를 다시 쓴다. 이 화의 방향: %s. 수정과 무관한 사건·대사는 가능한 한 그대로 둔다.\n", ep, strings.TrimSpace(old), ep, brief)
	return b.String()
}

func epPath(out string, ep int) string {
	return filepath.Join(out, "a", "eps", fmt.Sprintf("e%02d.md", ep))
}

func readEp(out string, ep int) string {
	b, _ := os.ReadFile(epPath(out, ep))
	return string(b)
}

var numRE = regexp.MustCompile(`\d+`)

// runA: outline, then per episode 2 calls (write, summary update); after episode revAfter[k], a revision =
// 1 pick call + one rewrite per picked episode + 1 summary refresh.
func runA(ctx context.Context, t *tracker, briefs []string) error {
	c := t.c
	m, f, err := meter(c.a, filepath.Join(c.out, "a"))
	if err != nil {
		return err
	}
	defer f.Close()
	step := func(label, prompt string) (string, error) {
		if err := t.check(); err != nil {
			return "", err
		}
		if err := write(filepath.Join(c.out, "a", "prompts", label+".txt"), prompt); err != nil {
			return "", err
		}
		return call(ctx, m, label, prompt)
	}
	outline, err := step("outline", OutlinePrompt(c.idea, briefs))
	if err != nil {
		return err
	}
	if err := write(filepath.Join(c.out, "a", "outline.md"), outline); err != nil {
		return err
	}
	t.record("A", "outline")
	summary := ""
	for e := 1; e <= c.eps; e++ {
		prev := ""
		if e > 1 {
			prev = readEp(c.out, e-1)
		}
		text, err := step(fmt.Sprintf("ep-%d", e), EpisodePrompt(c.idea, outline, summary, prev, briefs[e], e, c.eps))
		if err != nil {
			return err
		}
		if err := write(epPath(c.out, e), text); err != nil {
			return err
		}
		if summary, err = step(fmt.Sprintf("sum-%d", e), SummaryPrompt(summary, "", map[int]string{e: text})); err != nil {
			return err
		}
		if err := write(filepath.Join(c.out, "a", "summary.md"), summary); err != nil {
			return err
		}
		t.record("A", fmt.Sprintf("ep%d", e))
		k := c.revisionAfter(e)
		if k < 0 {
			continue
		}
		req, n := revReqs[k], k+1
		err = t.revStep("A", n, func() error {
			picked, err := step(fmt.Sprintf("rev%d-pick", n), PickPrompt(req, summary, e))
			if err != nil {
				return err
			}
			var eps []int
			seen := map[int]bool{}
			for _, s := range numRE.FindAllString(picked, -1) {
				if i, _ := strconv.Atoi(s); i >= 1 && i <= e && !seen[i] {
					seen[i] = true
					eps = append(eps, i)
				}
			}
			if len(eps) == 0 {
				return fmt.Errorf("rev%d-pick: no episode in %q", n, picked)
			}
			sort.Ints(eps)
			redone := map[int]string{}
			for _, i := range eps {
				prev := ""
				if i > 1 {
					prev = readEp(c.out, i-1)
				}
				text, err := step(fmt.Sprintf("rev%d-ep-%d", n, i), RewritePrompt(c.idea, outline, summary, req, readEp(c.out, i), prev, briefs[i], i))
				if err != nil {
					return err
				}
				if err := write(epPath(c.out, i), text); err != nil {
					return err
				}
				redone[i] = text
			}
			if summary, err = step(fmt.Sprintf("rev%d-sum", n), SummaryPrompt(summary, req, redone)); err != nil {
				return err
			}
			return write(filepath.Join(c.out, "a", "summary.md"), summary)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// runB drives the dieg CLI: write (ep1), continue (ep2..), revise at the same points as A. The idea file carries
// ep1's brief; later briefs go through --brief. dieg appends to b/calls.jsonl itself.
func runB(ctx context.Context, t *tracker, briefs []string) error {
	c := t.c
	b := filepath.Join(c.out, "b")
	if err := os.MkdirAll(c.out, 0o755); err != nil {
		return err
	}
	idea := filepath.Join(c.out, "b-idea.txt")
	if err := write(idea, strings.TrimSpace(c.idea)+fmt.Sprintf("\n\n## 연재 설정\n총 %d화, 화당 장면 %d개. 지금은 1화만 쓴다.\n1화 방향: %s", c.eps, perEp, briefs[1])); err != nil {
		return err
	}
	sc := strconv.Itoa(perEp)
	for e := 1; e <= c.eps; e++ {
		if err := t.check(); err != nil {
			return err
		}
		args := []string{"write", idea, "-o", b, "--scenes", sc}
		if e > 1 {
			args = []string{"continue", b, "--ep", strconv.Itoa(e), "--brief", briefs[e], "--scenes", sc}
		}
		if err := runDieg(ctx, c.out, args); err != nil {
			return fmt.Errorf("dieg %s (ep %d): %w (see %s)", args[0], e, err, filepath.Join(c.out, "b.log"))
		}
		t.record("B", fmt.Sprintf("ep%d", e))
		if k := c.revisionAfter(e); k >= 0 {
			err := t.revStep("B", k+1, func() error {
				if err := t.check(); err != nil {
					return err
				}
				// no --scenes: every scene whose render request changed must be re-rendered
				return runDieg(ctx, c.out, []string{"revise", b, revReqs[k]})
			})
			if err != nil {
				return fmt.Errorf("dieg revise (rev %d): %w (see %s)", k+1, err, filepath.Join(c.out, "b.log"))
			}
		}
	}
	return nil
}
