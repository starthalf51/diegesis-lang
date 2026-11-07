// renderer-ab 는 렌더러(단일 render.RenderVerified2 / 계층형 narrate.RenderLayered)와 렌더 모델(sonnet / haiku)을
// 같은 장면 4개(코퍼스 macbeth 1.3·chibi 45.88 + 연재 haejin 의 행위가 많은 2장면)에서 비교한다. 재추출은 모든 팔에서
// sonnet/low 로 고정한다. 실행 순서는 Experiment 를 본다(장면 단위로 판정 3팔 → 문체 교체 → S-hai → 참고 실행).
//
//	go run ./tools/renderer-ab -out examples/renderer-ab [-budget 119] [-load=false] [-weights claude-sonnet-5-5=3:15,claude-haiku-5-5=1:5]
//
// 실제 호출은 -budget 을 넘지 않는다(넘기 직전 호출은 errBudget 으로 막힌다). 저장소 루트에서 돌린다.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"diegesis/internal/narrate"
	"diegesis/internal/pg"
	"diegesis/internal/render"
)

const (
	sonnet, haiku = "claude-sonnet-5-5", "claude-haiku-5-5"
	renderEffort  = "medium"
	extractEffort = "low"
	target        = 2500
)

type Arm struct{ ID, Renderer, Model string }

var Arms = []Arm{{"S-son", "single", sonnet}, {"L-son", "layered", sonnet}, {"L-hai", "layered", haiku}, {"S-hai", "single", haiku}}

// Scene 은 실험 장면 하나: 작품 ID 와 렌더 요청.
type Scene struct {
	Work string
	S    render.Scene
}

// Run 은 (팔, 장면, 백엔드) 렌더 1회의 기록.
type Run struct {
	Arm, Scene, Backend   string
	Skipped, Err          string `json:",omitempty"`
	Passed                bool
	Calls                 int
	RenderCalls           int
	ExtractCalls          int
	RenderModel           string
	RenderUsage           render.Usage
	ExtractUsage          render.Usage
	Attempts              int     `json:",omitempty"` // single: 렌더 시도 수
	Tone                  float64 // 백엔드 목표 어투 비율
	Chars                 int     // 공백 뺀 글자 수
	Beats                 int     `json:",omitempty"`
	BeatAttempts          []int   `json:",omitempty"`
	BeatRetries           int     `json:",omitempty"`
	BeatFailed            []int   `json:",omitempty"`
	StyleOK               bool    `json:",omitempty"`
	FinalFrom             string  `json:",omitempty"`
	Missing, Leaked       []string
	NewEvents, FactHalluc []string // 환각을 자유 문장(new_events)과 사실형(unknown-fact·extra-act)으로 나눈다
	ExtractError          string   `json:",omitempty"`
	Prose                 string   `json:"-"`
}

var errBudget = errors.New("call budget reached")

// counter 는 실제 호출 수를 세고 예산을 넘는 호출을 막는다. Meter 안쪽에 둔다(라벨은 Meter 가 붙인다).
type counter struct {
	c       render.Completer
	n, used *int
	budget  int
}

func (k counter) Complete(ctx context.Context, system, user string) (render.Result, error) {
	if *k.used >= k.budget {
		return render.Result{}, errBudget
	}
	*k.used++
	*k.n++
	return k.c.Complete(ctx, system, user)
}

type cfg struct {
	out     string
	budget  int
	used    int
	mk      func(model, effort string) render.Completer
	weights map[string][2]float64
	// 지금까지 계층형에서 본 비트 수와 재시도 수(estimate 의 재시도율)
	beatsSeen, retriesSeen int
}

// meters: 렌더 역할과 재추출 역할을 따로 잰다(모델별 토큰).
func (c *cfg) meters(w io.Writer, model string, rn, xn *int) (*render.Meter, *render.Meter) {
	return render.NewMeter(counter{c.mk(model, renderEffort), rn, &c.used, c.budget}, w),
		render.NewMeter(counter{c.mk(sonnet, extractEffort), xn, &c.used, c.budget}, w)
}

// estimate 는 한 번 돌리기 전에 남겨 둘 호출 수: 단일은 최악(3×2). 계층형은 비트마다 (생성+검증) × (1+재시도율) +
// 문체·연결·최종·되돌림 대조 4. 재시도율은 지금까지 본 값이고 처음은 0.5 다.
func (c *cfg) estimate(renderer string, beats int) int {
	if renderer == "single" {
		return 6
	}
	rate := float64(c.retriesSeen+1) / float64(c.beatsSeen+2)
	return int(float64(beats*2)*(1+rate)+0.999) + 4
}

