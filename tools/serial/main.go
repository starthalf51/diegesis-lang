// serial benchmarks a 12-episode serial (2 scenes each, 2 revision requests) between A (the model writes it
// directly: outline, per-episode write + summary update, revisions by re-writing picked episodes) and B
// (`dieg write` ep1, `dieg continue` ep2.., `dieg revise`). Both arms get the same per-episode briefs from one
// shared `plan` call (not charged to either arm). The arms run interleaved (A epN, B epN, then any revision
// on both); when the call budget trips, both stop at the same last completed episode and that prefix is judged.
//
//	go run ./tools/serial -idea f.txt -out examples/bench/serial [-eps 12] [-rev-after 4,8] [-budget 220]
//	    [-price claude-opus-5-5=5:25,claude-sonnet-5-5=3:15] [-dry]
//
// Output: plan/ a/ b/ judge/ metrics.csv summary.json dieg.exe. Run from the repo root.
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"diegesis/internal/render"
)

// System is Plan 3's fixed system prompt (tools/bench), used by plan, A and the judge.
const System = "You are a careful Korean novelist and story engineer. Follow the user's instructions exactly. Output only what is asked."

// revReqs are the two fixed revision requests: a character exit/replacement, then a change of the secret's truth.
var revReqs = []string{
	"주인공을 돕는 조연 한 명을 이야기에서 퇴장시키고(떠나거나 사라진다), 그 역할을 새 인물 한 명이 맡게 바꿔라. 퇴장한 인물은 이후 화에 나오면 안 되고, 앞 화의 해당 장면도 새 인물 기준으로 일관되게 고쳐라.",
	"지금까지 독자에게 제시된 비밀의 진상을 다른 진상으로 바꿔라. 새 진상은 앞 화에서 이미 보여 준 사실과 모순되지 않아야 하고, 앞 화의 단서·증언·인물의 믿음도 새 진상에 맞게 고쳐라.",
}

const (
	perEp        = 2 // scenes per episode (fixed by the design)
	judgeReserve = 2 // calls kept for the judge (two halves at most)
	// lengthLine goes to both arms (A's prompts, B's idea file and --brief) so length does not skew the cost.
	lengthLine   = "화당 약 5000자, 장면당 약 2500자"
	opus, sonnet, haiku = "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"
)

// stepReserve is the worst case for one dieg step (code rounds + render/extract retries of a few scenes).
var stepReserve = 20

var errBudget = errors.New("call budget reached")

type cfg struct {
	idea, out string
	eps       int
	revAfter  []int
	budget    int
	price     map[string][2]float64 // model -> relative {input, output} weight; nil = per-model totals only
	a, judge  render.Completer      // a also serves the shared plan call
}

// diegBin is the dieg binary built once at start, so its real exit code (3 = a scene failed verify) reaches us
// (`go run` reports 1 for every failure).
var diegBin string

// runDieg runs `dieg <args...>` from the repo root, logging to <out>/b.log. A swap point for -dry.
// B shells out to the real CLI (not the internal functions): it measures exactly what a user runs and
// calls.jsonl is written by the CLI itself.
var runDieg = realDieg

func realDieg(ctx context.Context, out string, args []string) error {
	log, err := os.OpenFile(filepath.Join(out, "b.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, diegBin, args...)
	cmd.Stdout, cmd.Stderr = io.MultiWriter(log, os.Stdout), io.MultiWriter(log, os.Stderr)
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 { // some scene failed verify: keep going
		fmt.Fprintf(log, "serial: dieg %s exit 3\n", args[0])
		return nil
	}
	return err // exit 1 (code not clean / error) aborts: the next episode would build on a broken story
}

func meter(c render.Completer, dir string) (*render.Meter, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return render.NewMeter(c, f), f, nil
}

func write(path, s string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(s)+"\n"), 0o644)
}

// call refuses a non-end_turn result so a truncated episode is not passed on as the next one's context.
func call(ctx context.Context, m *render.Meter, label, user string) (string, error) {
	r, err := m.Label(label).Complete(ctx, System, user)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	if r.StopReason != "end_turn" {
		return r.Text, fmt.Errorf("%s: stop_reason %q", label, r.StopReason)
	}
	return r.Text, nil
}

// MTot is one model's share of a calls.jsonl.
type MTot struct {
	Calls int
	render.Usage
}

// Totals sums one calls.jsonl; a missing file is zero.
type Totals struct {
	render.Usage
	Calls, Errs int
	ByModel     map[string]*MTot
}

