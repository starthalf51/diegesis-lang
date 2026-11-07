package main

import (
	"context"
	"diegesis/internal/render"
	"fmt"
	"io"
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

// parseBriefs returns briefs[0..n] with from..n required; any missing episode is an error (a half plan would
// silently skew one arm).
func parseBriefs(text string, from, n int) ([]string, error) {
	b := make([]string, n+1)
	for _, m := range briefRE.FindAllStringSubmatch(text, -1) {
		if i, _ := strconv.Atoi(m[1]); i >= 1 && i <= n && b[i] == "" {
			b[i] = m[2]
		}
	}
	for i := from; i <= n; i++ {
		if b[i] == "" {
			return nil, fmt.Errorf("plan: no brief for episode %d", i)
		}
	}
	return b, nil
}

func runPlan(ctx context.Context, t *tracker) ([]string, error) {
	c := t.c
	if err := t.check(judgeReserve + 1); err != nil {
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
	return parseBriefs(text, 1, c.eps)
}

// ReplanPrompt asks for new directions for episodes from..n after revision request req; briefs are the current ones.
func ReplanPrompt(idea string, briefs []string, from int, req string) string {
	var b strings.Builder
	n := len(briefs) - 1
	fmt.Fprintf(&b, "# 과제: 화별 방향 재생성\n\n한국어 웹소설 총 %d화 연재 중 %d화까지 썼고, 아래 수정 요청이 확정되었다. 남은 %d~%d화의 방향을 수정 요청에 맞게 다시 정한다. 화마다 한 줄(60자 안팎).\n\n", n, from-1, from, n)
	b.WriteString("## 아이디어\n" + strings.TrimSpace(idea) + "\n\n## 수정 요청\n" + req + "\n\n## 지금의 화별 방향\n")
	for i := from; i <= n; i++ {
		fmt.Fprintf(&b, "%d화: %s\n", i, briefs[i])
	}
	fmt.Fprintf(&b, "\n## 출력 (다른 글 없이, 정확히 %d줄)\n%d화: ...\n", n-from+1, from)
	return b.String()
}

// replan runs the shared replan-<k+1> call BEFORE revision k reaches either arm (it needs neither arm's output) and rewrites briefs[e+1..] in place, so A and B (which
// hold this same slice) both use the new directions. Like plan it goes to plan/calls.jsonl and is charged to no arm.
func replan(ctx context.Context, t *tracker, briefs []string, k, e int) error {
	if err := t.check(judgeReserve + 1); err != nil {
		return err
	}
	m, f, err := meter(t.c.a, filepath.Join(t.c.out, "plan"))
	if err != nil {
		return err
	}
	defer f.Close()
	// One retry on a bad answer. Two bad answers count as a blocked replan: errBudget, so the run is cut cleanly
	// before revision k reaches either arm and the judge still runs.
	for try, label := 0, fmt.Sprintf("replan-%d", k+1); ; try++ {
		if try > 0 {
			label += "b"
			if err := t.check(judgeReserve + 1); err != nil {
				return err
			}
		}
		text, err := call(ctx, m, label, ReplanPrompt(t.c.idea, briefs, e+1, revReqs[k]))
		var nb []string
		if err == nil {
			if err = write(filepath.Join(t.c.out, "plan", label+".md"), text); err != nil {
				return err
			}
			if nb, err = parseBriefs(text, e+1, t.c.eps); err == nil {
				copy(briefs[e+1:], nb[e+1:])
				return nil
			}
		}
		if try == 1 {
			return fmt.Errorf("%w: replan failed twice (%v)", errBudget, err)
		}
	}
}

// confirmedRevisions is the text A's episode prompts and B's --brief share once n revisions are applied: the
// precedence rule and the revision list. Both arms always carry the block, empty list included, so they stay
// byte-identical from episode 1.
func confirmedRevisions(n int) string {
	s := "## 확정된 수정\n" + precedenceRule + "\n"
	if n == 0 {
		s += "- (아직 없음)\n"
	}
	for k := 0; k < n; k++ {
		s += fmt.Sprintf("- 수정 %d: %s\n", k+1, revReqs[k])
	}
	return s
}

// applied is how many revisions both arms have taken.
func (t *tracker) applied() (n int) {
	for _, d := range t.revDone {
		if d {
			n++
		}
	}
	return
}

// precedenceRule is shared word for word by A's prompts and B's --brief.
const precedenceRule = "아래 `확정된 수정` 이 개요·화별 방향보다 우선한다."

const aHead ="## 연재 규칙\n- 화당 장면 %d개. 장면 사이는 한 줄 `* * *` 로 나눈다. 해라체 서술, 화당 약 5000자, 장면당 약 2500자. 제목·머리말·설명 없이 산문만 출력한다.\n" +
	"- 인물의 이름·나이·관계·물건·시각, 누가 무엇을 아는가는 처음 정한 대로 지킨다.\n"

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

func EpisodePrompt(idea, outline, summary, prev, brief string, ep, n, applied int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 과제: 연재 소설 %d/%d화를 쓴다\n\n", ep, n)
	fmt.Fprintf(&b, aHead, perEp)
	b.WriteString("\n" + confirmedRevisions(applied))
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

func aEpText(c cfg, ep int) string { return strings.TrimSpace(readEp(c.out, ep)) }

var numRE = regexp.MustCompile(`\d+`)

// armA is the direct arm's state. Its calls go through step, which checks the budget first.
type armA struct {
	t       *tracker
	ctx     context.Context
	m       *render.Meter
	cl      io.Closer
	briefs  []string
	outline string
	summary string
}

func newArmA(ctx context.Context, t *tracker, briefs []string) (*armA, error) {
	m, cl, err := meter(t.c.a, filepath.Join(t.c.out, "a"))
	if err != nil {
		return nil, err
	}
	a := &armA{t: t, ctx: ctx, m: m, cl: cl, briefs: briefs}
	if a.outline, err = a.step("outline", OutlinePrompt(t.c.idea, briefs)); err != nil {
		return nil, err
	}
	if err := write(filepath.Join(t.c.out, "a", "outline.md"), a.outline); err != nil {
		return nil, err
	}
	t.record("A", "outline")
	return a, nil
}

func (a *armA) step(label, prompt string) (string, error) {
	if err := a.t.check(judgeReserve + 1); err != nil {
		return "", err
	}
	if err := write(filepath.Join(a.t.c.out, "a", "prompts", label+".txt"), prompt); err != nil {
		return "", err
	}
	return call(a.ctx, a.m, label, prompt)
}

// episode writes episode e (2 calls: write + summary update).
func (a *armA) episode(e int) error {
	c := a.t.c
	prev := ""
	if e > 1 {
		prev = readEp(c.out, e-1)
	}
	text, err := a.step(fmt.Sprintf("ep-%d", e), EpisodePrompt(c.idea, a.outline, a.summary, prev, a.briefs[e], e, c.eps, a.t.applied()))
	if err != nil {
		return err
	}
	if err := write(epPath(c.out, e), text); err != nil {
		return err
	}
	sum, err := a.step(fmt.Sprintf("sum-%d", e), SummaryPrompt(a.summary, "", map[int]string{e: text}))
	if err != nil {
		return err
	}
	a.summary = sum
	if err := write(filepath.Join(c.out, "a", "summary.md"), sum); err != nil {
		return err
	}
	a.t.record("A", fmt.Sprintf("ep%d", e))
	return nil
}

// revise runs revision k after episode e: 1 pick call, one rewrite per picked episode, 1 summary refresh. Nothing
// is applied until the returned commit runs, so an aborted revision leaves A's episodes as they were.
func (a *armA) revise(k, e int) (commit func(), err error) {
	c, n, req := a.t.c, k+1, revReqs[k]
	redone := map[int]string{}
	var newSummary string
	err = a.t.revStep("A", n, func() error {
		picked, err := a.step(fmt.Sprintf("rev%d-pick", n), PickPrompt(req, a.summary, e))
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
		for _, i := range eps {
			prev := ""
			if i > 1 { // the revised previous episode when it was just rewritten
				if prev = redone[i-1]; prev == "" {
					prev = readEp(c.out, i-1)
				}
			}
			text, err := a.step(fmt.Sprintf("rev%d-ep-%d", n, i), RewritePrompt(c.idea, a.outline, a.summary, req, readEp(c.out, i), prev, a.briefs[i], i))
			if err != nil {
				return err
			}
			redone[i] = text
		}
		newSummary, err = a.step(fmt.Sprintf("rev%d-sum", n), SummaryPrompt(a.summary, req, redone))
		return err
	})
	if err != nil {
		return nil, err
	}
	return func() {
		for i, text := range redone {
			write(epPath(c.out, i), text)
		}
		a.summary = newSummary
		write(filepath.Join(c.out, "a", "summary.md"), newSummary)
	}, nil
}