func beatsOf(s render.Scene) int {
	p, err := narrate.PlanScene(s, narrate.DefaultOpts().Plan)
	if err != nil {
		return 3
	}
	return len(p.Beats)
}

func fill(r *Run, rr render.Render, b render.Backend) {
	r.Passed, r.Prose = render.Passed(rr), rr.Prose
	r.Tone = rr.Tones[b.Tone]
	r.Chars = utf8.RuneCountInString(strings.Join(strings.Fields(rr.Prose), ""))
	rep := rr.Report
	r.Missing, r.Leaked, r.ExtractError = rep.Missing, rep.Leaked, rep.ExtractError
	r.NewEvents, r.FactHalluc = []string{}, []string{}
	for _, h := range rep.Hallucinated {
		if strings.HasPrefix(h, "unknown-fact:") || strings.HasPrefix(h, "extra-act:") {
			r.FactHalluc = append(r.FactHalluc, h)
		} else {
			r.NewEvents = append(r.NewEvents, h)
		}
	}
}

// runOne 은 (팔, 장면, 백엔드) 하나를 돌린다. keep 이 nil 이 아니면 계층형의 비트 글을 거기 남긴다(문체 교체용).
func (c *cfg) runOne(ctx context.Context, a Arm, sc Scene, b render.Backend, keep *[]string) Run {
	r := Run{Arm: a.ID, Scene: sc.Work + "/" + sc.S.ID, Backend: b.ID, RenderModel: a.Model}
	if need := c.estimate(a.Renderer, beatsOf(sc.S)); c.used+need > c.budget {
		r.Skipped = fmt.Sprintf("budget: used %d + est %d > %d", c.used, need, c.budget)
		return r
	}
	f, err := openLog(filepath.Join(c.out, a.ID, "calls.jsonl"))
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer f.Close()
	rm, xm := c.meters(f, a.Model, &r.RenderCalls, &r.ExtractCalls)
	var rr render.Render
	if a.Renderer == "single" {
		var n int
		rr, n, err = render.RenderVerified2(ctx, render.Completers{Render: rm.Label("render-" + sc.S.ID + "." + b.ID),
			Extract: xm.Label("extract-" + sc.S.ID + "." + b.ID)}, sc.Work, sc.S, b, render.VerifyOpts{K: 1, Retries: 2})
		r.Attempts = n
	} else {
		roles := narrate.Roles{Beat: rm, Verify: xm, Style: rm, Stitch: rm}
		o := narrate.DefaultOpts()
		var texts []string
		var st narrate.LayerStats
		if texts, st, err = narrate.Beats(ctx, roles, sc.S, b, o); err == nil {
			if keep != nil {
				*keep = texts
			}
			rr, st, err = narrate.Finish(ctx, roles, sc.Work, sc.S, b, o, texts, st)
		}
		r.Beats, r.BeatAttempts, r.BeatFailed, r.StyleOK, r.FinalFrom = st.Beats, st.BeatAttempts, st.BeatFailed, st.StyleOK, st.FinalFrom
		for _, n := range st.BeatAttempts {
			r.BeatRetries += n - 1
			c.beatsSeen, c.retriesSeen = c.beatsSeen+1, c.retriesSeen+n-1
		}
	}
	r.Calls = r.RenderCalls + r.ExtractCalls
	r.RenderUsage, r.ExtractUsage = rm.Total(), xm.Total()
	if err != nil {
		r.Err = err.Error()
	}
	fill(&r, rr, b)
	return r
}

// swap 은 keep 의 비트 글로 [3]·[4]+최종 대조만 백엔드 b 로 다시 돈다.
func (c *cfg) swap(ctx context.Context, sc Scene, b render.Backend, texts []string) Run {
	r := Run{Arm: "L-hai-swap", Scene: sc.Work + "/" + sc.S.ID, Backend: b.ID, RenderModel: haiku}
	if len(texts) == 0 {
		r.Skipped = "no beat texts"
		return r
	}
	if c.used+5 > c.budget {
		r.Skipped = "budget"
		return r
	}
	f, err := openLog(filepath.Join(c.out, r.Arm, "calls.jsonl"))
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer f.Close()
	rm, xm := c.meters(f, haiku, &r.RenderCalls, &r.ExtractCalls)
	st := narrate.LayerStats{Beats: len(texts)}
	rr, st, err := narrate.Finish(ctx, narrate.Roles{Beat: rm, Verify: xm, Style: rm, Stitch: rm}, sc.Work, sc.S, b, narrate.DefaultOpts(), texts, st)
	if err != nil {
		r.Err = err.Error()
	}
	r.StyleOK, r.FinalFrom = st.StyleOK, st.FinalFrom
	r.Calls = r.RenderCalls + r.ExtractCalls
	r.RenderUsage, r.ExtractUsage = rm.Total(), xm.Total()
	fill(&r, rr, b)
	return r
}