// modelOf maps a call label to its model. calls.jsonl records no model, so this assumes dieg's default roles
// (code-/fix- on opus; render- on haiku; extract- on sonnet) and opus for every harness call.
func modelOf(label string) string {
	if strings.HasPrefix(label, "render-") {
		return haiku
	}
	if strings.HasPrefix(label, "extract-") {
		return sonnet
	}
	return opus
}

// In is the whole prompt: the claude CLI puts most of it in the cache fields.
func In(u render.Usage) int64 { return u.Input + u.CacheRead + u.CacheWrite }

// Weighted is the cache-weighted input estimate (reads 0.1, writes 1.25), as in Plan 3.
func Weighted(u render.Usage) int64 {
	return int64(math.Round(float64(u.Input) + 0.1*float64(u.CacheRead) + 1.25*float64(u.CacheWrite)))
}

func ReadCalls(path string) (Totals, error) {
	t := Totals{ByModel: map[string]*MTot{}}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return t, nil
	}
	if err != nil {
		return t, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r struct {
			render.Usage
			Label, Err string
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return t, fmt.Errorf("%s: %w", path, err)
		}
		mo := modelOf(r.Label)
		m := t.ByModel[mo]
		if m == nil {
			m = &MTot{}
			t.ByModel[mo] = m
		}
		t.Calls++
		m.Calls++
		if r.Err != "" {
			t.Errs++
		}
		for _, u := range []*render.Usage{&t.Usage, &m.Usage} {
			u.Input += r.Input
			u.Output += r.Output
			u.CacheRead += r.CacheRead
			u.CacheWrite += r.CacheWrite
		}
	}
	return t, sc.Err()
}

// RevCost is one revision's cost for one arm.
type RevCost struct {
	Calls    int   `json:"calls"`
	Weighted int64 `json:"cache_weighted"`
}

// tracker records one CSV row per step (cumulative per arm), per-revision deltas, B's scene files per episode,
// and enforces the call budget.
type tracker struct {
	c       cfg
	rows    [][]string
	revs    map[string][]RevCost // "A"/"B" -> revision costs, in revision order
	revDone []bool               // revision k applied to both arms
	cut     int                  // last episode completed by both arms
	bScenes map[int][]string     // episode -> its rendered scene files in b/novel/
}

func (t *tracker) totals(arm string) Totals {
	x, _ := ReadCalls(filepath.Join(t.c.out, strings.ToLower(arm), "calls.jsonl"))
	return x
}

// check fails with errBudget unless used calls (plan+a+b+judge) + reserve fit in the budget. A step reserves its own
// worst case plus the judge's calls; the judge itself is exempt (already reserved).
func (t *tracker) check(reserve int) error {
	n := 0
	for _, d := range []string{"plan", "a", "b", "judge"} {
		x, err := ReadCalls(filepath.Join(t.c.out, d, "calls.jsonl"))
		if err != nil {
			return err
		}
		n += x.Calls
	}
	if n+reserve > t.c.budget {
		return fmt.Errorf("%w (%d used + %d reserved of %d)", errBudget, n, reserve, t.c.budget)
	}
	return nil
}

func (t *tracker) record(arm, step string) {
	x := t.totals(arm)
	t.rows = append(t.rows, []string{step, arm, strconv.Itoa(x.Calls), strconv.Itoa(x.Errs), fmt.Sprint(In(x.Usage)), fmt.Sprint(Weighted(x.Usage)), fmt.Sprint(x.Output)})
}

// revStep runs f (one revision of one arm) and records its cost delta.
func (t *tracker) revStep(arm string, k int, f func() error) error {
	before := t.totals(arm)
	if err := f(); err != nil {
		return err
	}
	after := t.totals(arm)
	t.revs[arm] = append(t.revs[arm], RevCost{after.Calls - before.Calls, Weighted(after.Usage) - Weighted(before.Usage)})
	t.record(arm, fmt.Sprintf("rev%d", k))
	return nil
}

// revisionAfter returns the revision index (0-based) that follows episode ep, or -1.
func (c cfg) revisionAfter(ep int) int {
	for k, a := range c.revAfter {
		if a == ep {
			return k
		}
	}
	return -1
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "serial:", err)
		os.Exit(1)
	}
}

