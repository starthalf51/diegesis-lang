package main

// minedit: spike — minimal-edit render for revise (Arm M). For every scene whose render request changed after a
// retcon (reusing B's revised IR from examples/value/retcon), haiku gets the OLD prose + the NEW scene request + the
// exact old->new request diff and edits only the sentences the diff touches. Card-verified with render.ScoreCard
// (defaults); one retry with the card feedback; then one judge call per retcon on the full revised novel.
//
//	go run ./tools/serial minedit -me-out examples/value/minedit [-me-budget 60] [-me-plan 1,3,2:6] [-me-dry]
//
// Every real call is appended to <out>/calls.jsonl behind a budget guard. No dieg production prompt is touched.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"diegesis/internal/pg"
	"diegesis/internal/render"
)

// ---- diff of two scene requests ----

// sceneLines flattens a scene request into "section: text" lines (what the renderer is told).
func sceneLines(s render.Scene) []string {
	name := func(id string) string {
		if e, ok := s.Entity(id); ok && len(e.Names["ko"]) > 0 {
			return e.Names["ko"][0]
		}
		return id
	}
	fact := func(id string) string { f, _ := s.FactByID(id); return f.Text }
	var l []string
	add := func(sec, t string) {
		if strings.TrimSpace(t) != "" {
			l = append(l, sec+": "+t)
		}
	}
	add("제목", s.Title)
	add("장소", s.Place)
	add("이전 줄거리", s.Previously)
	for _, e := range s.Cast {
		add("등장인물", name(e.ID))
	}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			p := []string{}
			if a.Actor != "" {
				p = append(p, name(a.Actor))
			}
			p = append(p, a.Verb)
			if a.Action != "" {
				p = append(p, "("+a.Action+")")
			}
			for _, t := range a.Targets {
				p = append(p, name(t))
			}
			t := strings.Join(p, " ")
			if a.Fact != "" {
				f, _ := s.FactByID(a.Fact)
				t += ": " + f.Text
				if !f.True {
					t += " (거짓 — 말하는 이가 믿거나 주장할 뿐, 서술자가 단정하면 안 된다)"
				}
			}
			if a.Quote != "" {
				t += fmt.Sprintf(" 대사: %q", a.Quote)
			}
			add("행위", t)
		}
	}
	add("시점 인물", name(s.Directives.POV))
	for _, x := range []struct {
		sec string
		ids []string
	}{{"시점 인물이 믿는 것", s.POVBeliefs}, {"독자가 이미 아는 것", s.ReaderKnows}, {"말하면 안 되는 것", s.Withheld}} {
		for _, id := range x.ids {
			add(x.sec, fact(id))
		}
	}
	add("지시어", fmt.Sprintf("pace=%s tension=%g focus=%s omit=%s", s.Directives.Pace, s.Directives.Tension,
		strings.Join(s.Directives.Focus, ";"), strings.Join(s.Directives.Omit, ";")))
	return l
}

// sceneDiff is the multiset difference of two scene requests: lines only in old (removed), only in new (added).
func sceneDiff(old, nw render.Scene) (removed, added []string) {
	cnt := map[string]int{}
	for _, x := range sceneLines(old) {
		cnt[x]++
	}
	for _, x := range sceneLines(nw) {
		if cnt[x] > 0 {
			cnt[x]--
		} else {
			added = append(added, x)
		}
	}
	nc := map[string]int{}
	for _, x := range sceneLines(nw) {
		nc[x]++
	}
	for _, x := range sceneLines(old) {
		if nc[x] > 0 {
			nc[x]--
		} else {
			removed = append(removed, x)
		}
	}
	return
}

func diffText(rm, ad []string) string {
	var b strings.Builder
	b.WriteString("### 삭제되거나 바뀌기 전의 항목 (옛 명세에만 있음)\n")
	for _, x := range rm {
		b.WriteString("- " + x + "\n")
	}
	if len(rm) == 0 {
		b.WriteString("(없음)\n")
	}
	b.WriteString("\n### 추가되거나 바뀐 뒤의 항목 (새 명세에만 있음)\n")
	for _, x := range ad {
		b.WriteString("- " + x + "\n")
	}
	if len(ad) == 0 {
		b.WriteString("(없음)\n")
	}
	return b.String()
}