func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func (c *cfg) save(r Run) error {
	if r.Prose == "" {
		return nil
	}
	p := filepath.Join(c.out, r.Arm, strings.ReplaceAll(r.Scene, "/", "_")+"."+r.Backend+".md")
	return os.WriteFile(p, []byte(strings.TrimSpace(r.Prose)+"\n"), 0o644)
}

// Experiment 는 판정을 지키는 순서로 돈다. 장면을 계층형 비트 수가 적은 순으로 놓고, 장면마다 판정 3팔(S-son·L-son·L-hai)을
// 한꺼번에 돈다. 세 팔 추정치(+아직이면 문체 교체 5)가 남은 예산에 들어갈 때만 그 장면을 시작하므로, 끝난 장면은 세 팔이 모두
// 같은 장면이다. 첫 장면의 L-hai 비트 글로 문체 교체를 잰다. 다음은 끝난 장면의 S-hai, 같은 장면의 hapsho 단일(비교),
// 남은 예산으로 판정에 못 든 장면의 단일 팔(S-son-extra·S-hai-extra, 판정 밖 참고)이다.
func (c *cfg) Experiment(ctx context.Context, scenes []Scene, haera, hapsho render.Backend) []Run {
	var runs []Run
	add := func(r Run) {
		if err := c.save(r); err != nil && r.Err == "" {
			r.Err = err.Error()
		}
		fmt.Fprintf(os.Stderr, "renderer-ab: %-12s %-16s %-9s pass=%v calls=%d used=%d %s%s\n", r.Arm, r.Scene, r.Backend, r.Passed, r.Calls, c.used, r.Skipped, r.Err)
		runs = append(runs, r)
	}
	scenes = append([]Scene(nil), scenes...)
	sort.SliceStable(scenes, func(i, j int) bool { return beatsOf(scenes[i].S) < beatsOf(scenes[j].S) })
	var done, rest []Scene
	var keep []string
	for _, sc := range scenes {
		n := beatsOf(sc.S)
		need := c.estimate("single", n) + 2*c.estimate("layered", n)
		if len(done) == 0 {
			need += 5
		}
		if c.used+need > c.budget {
			for _, a := range Arms[:3] {
				add(Run{Arm: a.ID, Scene: sc.Work + "/" + sc.S.ID, Backend: haera.ID, RenderModel: a.Model,
					Skipped: fmt.Sprintf("budget: used %d + scene est %d > %d", c.used, need, c.budget)})
			}
			rest = append(rest, sc)
			continue
		}
		for _, a := range Arms[:3] {
			var k *[]string
			if a.ID == "L-hai" && len(done) == 0 {
				k = &keep
			}
			add(c.runOne(ctx, a, sc, haera, k))
		}
		if len(done) == 0 {
			add(c.swap(ctx, sc, hapsho, keep))
		}
		done = append(done, sc)
	}
	for _, sc := range done {
		add(c.runOne(ctx, Arms[3], sc, haera, nil))
	}
	if len(done) > 0 {
		add(c.runOne(ctx, Arm{"S-son-hapsho", "single", sonnet}, done[0], hapsho, nil))
	}
	for _, sc := range rest {
		add(c.runOne(ctx, Arm{"S-son-extra", "single", sonnet}, sc, haera, nil))
		add(c.runOne(ctx, Arm{"S-hai-extra", "single", haiku}, sc, haera, nil))
	}
	return runs
}

func parseWeights(s string) (map[string][2]float64, error) {
	m := map[string][2]float64{}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		i, o, ok2 := strings.Cut(v, ":")
		a, e1 := strconv.ParseFloat(i, 64)
		b, e2 := strconv.ParseFloat(o, 64)
		if !ok || !ok2 || e1 != nil || e2 != nil {
			return nil, fmt.Errorf("bad weight %q", kv)
		}
		m[k] = [2]float64{a, b}
	}
	return m, nil
}

