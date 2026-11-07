// serial benchmarks a 12-episode serial (2 scenes each, 2 revision requests) between A (the model writes it
// directly: outline, per-episode write + summary update, revisions by re-writing picked episodes) and B
// (`dieg write` ep1, `dieg continue` ep2.., `dieg revise`). Both arms get the same per-episode briefs from one
// shared `plan` call (not charged to either arm). Task 5 only builds and dry-runs it.
//
//	go run ./tools/serial -idea f.txt -out examples/bench/serial [-eps 12] [-rev-after 4,8] [-budget 220] [-dry]
//
// Output: plan/ a/ b/ judge/ metrics.csv summary.json. Run from the repo root.
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
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

type cfg struct {
	idea, out string
	eps       int
	revAfter  []int
	budget    int
	a, judge  render.Completer // a also serves the shared plan call
}

const perEp = 2 // scenes per episode (fixed by the design)

// runDieg runs `dieg <args...>` from the repo root, logging to <out>/b.log. A swap point for -dry. B shells out
// to the real CLI (not the internal functions): it measures exactly what a user runs, calls.jsonl is written by
// the CLI itself, and the harness cannot drift from the CLI's flags.
var runDieg = func(ctx context.Context, out string, args []string) error {
	log, err := os.OpenFile(filepath.Join(out, "b.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/dieg"}, args...)...)
	cmd.Stdout, cmd.Stderr = io.MultiWriter(log, os.Stdout), io.MultiWriter(log, os.Stderr)
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 { // 3 = some scene failed verify: keep going
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

// Totals sums one calls.jsonl; a missing file is zero.
type Totals struct {
	render.Usage
	Calls, Errs int
}

// In is the whole prompt: the claude CLI puts most of it in the cache fields.
func In(u render.Usage) int64 { return u.Input + u.CacheRead + u.CacheWrite }

// Weighted is the cache-weighted input estimate (reads 0.1, writes 1.25), as in Plan 3.
func Weighted(u render.Usage) int64 {
	return int64(math.Round(float64(u.Input) + 0.1*float64(u.CacheRead) + 1.25*float64(u.CacheWrite)))
}

func ReadCalls(path string) (Totals, error) {
	var t Totals
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
			Err string
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return t, fmt.Errorf("%s: %w", path, err)
		}
		t.Calls++
		if r.Err != "" {
			t.Errs++
		}
		t.Input += r.Input
		t.Output += r.Output
		t.CacheRead += r.CacheRead
		t.CacheWrite += r.CacheWrite
	}
	return t, sc.Err()
}

// RevCost is one revision's cost for one arm.
type RevCost struct {
	Calls    int   `json:"calls"`
	Weighted int64 `json:"cache_weighted"`
}

// tracker records one CSV row per step (cumulative per arm), per-revision deltas, and enforces the call budget.
type tracker struct {
	c    cfg
	rows [][]string
	revs map[string][]RevCost // "A"/"B" -> revision k
}

func (t *tracker) totals(arm string) Totals {
	x, _ := ReadCalls(filepath.Join(t.c.out, strings.ToLower(arm), "calls.jsonl"))
	return x
}

// check fails once the calls of plan, a, b and judge together reach the budget.
func (t *tracker) check() error {
	n := 0
	for _, d := range []string{"plan", "a", "b", "judge"} {
		x, err := ReadCalls(filepath.Join(t.c.out, d, "calls.jsonl"))
		if err != nil {
			return err
		}
		n += x.Calls
	}
	if n >= t.c.budget {
		return fmt.Errorf("call budget %d reached (%d used)", t.c.budget, n)
	}
	return nil
}

func (t *tracker) record(arm, step string) {
	x := t.totals(arm)
	t.rows = append(t.rows, []string{step, arm, strconv.Itoa(x.Calls), fmt.Sprint(In(x.Usage)), fmt.Sprint(Weighted(x.Usage)), fmt.Sprint(x.Output)})
}

// revStep runs f (one revision) and records its cost delta.
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

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serial", flag.ContinueOnError)
	idea := fs.String("idea", "", "idea file")
	out := fs.String("out", "", "output dir")
	eps := fs.Int("eps", 12, "episodes")
	revAfter := fs.String("rev-after", "4,8", "revision after these episodes (max 2)")
	budget := fs.Int("budget", 220, "abort when total calls reach this")
	dry := fs.Bool("dry", false, "fake every model call and the dieg subprocess")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text, err := os.ReadFile(*idea)
	if err != nil || *out == "" {
		return fmt.Errorf("need -idea and -out (%v)", err)
	}
	// A and the plan use claude-opus-5-5 at medium, B's code effort, so the effort lever is not B's alone.
	c := cfg{idea: string(text), out: *out, eps: *eps, budget: *budget,
		a: render.NewClaudeCLIWith("claude-opus-5-5", "medium"), judge: render.NewClaudeCLI()}
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
	}
	return runAll(ctx, c)
}

func runAll(ctx context.Context, c cfg) error {
	t := &tracker{c: c, revs: map[string][]RevCost{}}
	briefs, err := runPlan(ctx, t)
	if err != nil {
		return err
	}
	if err := runA(ctx, t, briefs); err != nil {
		return err
	}
	if err := runB(ctx, t, briefs); err != nil {
		return err
	}
	res, err := runJudge(ctx, t)
	if err != nil {
		return err
	}
	return writeMetrics(t, res)
}

type armTotals struct {
	Calls, Errs               int
	RealTokens, CacheWeighted int64 // RealTokens = input + cache_read + cache_write
	Output                    int64
}

func sum(x Totals) armTotals {
	return armTotals{x.Calls, x.Errs, In(x.Usage), Weighted(x.Usage), x.Output}
}

func writeMetrics(t *tracker, res judgeResult) error {
	f, err := os.Create(filepath.Join(t.c.out, "metrics.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write([]string{"step", "arm", "calls", "input_real", "cache_weighted", "output"})
	w.WriteAll(t.rows)
	if err := w.Error(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	type rev struct {
		AfterEp              int `json:"after_ep"`
		A, B                 RevCost
		AResidual, BResidual int // episodes the judge says the revision is not applied in (unverified; Task 6 checks quotes)
	}
	var revs []rev
	for k, after := range t.c.revAfter {
		revs = append(revs, rev{after, t.revs["A"][k], t.revs["B"][k], res.Residual["A"][k+1], res.Residual["B"][k+1]})
	}
	b, err := json.MarshalIndent(map[string]any{
		"episodes": t.c.eps, "scenes_per_episode": perEp, "rev_after": t.c.revAfter,
		"plan_uncharged": sum(t.totals("plan")), "A": sum(t.totals("a")), "B": sum(t.totals("b")), "judge": sum(t.totals("judge")),
		"revisions": revs,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(t.c.out, "summary.json"), b, 0o644)
}