// ---- the minimal-edit prompt (spike only; production prompts are untouched) ----

func editSystem(b render.Backend, s render.Scene) string {
	_, tone, _ := strings.Cut(sysOf(b, s), "\n목표 어투(반드시 지킨다): ")
	return `너는 한국어 소설 편집자다. 장면 하나의 옛 산문을 새 장면 명세에 맞게 '최소한으로' 고친다.

규칙:
- 아래 '변경 내역'에 나온 사실·행위와 직접 닿는 문장만 고친다. 닿지 않는 문장은 한 글자도 바꾸지 않고 그대로 둔다(띄어쓰기·문장부호·줄바꿈 포함). 문장을 다듬거나 순서를 바꾸지 않는다.
- '옛 명세에만 있음'에 있는 내용은 산문에서 말하거나 암시하지 않는다(그 내용을 전하던 문장은 새 명세에 맞게 고치거나, 새 명세에 자리가 없으면 그 문장만 지운다).
- '새 명세에만 있음'에 있는 내용은 그것을 전하는 인물·동사(누가 말하는가, 보는가)대로 해당 문장에 반영한다.
- 새 명세(Acts, Must not state or imply 등)에 없는 사건·사실·대사를 새로 지어내지 않는다. 말하면 안 되는 사실은 암시도 하지 않는다. 거짓으로 표시된 사실은 서술자가 사실로 단정하지 않는다. 시점은 그대로 유지한다.
- 고친 문장도 목표 어투를 지킨다.

목표 어투(반드시 지킨다): ` + strings.TrimSpace(tone) + `

출력: 고친 산문 전체만 쓴다. 제목·머리말·설명·표시 없이, 옛 산문과 같은 형식(문단 나눔)으로.`
}

func sysOf(b render.Backend, s render.Scene) string { sy, _, _ := b.Prompt(s); return sy }

func editUser(old string, s render.Scene, b render.Backend, diff, feedback string) string {
	_, user, _ := b.Prompt(s)
	u := "## 옛 산문\n\n" + old + "\n\n## 새 장면 명세\n\n" + user + "\n## 변경 내역 (옛 명세 → 새 명세)\n\n" + diff
	if feedback != "" {
		u += "\n" + feedback
	}
	return u
}

// meFeedback is the Korean analogue of render.feedback (unexported): which checklist items the last edit missed or leaked.
func meFeedback(s render.Scene, rep render.Report) string {
	desc := func(key string) string {
		p := strings.Split(key, "|")
		txt := func(id string) string {
			if f, ok := s.FactByID(id); ok && f.Text != "" {
				return f.Text
			}
			return id
		}
		if len(p) != 3 {
			return txt(key)
		}
		n := p[0]
		if e, ok := s.Entity(p[0]); ok && len(e.Names["ko"]) > 0 {
			n = e.Names["ko"][0]
		}
		return n + " (" + p[1] + "): " + txt(p[2])
	}
	var b strings.Builder
	b.WriteString("## 직전 수정본의 검수 결과\n직전 수정본은 아래 문제가 있었다. 같은 최소 수정 규칙으로 옛 산문에서 다시 고친다.\n")
	for _, m := range rep.Missing {
		b.WriteString("- 빠졌거나 전하는 이가 다르다. 반영할 것: " + desc(m) + "\n")
	}
	for _, l := range rep.Leaked {
		b.WriteString("- 드러나면 안 되는 내용이 드러났다. 지울 것: " + desc(l) + "\n")
	}
	if rep.ExtractError != "" {
		b.WriteString("- 검수 오류: " + rep.ExtractError + "\n")
	}
	return b.String()
}

// ---- IR cache: B's revised IR and the pre-retcon IR, dumped from PG once per distinct story ----

type irIndex struct{ IDs []string }