// loadScenes 는 작품을 적재하고 장면 4개를 PG 에서 받는다. 연재 장면은 행위 수가 많은 2개이고 FreeDialogue 다.
var loadScenes = func(ctx context.Context, load bool) ([]Scene, error) {
	dirs := []string{"corpus/macbeth", "corpus/chibi", "examples/serial/run1/b/story"}
	if load {
		for _, d := range dirs {
			cmd := exec.CommandContext(ctx, "go", "run", "./cmd/dieg", "load", d)
			cmd.Stderr = os.Stderr
			if out, err := cmd.Output(); err != nil {
				return nil, fmt.Errorf("load %s: %w (%s)", d, err, out)
			}
		}
	}
	pool, err := pg.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	get := func(work, id string) (render.Scene, error) {
		b, err := pg.RenderScene(ctx, pool, work, id)
		if err != nil {
			return render.Scene{}, err
		}
		return render.ParseScene(b)
	}
	var out []Scene
	for _, ws := range [][2]string{{"macbeth", "1.3"}, {"chibi", "45.88"}} {
		s, err := get(ws[0], ws[1])
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", ws[0], ws[1], err)
		}
		out = append(out, Scene{ws[0], s})
	}
	rows, err := pool.Query(ctx, `SELECT id FROM scene WHERE work_id = 'haejin' AND shown ORDER BY discourse_ord, id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	var ser []Scene
	for _, id := range ids {
		s, err := get("haejin", id)
		if err != nil {
			return nil, fmt.Errorf("haejin %s: %w", id, err)
		}
		s.FreeDialogue = true // 연재는 skeleton 모드로 썼다
		ser = append(ser, Scene{"haejin", s})
	}
	sort.SliceStable(ser, func(i, j int) bool { return acts(ser[i].S) > acts(ser[j].S) })
	if len(ser) < 2 {
		return nil, fmt.Errorf("haejin: %d shown scenes", len(ser))
	}
	return append(out, ser[:2]...), nil
}

func acts(s render.Scene) int {
	n := 0
	for _, b := range s.Beats {
		n += len(b.Acts)
	}
	return n
}

func backends(dir string) (haera, hapsho render.Backend, err error) {
	bs, err := render.LoadBackends(dir)
	if err != nil {
		return
	}
	for _, b := range bs {
		switch b.ID {
		case "ko-haera":
			haera = b
		case "ko-hapsho":
			hapsho = b
		}
	}
	if haera.ID == "" || hapsho.ID == "" {
		err = fmt.Errorf("%s: need ko-haera and ko-hapsho", dir)
	}
	return
}

// run 은 실험 전체: 장면 → 실행 → runs.json·summary.json·table.md.
func run(ctx context.Context, c *cfg, styles string, load bool) (Summary, error) {
	haera, hapsho, err := backends(styles)
	if err != nil {
		return Summary{}, err
	}
	scenes, err := loadScenes(ctx, load)
	if err != nil {
		return Summary{}, err
	}
	runs := c.Experiment(ctx, scenes, haera, hapsho)
	sum := Summarize(runs, c.weights)
	sum.Used = c.used
	for _, sc := range scenes {
		sum.Scenes = append(sum.Scenes, fmt.Sprintf("%s/%s (acts %d, beats %d)", sc.Work, sc.S.ID, acts(sc.S), beatsOf(sc.S)))
	}
	if err := writeJSON(filepath.Join(c.out, "runs.json"), runs); err != nil {
		return sum, err
	}
	if err := writeJSON(filepath.Join(c.out, "summary.json"), sum); err != nil {
		return sum, err
	}
	return sum, os.WriteFile(filepath.Join(c.out, "table.md"), []byte(Table(sum)), 0o644)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func main() {
	out := flag.String("out", "examples/renderer-ab", "output dir")
	budget := flag.Int("budget", 119, "max real calls")
	load := flag.Bool("load", true, "load the works into PG first (go run ./cmd/dieg load)")
	styles := flag.String("styles", "styles", "backend dir")
	ws := flag.String("weights", "claude-sonnet-5-5=3:15,claude-haiku-5-5=1:5", "relative token weights model=in:out")
	flag.Parse()
	w, err := parseWeights(*ws)
	if err != nil {
		fmt.Fprintln(os.Stderr, "renderer-ab:", err)
		os.Exit(2)
	}
	c := &cfg{out: *out, budget: *budget, weights: w,
		mk: func(m, e string) render.Completer { return render.NewClaudeCLIWith(m, e) }}
	sum, err := run(context.Background(), c, *styles, *load)
	fmt.Print(Table(sum))
	if err != nil {
		fmt.Fprintln(os.Stderr, "renderer-ab:", err)
		os.Exit(1)
	}
}
