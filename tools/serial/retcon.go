package main

// retcon: one mid-story change (retcon) applied independently to a fresh copy of the finished run3 story, per arm.
// A = serial's own revision method (pick -> rewrite picked episodes -> summary refresh; prompts shared with a.go).
// B = `dieg revise` with the harness defaults (card verifier, --render-effort low). Then one sonnet judge call per arm,
// a deterministic character-level collateral count, and cost from the same calls.jsonl labels run3 used.
//
//	go run ./tools/serial retcon -rc-out examples/value/retcon [-rc-budget 120] [-rc-only 1,2] [-rc-dry]
//
// Every call (A, judge, and B's dieg calls re-read from its calls.jsonl) is appended to <out>/calls.jsonl.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"diegesis/internal/render"
)

type retcon struct {
	ID, Kind, Req string
	Keys          *regexp.Regexp // pre-registered: units whose ORIGINAL text matches are "affected"
}

var retcons = []retcon{
	{"R1", "누가 일으켰나 (렌즈를 깬 사람)",
		"그날 밤 등대 렌즈를 깨뜨린 사람은 도경태가 아니라 백영호이며(불을 다시 켜려다 혼자 램프에 부딪혀 깼다), 렌즈가 깨진 경위를 말하거나 짐작하는 모든 장면을 이에 맞게 고쳐라.",
		regexp.MustCompile(`렌즈|파편|유리 조각|깨뜨`)},
	{"R2", "소품의 위치 (유리 조각을 주운 곳)",
		"정수아가 두꺼운 유리 조각을 주운 곳은 등대 아래 바위틈이 아니라 선착장 방파제 끝이며, 그 장소가 나오는 모든 장면을 이에 맞게 고쳐라.",
		regexp.MustCompile(`조각|바위틈|벼랑 밑|벼랑 바로`)},
	{"R3", "비밀을 안 시점 (앞당김)",
		"서윤은 도경태의 병원 알리바이가 거짓이라는 것을 진료부를 보기 훨씬 전인 4화(등대에 오른 날)에 이미 알게 되며(누구에게서 알았는지는 정해도 된다), 4화 이후 서윤이 그 알리바이를 여전히 믿는 것처럼 나오는 모든 부분을 이에 맞게 고쳐라.",
		regexp.MustCompile(`알리바이|병원|맹장|진료부|흉터`)},
	{"R4", "조연의 관계 (수아와 백영호)",
		"정수아는 백영호와 남남이 아니라 그의 손녀이며, 수아가 나오거나 백영호의 가족·혼자 사는 처지가 언급되는 모든 장면을 이에 맞게 고쳐라.",
		regexp.MustCompile(`수아|영호`)},
}

const rcJudgeTitle = "재설정 잔여 모순 판정"

func rcJudgePrompt(req, text string) string {
	return "# 과제: " + rcJudgeTitle + `

아래는 한국어 연재 소설 완성본이다. 연재 도중 아래 재설정이 확정되어 소설 전체에 반영되었어야 한다.
재설정과 어긋나는 곳이 아직 남아 있는지 찾는다. 문체·재미는 판정하지 않는다.

## 재설정
` + req + `

## 규칙
- residual: 재설정과 지금도 모순되는 모든 곳(앞 화든 뒤 화든). 재설정을 반영하지 않은 옛 설정 그대로의 문장을 포함한다.
- introduced: 재설정을 반영하느라 새로 생긴 것으로 보이는 연속성 모순(인물·물건·장소·시각·누가 무엇을 아는가가 앞뒤로 어긋남). 재설정과 직접 무관한 일반 연속성 오류는 넣지 않는다.
- 항목마다 소설 원문을 그대로 인용한다(한 글자도 바꾸지 않는다). 모순이면 서로 부딪히는 두 곳을 모두 인용한다.
- 해석으로 설명되는 것(인물의 의도적 거짓말, 서술상 생략, 독자에게 감춘 것)은 넣지 않는다. 확신이 없으면 넣지 않는다.

## 출력 (JSON 만, 다른 글 없이)
{"residual": [{"where": "3화", "quotes": ["...", "..."], "why": "..."}], "introduced": [{"where": "5화", "quotes": ["..."], "why": "..."}]}

## 소설

` + text
}