func dumpIR(ctx context.Context, dir, storyDir, bin string) error {
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err == nil {
		return nil // already dumped (and loaded before): skip
	}
	cmd := exec.CommandContext(ctx, bin, "load", storyDir)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("dieg load %s: %w", storyDir, err)
	}
	fields := strings.Fields(string(out))
	work := fields[len(fields)-1]
	pool, err := pg.Connect(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT id FROM scene WHERE work_id = $1 AND shown ORDER BY discourse_ord, id`, work)
	if err != nil {
		return err
	}
	var idx irIndex
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		idx.IDs = append(idx.IDs, id)
	}
	rows.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, id := range idx.IDs {
		b, err := pg.RenderScene(ctx, pool, work, id)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d.json", i)), b, 0o644); err != nil {
			return err
		}
	}
	b, _ := json.Marshal(idx)
	return os.WriteFile(filepath.Join(dir, "index.json"), b, 0o644)
}

func readIR(dir string) (ids []string, sc map[string]render.Scene, err error) {
	var idx irIndex
	b, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, nil, err
	}
	if err = json.Unmarshal(b, &idx); err != nil {
		return
	}
	sc = map[string]render.Scene{}
	for i := range idx.IDs {
		raw, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%03d.json", i)))
		if err != nil {
			return nil, nil, err
		}
		s, err := render.ParseScene(raw)
		if err != nil {
			return nil, nil, err
		}
		s.FreeDialogue = true // Rerender renders and hashes with Skeleton=true
		ids = append(ids, s.ID)
		sc[s.ID] = s
	}
	return ids, sc, nil
}

// ---- guarded, metered completer ----

type meCaller struct {
	s       *rcState
	real    render.Completer
	arm     *render.Meter // per-retcon M calls.jsonl, labels as dieg writes them (render-/extract-)
	glob    *render.Meter // global calls.jsonl, labels rN-M-...
	rc      string
	label   string
	reserve int
}

func (c *meCaller) Complete(ctx context.Context, sys, user string) (render.Result, error) {
	if err := c.s.guard(1, c.reserve); err != nil {
		return render.Result{}, err
	}
	return c.glob.With(c.arm.With(c.real).Label(c.label)).Label(c.rc + "-M-" + c.label).Complete(ctx, sys, user)
}

// ---- cost helpers ----

// costLabels sums the calls whose label keep() accepts, rewritten to keep()'s returned label (so modelOf applies).
func costLabels(path string, keep func(label string) (string, bool)) Cost {
	tmp, _ := os.CreateTemp("", "me-*.jsonl")
	defer os.Remove(tmp.Name())
	b, _ := os.ReadFile(path)
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(ln), &rec) != nil {
			continue
		}
		if nl, ok := keep(fmt.Sprint(rec["label"])); ok {
			rec["label"] = nl
			o, _ := json.Marshal(rec)
			tmp.Write(append(o, '\n'))
		}
	}
	tmp.Close()
	return costOf(tmp.Name())
}

var sceneLabelRE = regexp.MustCompile(`^(render|extract)-(s\d+)-a\d+$`)

// ---- one retcon ----

type meScene struct {
	ID                                  string
	Edits, CardCalls                    int
	Pass                                bool
	Missing, Leaked                     int
	DiffRemoved, DiffAdded              int
	MChars, BChars                      int // edit distance old->M and old->B
	BPass                               bool
	BCalls                              int
	EditStop                            string
	Err                                 string `json:",omitempty"`
}

type meResult struct {
	ID                        string
	Pilot                     int // >0: only the first Pilot affected scenes, no judge
	Affected, NewScenes       []string
	Scenes                    []meScene
	M, BAff, BCode, BNetRC    Cost
	MTotalPriced, BTotalAff   float64
	CollM, CollB              Collateral
	PassM, PassB              string
	JudgeResidual, JudgeIntro int
	QuotesOK                  string
	HashChecks                string
	Err                       string `json:",omitempty"`
}

type meEnv struct {
	s        *rcState
	irDir    string
	b        render.Backend
	out, rc  string
	edit, ex render.Completer
	glob     *render.Meter
}

func (e *meEnv) runOne(ctx context.Context, r retcon, pilot int) (meResult, error) {
	id := strings.ToLower(r.ID)
	res := meResult{ID: r.ID, Pilot: pilot}
	rdir := filepath.Join(e.out, id)
	rcDir := e.rc
	bDir := filepath.Join(rcDir, id, "b")
	_, oldSc, err := readIR(filepath.Join(e.irDir, "base"))
	if err != nil {
		return res, err
	}
	newIDs, newSc, err := readIR(filepath.Join(e.irDir, id))
	if err != nil {
		return res, err
	}
	// validation: our IR reproduces the stored render hashes of both stories
	okOld, okNew, nOld, nNew := 0, 0, 0, 0
	var aff, added []string
	for _, sid := range newIDs {
		hn, _ := render.RenderHash(newSc[sid], e.b)
		if h := strings.TrimSpace(rcRead(filepath.Join(bDir, "novel", sid+".ko-haera.hash"))); h != "" {
			nNew++
			if h == hn {
				okNew++
			}
		}
		o, inOld := oldSc[sid]
		if !inOld {
			added = append(added, sid)
			continue
		}
		ho, _ := render.RenderHash(o, e.b)
		if h := strings.TrimSpace(rcRead(filepath.Join(e.s.src, "b", "novel", sid+".ko-haera.hash"))); h != "" {
			nOld++
			if h == ho {
				okOld++
			}
		}
		if ho != hn && strings.TrimSpace(rcRead(filepath.Join(e.s.src, "b", "novel", sid+".ko-haera.md"))) != "" {
			aff = append(aff, sid)
		}
	}
	res.Affected, res.NewScenes = aff, added
	res.HashChecks = fmt.Sprintf("old IR hash == stored %d/%d, new IR hash == B stored %d/%d", okOld, nOld, okNew, nNew)
	if okOld != nOld || okNew != nNew {
		return res, fmt.Errorf("%s IR does not reproduce stored hashes: %s", r.ID, res.HashChecks)
	}
	todo := aff
	if pilot > 0 && pilot < len(todo) {
		todo = todo[:pilot]
	}
	reserve := 1
	if pilot > 0 {
		reserve = 0
	}
	if err := e.s.guard(2*len(todo)+reserve, 0); err != nil {
		return res, fmt.Errorf("%s not started: %w", r.ID, err)
	}
	if err := os.MkdirAll(filepath.Join(rdir, "m"), 0o755); err != nil {
		return res, err
	}
	armF, err := os.OpenFile(filepath.Join(rdir, "m", "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return res, err
	}
	defer armF.Close()
	arm := render.NewMeter(nil, armF)
	call := func(real render.Completer, label string, rsv int) *meCaller {
		return &meCaller{s: e.s, real: real, arm: arm, glob: e.glob, rc: id, label: label, reserve: rsv}
	}
	// M's work dir = B's final work dir with the affected scenes' prose replaced by the edits
	if err := copyTree(bDir, filepath.Join(rdir, "m"), func(rel string) bool {
		return rel == "calls.jsonl" || rel == "rounds" || strings.HasPrefix(rel, "diags") || rel == "b.log"
	}); err != nil {
		return res, err
	}
	for _, sid := range todo {
		old := strings.TrimSpace(rcRead(filepath.Join(e.s.src, "b", "novel", sid+".ko-haera.md")))
		rm, ad := sceneDiff(oldSc[sid], newSc[sid])
		diff := diffText(rm, ad)
		sc := newSc[sid]
		ms := meScene{ID: sid, DiffRemoved: len(rm), DiffAdded: len(ad)}
		for _, d := range []struct{ n, t string }{{"diff", diff}} {
			if err := write(filepath.Join(rdir, d.n, sid+".md"), d.t); err != nil {
				return res, err
			}
		}
		sys, fb, prose := editSystem(e.b, sc), "", ""
		for att := 1; att <= 2; att++ {
			user := editUser(old, sc, e.b, diff, fb)
			if err := write(filepath.Join(rdir, "prompts", fmt.Sprintf("%s-a%d.txt", sid, att)), "## SYSTEM\n\n"+sys+"\n\n## USER\n\n"+user); err != nil {
				return res, err
			}
			lab := func(k string) string { return fmt.Sprintf("%s-%s-a%d", k, sid, att) }
			rs, err := call(e.edit, lab("render"), reserve).Complete(ctx, sys, user)
			if err != nil {
				ms.Err = err.Error()
				res.Scenes = append(res.Scenes, ms)
				return res, fmt.Errorf("%s %s edit: %w", r.ID, sid, err)
			}
			ms.Edits++
			ms.EditStop = rs.StopReason
			prose = strings.TrimSpace(rs.Text)
			if err := write(filepath.Join(rdir, "edited", sid+fmt.Sprintf("-a%d.md", att)), prose); err != nil {
				return res, err
			}
			var rep render.Report
			if rs.StopReason != "end_turn" || prose == "" {
				rep.ExtractError = "edit stopped: " + rs.StopReason
			} else if rep, err = render.ScoreCard(ctx, call(e.ex, lab("extract"), reserve), sc, e.b, prose, render.CardOpts{}); err != nil {
				ms.Err = err.Error()
				res.Scenes = append(res.Scenes, ms)
				return res, fmt.Errorf("%s %s card: %w", r.ID, sid, err)
			}
			ms.Pass = render.Passed(render.Render{Report: rep})
			ms.Missing, ms.Leaked = len(rep.Missing), len(rep.Leaked)
			rj, _ := json.MarshalIndent(rep, "", "  ")
			if err := write(filepath.Join(rdir, "edited", sid+fmt.Sprintf("-a%d.report.json", att)), string(rj)); err != nil {
				return res, err
			}
			if ms.Pass || att == 2 {
				break
			}
			fb = meFeedback(sc, rep)
		}
		if err := os.WriteFile(filepath.Join(rdir, "m", "novel", sid+".ko-haera.md"), []byte(prose+"\n"), 0o644); err != nil {
			return res, err
		}
		ms.MChars = editDist([]rune(old), []rune(prose))
		bp := []rune(strings.TrimSpace(rcRead(filepath.Join(bDir, "novel", sid+".ko-haera.md"))))
		ms.BChars = editDist([]rune(old), bp)
		var brep render.Report
		if b, err := os.ReadFile(filepath.Join(bDir, "novel", sid+".ko-haera.report.json")); err == nil && json.Unmarshal(b, &brep) == nil {
			ms.BPass = render.Passed(render.Render{Report: brep})
		}
		res.Scenes = append(res.Scenes, ms)
	}
	e.measure(r, &res, bDir, rdir)
	if pilot > 0 {
		return res, nil
	}
	// judge: same prompt/method as the retcon spike, on M's full revised novel (edited scenes + untouched + B's new scenes)
	idsM, afterM := bUnits(filepath.Join(rdir, "m"), e.s.maxEp)
	e.s.out = e.out
	j, err := e.s.doJudge(ctx, r, "M", rdir, joinUnits(idsM, afterM, "장면"))
	res.JudgeResidual, res.JudgeIntro, res.QuotesOK = j.Residual, j.Introduced, j.QuotesOK
	if j.Err != "" {
		res.Err = j.Err
	}
	return res, err
}

func (e *meEnv) measure(r retcon, res *meResult, bDir, rdir string) {
	ids0, before := bUnits(filepath.Join(e.s.src, "b"), e.s.maxEp)
	_, afterB := bUnits(bDir, e.s.maxEp)
	_, afterM := bUnits(filepath.Join(rdir, "m"), e.s.maxEp)
	res.CollM = collateral(ids0, before, afterM, r.Keys)
	res.CollB = collateral(ids0, before, afterB, r.Keys)
	done := map[string]bool{}
	for _, s := range res.Scenes {
		done[s.ID] = true
	}
	aff := map[string]bool{}
	for _, s := range res.Affected {
		aff[s] = true
	}
	bcalls := filepath.Join(bDir, "calls.jsonl")
	// B on the same scenes M did (affected old scenes only; new scenes' renders are in neither side)
	res.BAff = costLabels(bcalls, func(l string) (string, bool) {
		m := sceneLabelRE.FindStringSubmatch(l)
		return l, m != nil && done[m[2]]
	})
	res.BCode = costLabels(bcalls, func(l string) (string, bool) {
		return l, sceneLabelRE.FindStringSubmatch(l) == nil
	})
	res.M = costLabels(filepath.Join(e.out, "calls.jsonl"), func(l string) (string, bool) {
		p := strings.ToLower(r.ID) + "-M-"
		if !strings.HasPrefix(l, p) || !sceneLabelRE.MatchString(l[len(p):]) {
			return "", false
		}
		return l[len(p):], true
	})
	pm, pb := 0, 0
	for _, s := range res.Scenes {
		if s.Pass {
			pm++
		}
		if s.BPass {
			pb++
		}
	}
	res.PassM = fmt.Sprintf("%d/%d", pm, len(res.Scenes))
	res.PassB = fmt.Sprintf("%d/%d", pb, len(res.Scenes))
	res.MTotalPriced = res.M.Priced + res.BCode.Priced
	res.BTotalAff = res.BAff.Priced + res.BCode.Priced
}

func runMinedit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("minedit", flag.ContinueOnError)
	out := fs.String("me-out", "", "output dir")
	ir := fs.String("me-ir", "", "IR cache dir (default <out>/ir; dry runs point it at the real cache)")
	src := fs.String("me-src", "examples/serial/run3", "finished run3 dir (read only)")
	rc := fs.String("me-rc", "examples/value/retcon", "retcon spike dir (read only)")
	budget := fs.Int("me-budget", 60, "hard cap on real calls")
	plan := fs.String("me-plan", "1,3,2:6", "retcon numbers in run order; N:k = pilot of the first k affected scenes, no judge")
	dry := fs.Bool("me-dry", false, "fake every model call; print one full prompt")
	dumpOnly := fs.Bool("me-dump", false, "only build the IR cache (PG load, no model call)")
	bin := fs.String("me-bin", filepath.Join(os.TempDir(), "dieg-minedit.exe"), "dieg binary built for the IR dump")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("need -me-out")
	}
	irDir := *ir
	if irDir == "" {
		irDir = filepath.Join(*out, "ir")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	bs, err := render.LoadBackends("styles")
	if err != nil {
		return err
	}
	var be render.Backend
	for _, x := range bs {
		if x.ID == "ko-haera" {
			be = x
		}
	}
	s := &rcState{out: *out, src: *src, budget: *budget, dry: *dry, maxEp: 11}
	if err := s.load(); err != nil {
		return err
	}
	// IR cache
	if _, err := os.Stat(filepath.Join(irDir, "base", "index.json")); err != nil {
		build := exec.CommandContext(ctx, "go", "build", "-o", *bin, "./cmd/dieg")
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("build dieg: %w", err)
		}
		tmp, err := os.MkdirTemp("", "me-base-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := copyTree(filepath.Join(*src, "b"), tmp, func(rel string) bool { // the copy B's revise started from
			return rel == "story/ep12" || rel == "story/ep11/s32.dieg.md" || rel == "calls.jsonl" || rel == "rounds" ||
				strings.HasPrefix(rel, "diags") || strings.HasPrefix(rel, "digest") || strings.HasPrefix(rel, "novel")
		}); err != nil {
			return err
		}
		if err := dumpIR(ctx, filepath.Join(irDir, "base"), filepath.Join(tmp, "story"), *bin); err != nil {
			return err
		}
		for _, n := range []string{"r1", "r2", "r3"} {
			if err := dumpIR(ctx, filepath.Join(irDir, n), filepath.Join(*rc, n, "b", "story"), *bin); err != nil {
				return err
			}
		}
	}
	if *dumpOnly {
		return nil
	}
	gf, err := os.OpenFile(filepath.Join(*out, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer gf.Close()
	s.glog = gf
	s.judge = render.NewClaudeCLIWith(sonnet, "")
	edit, ex := render.Completer(render.NewClaudeCLIWith(haiku, "low")), render.Completer(render.NewClaudeCLIWith(sonnet, "low"))
	if *dry {
		s.judge, edit, ex = rcFake{}, meFake{}, meFake{}
	}
	e := &meEnv{irDir: irDir, s: s, b: be, out: *out, rc: *rc, edit: edit, ex: ex, glob: render.NewMeter(nil, gf)}
	var all []meResult
	var stop string
	for _, it := range strings.Split(*plan, ",") {
		num, k, _ := strings.Cut(strings.TrimSpace(it), ":")
		pilot, _ := strconv.Atoi(k)
		var r retcon
		for _, x := range retcons {
			if x.ID == "R"+num {
				r = x
			}
		}
		if r.ID == "" {
			return fmt.Errorf("bad plan item %q", it)
		}
		fmt.Printf("== %s %s (pilot=%d)\n", r.ID, r.Kind, pilot)
		res, err := e.runOne(ctx, r, pilot)
		all = append(all, res)
		if err != nil {
			stop = fmt.Sprintf("%s: %v", r.ID, err)
			break
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"calls_used": s.used(), "budget": s.budget, "stopped": stop, "results": all}, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "results.json"), b, 0o644); err != nil {
		return err
	}
	fmt.Printf("\ncalls used %d/%d stop=%q\n", s.used(), s.budget, stop)
	for _, r := range all {
		pass := 0
		for _, sc := range r.Scenes {
			if sc.Pass {
				pass++
			}
		}
		fmt.Printf("%s scenes=%d/%d calls M=%d B(same scenes)=%d | coll M=%d B=%d | pass M=%s B=%s | judge res=%d intro=%d | %s\n",
			r.ID, len(r.Scenes), len(r.Affected), r.M.Calls, r.BAff.Calls, r.CollM.CharsCollateral, r.CollB.CharsCollateral, r.PassM, r.PassB,
			r.JudgeResidual, r.JudgeIntro, r.HashChecks)
	}
	return nil
}

// ---- dry-run fake ----

type meFake struct{}

var (
	checkIDRE = regexp.MustCompile(`(?m)^(E\d+)\. `)
	witIDRE   = regexp.MustCompile(`(?m)^(W\d+)\. `)
	meN       int
)

func (meFake) Complete(_ context.Context, sys, user string) (render.Result, error) {
	u := render.Usage{Input: 2, Output: 100, CacheRead: 500, CacheWrite: 300}
	if strings.HasPrefix(sys, "You audit") || strings.HasPrefix(sys, "You re-check") {
		var items, wit []string
		bad := strings.Contains(user, "수정(시도1)") && strings.Contains(user, "가짜") && meN%3 == 0
		for i, m := range checkIDRE.FindAllStringSubmatch(user, -1) {
			p := "yes"
			if bad && i == 0 {
				p = "no"
			}
			items = append(items, fmt.Sprintf(`{"id":%q,"fact_present":%q,"as_stated":%q,"quote":""}`, m[1], p, p))
		}
		for _, m := range witIDRE.FindAllStringSubmatch(user, -1) {
			wit = append(wit, fmt.Sprintf(`{"id":%q,"revealed":"no","quote":""}`, m[1]))
		}
		if strings.HasPrefix(sys, "You re-check") {
			return render.Result{Text: "```json\n{\"recheck\":[" + strings.Join(items, ",") + "]}\n```", StopReason: "end_turn", Usage: u}, nil
		}
		return render.Result{Text: "```json\n{\"items\":[" + strings.Join(items, ",") + "],\"withheld\":[" + strings.Join(wit, ",") + "],\"other\":[]}\n```", StopReason: "end_turn", Usage: u}, nil
	}
	meN++
	t := "가짜 수정(시도1). 종이 울렸다."
	if strings.Contains(user, "직전 수정본의 검수 결과") {
		t = "가짜 수정(시도2). 종이 울렸다."
	}
	return render.Result{Text: t, StopReason: "end_turn", Usage: u}, nil
}