// parsePrice reads "model=in:out,model=in:out".
func parsePrice(s string) (map[string][2]float64, error) {
	if s == "" {
		return nil, nil
	}
	p := map[string][2]float64{}
	for _, kv := range strings.Split(s, ",") {
		m, v, ok := strings.Cut(kv, "=")
		i, o, ok2 := strings.Cut(v, ":")
		fi, e1 := strconv.ParseFloat(i, 64)
		fo, e2 := strconv.ParseFloat(o, 64)
		if !ok || !ok2 || e1 != nil || e2 != nil {
			return nil, fmt.Errorf("-price %q: want model=in:out,...", kv)
		}
		p[m] = [2]float64{fi, fo}
	}
	return p, nil
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serial", flag.ContinueOnError)
	idea := fs.String("idea", "", "idea file")
	out := fs.String("out", "", "output dir")
	eps := fs.Int("eps", 12, "episodes")
	revAfter := fs.String("rev-after", "4,8", "revision after these episodes (max 2)")
	budget := fs.Int("budget", 220, "total calls incl. the judge reserve; arms stop at the same episode when it trips")
	price := fs.String("price", "", "relative per-token weights model=in:out,... (optional)")
	dry := fs.Bool("dry", false, "fake every model call and the dieg subprocess")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text, err := os.ReadFile(*idea)
	if err != nil || *out == "" {
		return fmt.Errorf("need -idea and -out (%v)", err)
	}
	// A and the plan use opus at medium, B's code effort, so the effort lever is not B's alone.
	c := cfg{idea: string(text), out: *out, eps: *eps, budget: *budget,
		a: render.NewClaudeCLIWith(opus, "medium"), judge: render.NewClaudeCLI()}
	if c.price, err = parsePrice(*price); err != nil {
		return err
	}
	for _, s := range strings.Split(*revAfter, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 1 || n >= c.eps || len(c.revAfter) >= len(revReqs) {
			return fmt.Errorf("-rev-after %q: want up to %d episode numbers in 1..%d", *revAfter, len(revReqs), c.eps-1)
		}
		c.revAfter = append(c.revAfter, n)
	}
	if *dry {
		c.a, c.judge = fake{c.eps}, fake{c.eps}
		prev := runDieg
		runDieg = fakeDieg
		defer func() { runDieg = prev }()
	} else {
		if err := os.MkdirAll(c.out, 0o755); err != nil {
			return err
		}
		if diegBin, err = filepath.Abs(filepath.Join(c.out, "dieg.exe")); err != nil {
			return err
		}
		build := exec.CommandContext(ctx, "go", "build", "-o", diegBin, "./cmd/dieg")
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("build dieg: %w", err)
		}
	}
	return runAll(ctx, c)
}

// runAll runs plan, then A and B interleaved per episode (revisions applied to both right after their episode),
// writes metrics, judges the common prefix, and writes metrics again with the judge's marks. A budget trip is a
// result, not an error; so is a judge failure (marks become "unknown").
func runAll(ctx context.Context, c cfg) error {
	t := &tracker{c: c, revs: map[string][]RevCost{}, revDone: make([]bool, len(c.revAfter)), bScenes: map[int][]string{}}
	if err := os.MkdirAll(c.out, 0o755); err != nil {
		return err
	}
	briefs, err := runPlan(ctx, t)
	if err != nil {
		return err
	}
	a, err := newArmA(ctx, t, briefs)
	if a != nil {
		defer a.cl.Close()
	}
	if err == nil {
		err = runInterleaved(ctx, t, a, briefs)
	}
	if err != nil && !errors.Is(err, errBudget) {
		writeMetrics(t, nil)
		return err
	}
	stop := ""
	if err != nil {
		stop = err.Error()
	}
	if err := writeMetrics(t, &judgeOutcome{Status: "unknown: not judged yet", stop: stop}); err != nil {
		return err
	}
	res := runJudge(ctx, t)
	res.stop = stop
	return writeMetrics(t, &res)
}

func runInterleaved(ctx context.Context, t *tracker, a *armA, briefs []string) error {
	c := t.c
	for e := 1; e <= c.eps; e++ {
		if err := a.episode(e); err != nil {
			return err
		}
		if err := runBEpisode(ctx, t, briefs, e); err != nil {
			return err
		}
		t.cut = e
		k := c.revisionAfter(e)
		if k < 0 {
			continue
		}
		// The replan needs neither arm's output, so it goes first: if it is blocked or fails, revision k reaches
		// neither arm and the run is cut cleanly at e. briefs is shared with A and B: both see the new directions.
		if err := replan(ctx, t, briefs, k, e); err != nil {
			return err
		}
		commit, err := a.revise(k, e) // stages A's rewrites; applied only if B's revise also succeeds
		if err != nil {
			return err
		}
		if err := runBRevise(ctx, t, k, e); err != nil {
			return err
		}
		commit()
		t.revDone[k] = true
	}
	return nil
}