type rcState struct {
	out, src            string
	budget              int
	dry                 bool
	idea, outline, summ string
	briefs              []string
	baseA               map[int]string // episode -> baseline text
	a, judge            render.Completer
	glog                io.Writer
	runDiegFn           func(ctx context.Context, out string, args []string) error
	maxEp               int // A's judged episodes (1..maxEp); B's are the scenes of those episode dirs
	v2                  bool   // plan-7 benchmark: list from file, stamped full B base, s>=skipFrom never rendered, A limited to 1..maxEp
	baseB               string // v2: stamped copy of the run3 B work (every retcon starts from it)
	skipFrom            int    // v2: scenes numbered >= this are excluded from both arms
	bOnly               bool   // v2: run arm B and its judge only
	bReserve            int    // v2: calls kept free before a retcon starts for B's worst case
}

// loadRetcons reads the retcon list file: [{"ID","Kind","Req","Keys"}] with Keys a regexp (pre-registered).
func loadRetcons(path string) ([]retcon, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw []struct{ ID, Kind, Req, Keys string }
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	var out []retcon
	for _, r := range raw {
		re, err := regexp.Compile(r.Keys)
		if err != nil {
			return nil, fmt.Errorf("%s keys: %w", r.ID, err)
		}
		out = append(out, retcon{r.ID, r.Kind, r.Req, re})
	}
	return out, nil
}

// sceneNum is the number in "s32" (0 when it is not an sN id).
func sceneNum(id string) int { n, _ := strconv.Atoi(strings.TrimPrefix(id, "s")); return n }

// skipIDs lists the scene ids >= from found under story/ of a B work.
func skipIDs(work string, from int) []string {
	var ids []string
	fs, _ := filepath.Glob(filepath.Join(work, "story", "ep*", "s*.dieg.md"))
	for _, f := range fs {
		if id := strings.TrimSuffix(filepath.Base(f), ".dieg.md"); sceneNum(id) >= from {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return sceneNum(ids[i]) < sceneNum(ids[j]) })
	return ids
}

// reviseLine pulls the summary fields of the last "revise:" line of a b.log (patched, changed, added, state_stale, skipped).
var reviseRE = regexp.MustCompile(`revise: .*?changed=(\S*) added=(\S*) removed=(\S*) state_stale=(\S*) skipped=(\S*) patched=(\d+)`)

type reviseStat struct{ Changed, Added, StateStale, Skipped []string; Patched int }

func parseRevise(logPath string) (st reviseStat, ok bool) {
	b, _ := os.ReadFile(logPath)
	ms := reviseRE.FindAllStringSubmatch(string(b), -1)
	if len(ms) == 0 {
		return st, false
	}
	m := ms[len(ms)-1]
	sp := func(s string) (o []string) {
		for _, x := range strings.Split(s, ",") {
			if x != "" {
				o = append(o, x)
			}
		}
		return
	}
	st.Changed, st.Added, st.StateStale, st.Skipped = sp(m[1]), sp(m[2]), sp(m[4]), sp(m[5])
	st.Patched, _ = strconv.Atoi(m[6])
	return st, true
}

func (s *rcState) used() int {
	t, _ := ReadCalls(filepath.Join(s.out, "calls.jsonl"))
	return t.Calls
}

// guard refuses to go on unless n more calls (plus judgeRes reserved for the judge) fit in the hard cap.
func (s *rcState) guard(n, judgeRes int) error {
	if u := s.used(); u+n+judgeRes > s.budget {
		return fmt.Errorf("%w (%d used + %d + %d reserved of %d)", errBudget, u, n, judgeRes, s.budget)
	}
	return nil
}

// meterTo is a Meter whose lines go to the global log and to one extra per-arm file.
func (s *rcState) meterTo(c render.Completer, perArm string) (*render.Meter, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(perArm), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(perArm, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return render.NewMeter(c, io.MultiWriter(s.glog, f)), f, nil
}

func (s *rcState) step(ctx context.Context, m *render.Meter, label, prompt string, judgeRes int) (string, error) {
	if err := s.guard(1, judgeRes); err != nil {
		return "", err
	}
	return call(ctx, m, label, prompt)
}

func rcRead(p string) string { b, _ := os.ReadFile(p); return string(b) }

func (s *rcState) load() error {
	r3 := s.src
	s.idea = rcRead(filepath.Join(r3, "..", "idea.txt"))
	s.outline = rcRead(filepath.Join(r3, "a", "outline.md"))
	s.summ = rcRead(filepath.Join(r3, "a", "summary.md"))
	if s.idea == "" || s.outline == "" || s.summ == "" {
		return fmt.Errorf("missing idea/outline/summary under %s", r3)
	}
	// the final briefs: plan, overwritten from ep 5 by replan-1 and from ep 9 by replan-2 (as replan() does)
	pl, err := parseBriefs(rcRead(filepath.Join(r3, "plan", "plan.md")), 1, 12)
	if err != nil {
		return err
	}
	for _, rp := range []struct {
		f    string
		from int
	}{{"replan-1.md", 5}, {"replan-2.md", 9}} {
		nb, err := parseBriefs(rcRead(filepath.Join(r3, "plan", rp.f)), rp.from, 12)
		if err != nil {
			return err
		}
		copy(pl[rp.from:], nb[rp.from:])
	}
	s.briefs = pl
	s.baseA = map[int]string{}
	for e := 1; e <= 12; e++ {
		t := strings.TrimSpace(rcRead(filepath.Join(r3, "a", "eps", fmt.Sprintf("e%02d.md", e))))
		if t == "" {
			return fmt.Errorf("A ep %d missing", e)
		}
		s.baseA[e] = t
	}
	return nil
}

// ---- A ----

func (s *rcState) arm_A(ctx context.Context, r retcon, dir string) (map[int]string, error) {
	m, cl, err := s.meterTo(s.a, filepath.Join(dir, "a", "calls.jsonl"))
	if err != nil {
		return nil, err
	}
	defer cl.Close()
	lab := func(l string) string { return strings.ToLower(r.ID) + "-A-" + l }
	picked, err := s.step(ctx, m, lab("pick"), PickPrompt(r.Req, s.summ, 12), judgeReserve)
	if err != nil {
		return nil, err
	}
	var eps []int
	seen := map[int]bool{}
	for _, x := range numRE.FindAllString(picked, -1) {
		if i, _ := strconv.Atoi(x); i >= 1 && i <= 12 && !seen[i] && (!s.v2 || i <= s.maxEp) { // v2: ep 12 holds the excluded scenes
			seen[i] = true
			eps = append(eps, i)
		}
	}
	if len(eps) == 0 && !s.v2 {
		return nil, fmt.Errorf("%s pick: no episode in %q", r.ID, picked)
	}
	// v2: a pick that names only ep 12 (excluded) means A edits nothing; its residuals get judged as they are
	sort.Ints(eps)
	redone := map[int]string{}
	for _, i := range eps {
		prev := ""
		if i > 1 {
			if prev = redone[i-1]; prev == "" {
				prev = s.baseA[i-1]
			}
		}
		t, err := s.step(ctx, m, lab(fmt.Sprintf("ep-%d", i)), RewritePrompt(s.idea, s.outline, s.summ, r.Req, s.baseA[i], prev, s.briefs[i], i), judgeReserve)
		if err != nil {
			return nil, err
		}
		redone[i] = strings.TrimSpace(t)
	}
	// the summary refresh is part of the harness revision cost (a later episode would read it)
	if len(redone) > 0 {
		if _, err := s.step(ctx, m, lab("sum"), SummaryPrompt(s.summ, r.Req, redone), judgeReserve); err != nil {
			return nil, err
		}
	}
	final := map[int]string{}
	for e, t := range s.baseA {
		final[e] = t
	}
	for e, t := range redone {
		final[e] = t
		if err := write(filepath.Join(dir, "a", "eps", fmt.Sprintf("e%02d.md", e)), t); err != nil {
			return nil, err
		}
	}
	return final, nil
}

// ---- B ----

func copyTree(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		rel = filepath.ToSlash(rel)
		if rel != "." && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

// bUnits maps scene id -> prose, in story order, for episodes 1..maxEp of a B work dir (story/epNN/sK.dieg.md).
func bUnits(work string, maxEp int) (ids []string, text map[string]string) {
	text = map[string]string{}
	for e := 1; e <= maxEp; e++ {
		fs, _ := filepath.Glob(filepath.Join(work, "story", fmt.Sprintf("ep%02d", e), "s*.dieg.md"))
		var sc []string
		for _, f := range fs {
			sc = append(sc, strings.TrimSuffix(filepath.Base(f), ".dieg.md"))
		}
		sort.Slice(sc, func(i, j int) bool {
			a, _ := strconv.Atoi(sc[i][1:])
			b, _ := strconv.Atoi(sc[j][1:])
			return a < b
		})
		for _, id := range sc {
			t := strings.TrimSpace(rcRead(filepath.Join(work, "novel", id+".ko-haera.md")))
			if t == "" { // coded but never rendered (s32): not part of the text
				continue
			}
			ids = append(ids, id)
			text[id] = t
		}
	}
	return
}

func (s *rcState) arm_B(ctx context.Context, r retcon, dir string) error {
	work := filepath.Join(dir, "b")
	// A fresh copy of the final B work. s32..s35 were never rendered (pipeline cut): `revise` would treat them as new
	// and render them, so ep12 and s32 are left out of the copy. Old calls/rounds/digests are not copied (calls.jsonl
	// must hold this revision only).
	skip := func(rel string) bool {
		return rel == "story/ep12" || rel == "story/ep11/s32.dieg.md" || rel == "calls.jsonl" || rel == "rounds" ||
			strings.HasPrefix(rel, "diags") || strings.HasPrefix(rel, "digest")
	}
	args := []string{"revise", work, r.Req}
	if s.v2 { // full stamped base; scenes >= skipFrom stay coded but are never rendered (revise --skip)
		if err := copyTree(s.baseB, work, func(rel string) bool { return rel == "calls.jsonl" }); err != nil {
			return err
		}
		if sk := skipIDs(work, s.skipFrom); len(sk) > 0 {
			args = append(args, "--skip", strings.Join(sk, ","))
		}
		if err := s.guard(2, 0); err != nil { // at least the two judge calls must still fit
			return err
		}
		os.Setenv("DIEG_CALL_CAP", strconv.Itoa(s.budget-s.used()-2)) // hard stop inside dieg: the judges keep 2
	} else {
		if err := copyTree(filepath.Join(s.src, "b"), work, skip); err != nil {
			return err
		}
		if err := s.guard(stepReserve+2, 0); err != nil { // worst case of one revise + the two judge calls
			return err
		}
	}
	err := s.runDiegFn(ctx, dir, withEffort(args))
	os.Unsetenv("DIEG_CALL_CAP")
	// copy dieg's own calls into the global log, whether or not it succeeded
	if b, e2 := os.ReadFile(filepath.Join(work, "calls.jsonl")); e2 == nil {
		for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			var rec map[string]any
			if json.Unmarshal([]byte(ln), &rec) == nil {
				rec["label"] = strings.ToLower(r.ID) + "-B-" + fmt.Sprint(rec["label"])
				o, _ := json.Marshal(rec)
				s.glog.Write(append(o, '\n'))
			}
		}
	}
	return err
}

// ---- metrics ----

func editDist(a, b []rune) int {
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	if len(a) == 0 || len(b) == 0 {
		return len(a) + len(b)
	}
	prev, cur := make([]int, len(b)+1), make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := prev[j-1]
			if a[i-1] != b[j-1] {
				c++
			}
			cur[j] = min(c, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

type Collateral struct {
	Units, UnitsChanged, UnitsAffected, AffectedChanged int
	BaseChars, CharsChanged, CharsCollateral            int
	CollateralUnits                                     []string // changed units outside the affected set, "id:chars"
	AffectedUnits                                       []string
	AddedUnits                                          []string // scenes that did not exist before (not rewrites of old text), "id:chars"
	AddedChars                                          int
}

func collateral(ids []string, before, after map[string]string, keys *regexp.Regexp) Collateral {
	var c Collateral
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	for id, t := range after { // a scene the revision added is new text, not a rewrite of old text: counted apart
		if !seen[id] {
			c.AddedUnits = append(c.AddedUnits, fmt.Sprintf("%s:%d", id, len([]rune(t))))
			c.AddedChars += len([]rune(t))
		}
	}
	sort.Strings(c.AddedUnits)
	for _, id := range ids {
		b, a := []rune(before[id]), []rune(after[id])
		c.Units++
		c.BaseChars += len(b)
		aff := keys.MatchString(before[id])
		if aff {
			c.UnitsAffected++
			c.AffectedUnits = append(c.AffectedUnits, id)
		}
		d := editDist(b, a)
		if d == 0 {
			continue
		}
		c.UnitsChanged++
		c.CharsChanged += d
		if aff {
			c.AffectedChanged++
		} else {
			c.CharsCollateral += d
			c.CollateralUnits = append(c.CollateralUnits, fmt.Sprintf("%s:%d", id, d))
		}
	}
	return c
}

type Cost struct {
	Calls    int     `json:"calls"`
	Errs     int     `json:"errs"`
	Weighted int64   `json:"weighted_in"`
	Output   int64   `json:"output"`
	Priced   float64 `json:"priced"` // run3 README formula: sum over models factor*(weighted_in + 5*out), opus 1, sonnet 0.6, haiku 0.2
}

func costOf(path string) Cost {
	t, _ := ReadCalls(path)
	c := Cost{Calls: t.Calls, Errs: t.Errs, Weighted: Weighted(t.Usage), Output: t.Output}
	f := map[string]float64{opus: 1, sonnet: 0.6, haiku: 0.2}
	for m, v := range t.ByModel {
		c.Priced += f[m] * (float64(Weighted(v.Usage)) + 5*float64(v.Output))
	}
	return c
}

type rcJudgeOut struct {
	Residual   []rcItem `json:"residual"`
	Introduced []rcItem `json:"introduced"`
}
type rcItem struct {
	Where  string   `json:"where"`
	Quotes []string `json:"quotes"`
	Why    string   `json:"why"`
}

// rcRevise: stale = IR-changed or state-stale or new scenes that were (re)rendered; Patched ones went through the
// sentence patch, Full = the rest. IRStale = Changed minus StateStale.
type rcRevise struct {
	Stale, Patched, Full, IRStale, StateStale, Added int
	Skipped                                         []string
}

type rcResult struct {
	ID, Arm    string
	Cost       Cost
	CostNet    *Cost    `json:",omitempty"` // B only: without the calls of re-created old-s32-slot scenes
	Regen      []string `json:",omitempty"`
	Revise     *rcRevise `json:",omitempty"` // B only (v2): what dieg revise re-rendered, from its summary line
	Collateral Collateral
	JudgeRaw   string `json:",omitempty"`
	Residual   int    // judge's raw item counts (human-verified marks live in README/verification.md)
	Introduced int
	QuotesOK   string
	Err        string `json:",omitempty"`
}

func (s *rcState) doJudge(ctx context.Context, r retcon, arm, dir, text string) (rcResult, error) {
	res := rcResult{ID: r.ID, Arm: arm}
	m, cl, err := s.meterTo(s.judge, filepath.Join(dir, "judge.calls.jsonl"))
	if err != nil {
		return res, err
	}
	defer cl.Close()
	prompt := rcJudgePrompt(r.Req, text)
	name := strings.ToLower(arm)
	if err := write(filepath.Join(dir, "judge", name+".prompt.txt"), prompt); err != nil {
		return res, err
	}
	raw, err := s.step(ctx, m, strings.ToLower(r.ID)+"-judge-"+arm, prompt, 0)
	if err != nil {
		return res, err
	}
	if err := write(filepath.Join(dir, "judge", name+".out.txt"), raw); err != nil {
		return res, err
	}
	res.JudgeRaw = filepath.ToSlash(filepath.Join("judge", name+".out.txt"))
	j := raw
	if a, b := strings.Index(j, "{"), strings.LastIndex(j, "}"); a >= 0 && b > a {
		j = j[a : b+1]
	}
	var got rcJudgeOut
	if err := json.Unmarshal([]byte(j), &got); err != nil {
		res.Residual, res.Introduced = -1, -1
		res.Err = "judge output not JSON: " + err.Error()
		return res, nil
	}
	res.Residual, res.Introduced = len(got.Residual), len(got.Introduced)
	var q strings.Builder
	src, ok, all := norm(text), 0, 0
	for _, g := range []struct {
		n  string
		it []rcItem
	}{{"residual", got.Residual}, {"introduced", got.Introduced}} {
		fmt.Fprintf(&q, "## %s (%d)\n", g.n, len(g.it))
		for n, it := range g.it {
			fmt.Fprintf(&q, "\n%s%d. %s — %s\n", g.n[:1], n+1, it.Where, it.Why)
			for _, qu := range it.Quotes {
				all++
				mark := "없음"
				if strings.Contains(src, norm(qu)) {
					mark, ok = "있음", ok+1
				}
				fmt.Fprintf(&q, "   - (%s) %s\n", mark, qu)
			}
		}
	}
	res.QuotesOK = fmt.Sprintf("%d/%d", ok, all)
	return res, write(filepath.Join(dir, "judge", name+".quotes.md"), "# 판정 인용 자동 대조 (있음 = 원문에 그대로 있음; 지적의 옳고 그름은 verification.md)\n\n"+q.String())
}

func joinUnits(ids []string, text map[string]string, label string) string {
	var b strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&b, "### %s %s\n\n%s\n\n", label, id, text[id])
	}
	return b.String()
}

func (s *rcState) one(ctx context.Context, r retcon) ([]rcResult, error) {
	dir := filepath.Join(s.out, strings.ToLower(r.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var res []rcResult
	var finalA map[int]string
	if !s.bOnly { // rerun of B alone: A's results are those of the first run, same inputs
		var errA error
		if finalA, errA = s.arm_A(ctx, r, dir); errA != nil {
			return res, fmt.Errorf("%s A: %w", r.ID, errA)
		}
	}
	errB := s.arm_B(ctx, r, dir)
	if errB != nil {
		return res, fmt.Errorf("%s B: %w", r.ID, errB)
	}
	// texts
	_ = finalA
	resA, resB, idsA, afterA, idsB, afterB := s.measure(r, dir)
	if err := s.guard(2, 0); err != nil {
		return []rcResult{resA, resB}, err
	}
	var err error
	if s.bOnly {
		resB, err = s.judged(ctx, r, "B", dir, resB, joinUnits(idsB, afterB, "장면"))
		return []rcResult{resB}, err
	}
	if resA, err = s.judged(ctx, r, "A", dir, resA, joinUnits(idsA, afterA, "화")); err != nil {
		return []rcResult{resA, resB}, err
	}
	if resB, err = s.judged(ctx, r, "B", dir, resB, joinUnits(idsB, afterB, "장면")); err != nil {
		return []rcResult{resA, resB}, err
	}
	// revised texts for the record (B's novel/ is already under b/)
	return []rcResult{resA, resB}, nil
}

func (s *rcState) judged(ctx context.Context, r retcon, arm, dir string, base rcResult, text string) (rcResult, error) {
	j, err := s.doJudge(ctx, r, arm, dir, text)
	j.Cost, j.Collateral, j.Revise, j.CostNet, j.Regen = base.Cost, base.Collateral, base.Revise, base.CostNet, base.Regen
	return j, err
}

func runRetcon(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("retcon", flag.ContinueOnError)
	out := fs.String("rc-out", "", "output dir")
	src := fs.String("rc-src", "examples/serial/run3", "finished run3 dir (read only)")
	budget := fs.Int("rc-budget", 120, "hard cap on real calls (A + B + judge)")
	only := fs.String("rc-only", "", "comma list of retcon numbers (default all)")
	recompute := fs.Bool("rc-recompute", false, "re-derive cost/collateral from files of a finished run (no call); writes results2.json")
	dry := fs.Bool("rc-dry", false, "fake every model call and the dieg subprocess; print the commands")
	list := fs.String("rc-list", "", "plan-7 benchmark: retcon list JSON; B starts from the stamped full base, scenes >= -rc-skip-from are never rendered")
	skipFrom := fs.Int("rc-skip-from", 32, "with -rc-list: scenes numbered >= this are excluded from both arms")
	bOnly := fs.Bool("rc-b-only", false, "with -rc-list: run arm B only (A comes from an earlier run)")
	bRes := fs.Int("rc-b-reserve", 45, "with -rc-list: calls that must be free before a retcon starts, for B's worst case")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("need -rc-out")
	}
	s := &rcState{out: *out, src: *src, budget: *budget, dry: *dry, maxEp: 11}
	if err := os.MkdirAll(s.out, 0o755); err != nil {
		return err
	}
	if *list != "" {
		rs, err := loadRetcons(*list)
		if err != nil {
			return err
		}
		retcons = rs
		s.v2, s.skipFrom, s.bReserve, s.bOnly = true, *skipFrom, *bRes, *bOnly
		s.baseB = filepath.Join(s.out, "base", "b")
	}
	if err := s.load(); err != nil {
		return err
	}
	if *recompute {
		return s.recompute()
	}
	gf, err := os.OpenFile(filepath.Join(s.out, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer gf.Close()
	s.glog = gf
	s.a, s.judge = render.NewClaudeCLIWith(opus, "medium"), render.NewClaudeCLIWith(sonnet, "")
	s.runDiegFn = runDieg
	if *dry {
		s.a, s.judge = rcFake{}, rcFake{}
		s.runDiegFn = func(ctx context.Context, out string, a []string) error {
			fmt.Printf("DRY dieg %s\n", strings.Join(append([]string{a[0], "<" + filepath.Base(a[1]) + ">"}, a[2:]...), " "))
			if a[0] == "stamp" {
				return nil
			}
			return fakeRetconDieg(ctx, out, a)
		}
	} else {
		if diegBin, err = filepath.Abs(filepath.Join(s.out, "dieg.exe")); err != nil {
			return err
		}
		build := exec.CommandContext(ctx, "go", "build", "-o", diegBin, "./cmd/dieg")
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("build dieg: %w", err)
		}
	}
	if s.v2 { // stamped base: once, no model call (skipped when the marker is there)
		mark := filepath.Join(s.out, "base", ".stamped")
		if _, err := os.Stat(mark); err != nil {
			skipB := func(rel string) bool {
				return rel == "calls.jsonl" || rel == "rounds" || strings.HasPrefix(rel, "diags") || strings.HasPrefix(rel, "digest")
			}
			if err := copyTree(filepath.Join(s.src, "b"), s.baseB, skipB); err != nil {
				return err
			}
			if err := s.runDiegFn(ctx, filepath.Join(s.out, "base"), []string{"stamp", s.baseB}); err != nil {
				return fmt.Errorf("stamp: %w", err)
			}
			if err := os.WriteFile(mark, []byte("stamped\n"), 0o644); err != nil {
				return err
			}
		}
	}
	want := map[string]bool{}
	for _, x := range strings.Split(*only, ",") {
		if x = strings.TrimSpace(x); x != "" {
			want["R"+x] = true
		}
	}
	var all []rcResult
	var stop string
	for _, r := range retcons {
		if len(want) > 0 && !want[r.ID] {
			continue
		}
		// do not start a retcon that clearly cannot finish: A ~8 + B worst case + judges
		need := 8 + stepReserve + 2
		if s.v2 {
			need = 10 + s.bReserve + 2
			if s.bOnly {
				need = s.bReserve + 1
			}
		}
		if err := s.guard(need, 0); err != nil {
			stop = fmt.Sprintf("%s not started: %v", r.ID, err)
			break
		}
		fmt.Printf("== %s %s\n", r.ID, r.Kind)
		rs, err := s.one(ctx, r)
		all = append(all, rs...)
		if err != nil {
			stop = fmt.Sprintf("%s: %v", r.ID, err)
			break
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"calls_used": s.used(), "budget": s.budget, "stopped": stop, "results": all}, "", "  ")
	if err := os.WriteFile(filepath.Join(s.out, "results.json"), b, 0o644); err != nil {
		return err
	}
	fmt.Printf("\ncalls used %d/%d stop=%q\n", s.used(), s.budget, stop)
	for _, r := range all {
		c := r.Collateral
		fmt.Printf("%s %s calls=%d w_in=%d out=%d priced=%.0f | judge residual=%d introduced=%d quotes=%s | units %d/%d changed (affected %d), chars changed %d collateral %d\n",
			r.ID, r.Arm, r.Cost.Calls, r.Cost.Weighted, r.Cost.Output, r.Cost.Priced, r.Residual, r.Introduced, r.QuotesOK, c.UnitsChanged, c.Units, c.UnitsAffected, c.CharsChanged, c.CharsCollateral)
	}
	return nil
}

// ---- dry-run fakes ----

type rcFake struct{}

func (rcFake) Complete(ctx context.Context, sys, user string) (render.Result, error) {
	if strings.Contains(user, rcJudgeTitle) {
		return render.Result{Text: `{"residual": [{"where": "3화", "quotes": ["가짜 장면"], "why": "x"}], "introduced": []}`, StopReason: "end_turn",
			Usage: render.Usage{Input: 2, Output: 10, CacheRead: 100, CacheWrite: 50}}, nil
	}
	return fake{eps: 12}.Complete(ctx, sys, user)
}

// fakeRetconDieg: 3 metered calls into <work>/calls.jsonl and one rewritten scene.
func fakeRetconDieg(ctx context.Context, out string, args []string) error {
	work := args[1]
	m, f, err := meter(fake{}, work)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range []string{"revise-r1", "render-s3-a1", "extract-s3-a1"} {
		if _, err := m.Label(l).Complete(ctx, "", ""); err != nil {
			return err
		}
	}
	line := "revise: rounds=1 clean=true changed=s3,s4 added= removed= state_stale=s4 skipped=s32 patched=1 failed=0[] calls=3\n"
	if err := os.WriteFile(filepath.Join(out, "b.log"), []byte(line), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(work, "novel", "s3.ko-haera.md"), []byte("가짜 장면이다. 수정됨.\n"), 0o644)
}

// measure reads only files: A's final episodes (run3 baseline overridden by <dir>/a/eps), B's revised work, calls.jsonl.
func (s *rcState) measure(r retcon, dir string) (resA, resB rcResult, idsA []string, afterA map[string]string, idsB []string, afterB map[string]string) {
	before, afterA := map[string]string{}, map[string]string{}
	for e := 1; e <= s.maxEp; e++ {
		id := fmt.Sprintf("e%02d", e)
		idsA = append(idsA, id)
		before[id] = s.baseA[e]
		afterA[id] = s.baseA[e]
		if t := strings.TrimSpace(rcRead(filepath.Join(dir, "a", "eps", id+".md"))); t != "" {
			afterA[id] = t
		}
	}
	resA = rcResult{ID: r.ID, Arm: "A", Cost: costOf(filepath.Join(dir, "a", "calls.jsonl")), Collateral: collateral(idsA, before, afterA, r.Keys)}
	ids0, beforeB := bUnits(filepath.Join(s.src, "b"), s.maxEp)
	idsB, afterB = bUnits(filepath.Join(dir, "b"), s.maxEp)
	resB = rcResult{ID: r.ID, Arm: "B", Cost: costOf(filepath.Join(dir, "b", "calls.jsonl")), Collateral: collateral(ids0, beforeB, afterB, r.Keys)}
	if st, ok := parseRevise(filepath.Join(dir, "b.log")); ok {
		n := len(st.Changed) + len(st.Added)
		resB.Revise = &rcRevise{Stale: n, Patched: st.Patched, Full: n - st.Patched, IRStale: len(st.Changed) - len(st.StateStale),
			StateStale: len(st.StateStale), Added: len(st.Added), Skipped: st.Skipped}
	}
	// B's net cost: without the render/extract calls of scenes in the old s32 slot (ep11) that the copy lacked and revise re-created
	var regen []string
	for _, u := range resB.Collateral.AddedUnits {
		id, _, _ := strings.Cut(u, ":")
		if m, _ := filepath.Glob(filepath.Join(dir, "b", "story", "ep11", id+".dieg.md")); len(m) > 0 {
			regen = append(regen, id)
		}
	}
	if len(regen) > 0 {
		c := costExcl(filepath.Join(dir, "b", "calls.jsonl"), regen)
		resB.CostNet = &c
		resB.Regen = regen
	}
	return
}

func costExcl(path string, ids []string) Cost {
	tmp, _ := os.CreateTemp("", "rc-*.jsonl")
	defer os.Remove(tmp.Name())
	b, _ := os.ReadFile(path)
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		drop := false
		for _, id := range ids {
			if strings.Contains(ln, "\"label\":\"render-"+id+"-") || strings.Contains(ln, "\"label\":\"extract-"+id+"-") {
				drop = true
			}
		}
		if !drop {
			tmp.WriteString(ln + "\n")
		}
	}
	tmp.Close()
	return costOf(tmp.Name())
}

// recompute re-derives cost and collateral from files (no model call) and keeps the judge counts of results.json.
func (s *rcState) recompute() error {
	var old struct{ Results []rcResult }
	if b, err := os.ReadFile(filepath.Join(s.out, "results.json")); err != nil || json.Unmarshal(b, &old) != nil {
		return fmt.Errorf("results.json: %v", err)
	}
	var out []rcResult
	for _, o := range old.Results {
		for _, r := range retcons {
			if r.ID != o.ID {
				continue
			}
			a, b, _, _, _, _ := s.measure(r, filepath.Join(s.out, strings.ToLower(r.ID)))
			n := a
			if o.Arm == "B" {
				n = b
			}
			n.JudgeRaw, n.Residual, n.Introduced, n.QuotesOK, n.Err = o.JudgeRaw, o.Residual, o.Introduced, o.QuotesOK, o.Err
			out = append(out, n)
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"results": out}, "", "  ")
	return os.WriteFile(filepath.Join(s.out, "results2.json"), b, 0o644)
}