type modelOut struct {
	Calls         int
	RealTokens    int64
	CacheWeighted int64
	Output        int64
}

type armTotals struct {
	Calls, Errs               int
	RealTokens, CacheWeighted int64 // RealTokens = input + cache_read + cache_write
	Output                    int64
	ByModel                   map[string]modelOut
	PriceWeighted             *float64 `json:",omitempty"` // only with -price
}

func (c cfg) sum(x Totals) armTotals {
	a := armTotals{Calls: x.Calls, Errs: x.Errs, RealTokens: In(x.Usage), CacheWeighted: Weighted(x.Usage), Output: x.Output, ByModel: map[string]modelOut{}}
	pw, priced := 0.0, c.price != nil
	for m, v := range x.ByModel {
		a.ByModel[m] = modelOut{v.Calls, In(v.Usage), Weighted(v.Usage), v.Output}
		if p, ok := c.price[m]; ok {
			pw += float64(Weighted(v.Usage))*p[0] + float64(v.Output)*p[1]
		} else {
			priced = false // a model without a price: no total rather than a wrong one
		}
	}
	if priced {
		a.PriceWeighted = &pw
	}
	return a
}

func writeMetrics(t *tracker, j *judgeOutcome) error {
	c := t.c
	f, err := os.Create(filepath.Join(c.out, "metrics.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write([]string{"step", "arm", "calls", "errors", "input_real", "cache_weighted", "output"})
	w.WriteAll(t.rows)
	if err := w.Error(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	type rev struct {
		AfterEp   int `json:"after_ep"`
		A, B      RevCost
		AResidual any // episodes the judge marks not-applied (unverified; Task 6 checks quotes); "unknown" if not judged
		BResidual any
	}
	var revs []rev
	var included []int
	for k, after := range c.revAfter {
		if !t.revDone[k] {
			continue
		}
		included = append(included, after)
		revs = append(revs, rev{after, t.revs["A"][k], t.revs["B"][k], j.residual("A", k+1), j.residual("B", k+1)})
	}
	chars := map[string][]int{}
	for e := 1; e <= t.cut; e++ {
		chars["A"] = append(chars["A"], len([]rune(aEpText(c, e))))
		chars["B"] = append(chars["B"], len([]rune(t.bEpText(e))))
	}
	status, stop := "unknown: not judged", ""
	if j != nil {
		status, stop = j.Status, j.stop
	}
	b, err := json.MarshalIndent(map[string]any{
		"episodes": c.eps, "cut_at_episode": t.cut, "stopped_by": stop, "scenes_per_episode": perEp, "rev_after": included,
		"judge_status": status, "length_target": lengthLine, "chars_per_episode": chars, "b_missing_scenes": t.bMissing(),
		"plan_uncharged": c.sum(t.totals("plan")), "A": t.atCut("A"), "B": t.atCut("B"), "A_total_including_cut_work": c.sum(t.totals("a")), "B_total_including_cut_work": c.sum(t.totals("b")), "judge": c.sum(t.totals("judge")),
		"revisions": revs,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.out, "summary.json"), b, 0o644)
}

type cutTotals struct {
	Calls                             int
	RealTokens, CacheWeighted, Output int64
	Errs                              int
}

// atCut is arm's cumulative totals after its last step that counts: outline, episodes 1..cut and the revisions
// applied to both arms. Work for an episode or revision that only one arm finished is left out.
func (t *tracker) atCut(arm string) cutTotals {
	ok := map[string]bool{"outline": true}
	for e := 1; e <= t.cut; e++ {
		ok[fmt.Sprintf("ep%d", e)] = true
	}
	for k, d := range t.revDone {
		ok[fmt.Sprintf("rev%d", k+1)] = d
	}
	var r []string
	for _, row := range t.rows {
		if row[1] == arm && ok[row[0]] {
			r = row
		}
	}
	var c cutTotals
	if r != nil {
		c.Calls, _ = strconv.Atoi(r[2])
		c.Errs, _ = strconv.Atoi(r[3])
		c.RealTokens, _ = strconv.ParseInt(r[4], 10, 64)
		c.CacheWeighted, _ = strconv.ParseInt(r[5], 10, 64)
		c.Output, _ = strconv.ParseInt(r[6], 10, 64)
	}
	return c
}
