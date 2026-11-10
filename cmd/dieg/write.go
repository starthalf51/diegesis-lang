package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
	"github.com/starthalf51/diegesis-lang/internal/write"
	"github.com/starthalf51/diegesis-lang/rules"
)

// corpusWorks are the shipped corpus work ids; a generated work may not take one (loading it would overwrite
// the corpus work in the shared DB). Dirs under ./corpus are added when present.
func corpusWorks() []string {
	set := map[string]bool{}
	for _, w := range []string{"bible", "chibi", "hamlet", "holmes", "macbeth", "odyssey", "othello"} {
		set[w] = true
	}
	if es, err := os.ReadDir("corpus"); err == nil {
		for _, e := range es {
			if e.IsDir() {
				set[e.Name()] = true
			}
		}
	}
	var out []string
	for w := range set {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// writeChecker compiles dir; when that has no errors it loads the work into PG and lints it. Waived diags are dropped,
// paths are made relative to dir. A work id taken from the corpus is a "bundle" error and nothing is loaded.
func writeChecker(reserved []string) write.Checker {
	return func(ctx context.Context, dir string) ([]write.Diag, error) {
		w, cds, err := compile(dir)
		if err != nil {
			return nil, err
		}
		var ds []write.Diag
		failed := false
		for _, d := range cds {
			if d.Waived {
				continue
			}
			f := d.File
			if rel, err := filepath.Rel(dir, filepath.FromSlash(f)); err == nil {
				f = filepath.ToSlash(rel)
			}
			ds = append(ds, write.Diag{File: f, Line: d.Line, Col: d.Col, Severity: d.Severity, Rule: d.Rule, Msg: d.Msg})
			failed = failed || d.Severity == "error"
		}
		if failed {
			return ds, nil
		}
		if len(w.Works) == 0 {
			return append(ds, write.Diag{Severity: "error", Rule: "bundle", Msg: "no `work` declaration"}), nil
		}
		id := w.Works[0].ID
		for _, r := range reserved {
			if id == r {
				return append(ds, write.Diag{Severity: "error", Rule: "bundle",
					Msg: fmt.Sprintf("work id %q belongs to a corpus work; pick a new id", id)}), nil
			}
		}
		if err := migrate(io.Discard); err != nil {
			return nil, err
		}
		pool, err := pg.Connect(ctx)
		if err != nil {
			return nil, err
		}
		defer pool.Close()
		if err := pg.Load(ctx, pool, w); err != nil {
			return nil, err
		}
		lds, err := pg.Lint(ctx, pool, id, rules.FS)
		if err != nil {
			return nil, err
		}
		for _, d := range lds {
			if !d.Waived {
				ds = append(ds, write.Diag{File: d.File, Line: d.Line, Col: d.Col, Severity: d.Severity, Rule: d.Rule, Msg: d.Message})
			}
		}
		return ds, nil
	}
}

// shownScenes loads dir and returns its work id and the render requests of its shown scenes in discourse order.
func shownScenes(ctx context.Context, dir string) (string, []render.Scene, error) {
	work, err := load(dir, io.Discard)
	if err != nil {
		return "", nil, err
	}
	pool, err := pg.Connect(ctx)
	if err != nil {
		return "", nil, err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT id FROM scene WHERE work_id = $1 AND shown ORDER BY discourse_ord, id`, work)
	if err != nil {
		return "", nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return "", nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	var ss []render.Scene
	for _, id := range ids {
		b, err := pg.RenderScene(ctx, pool, work, id)
		if err != nil {
			return "", nil, err
		}
		s, err := render.ParseScene(b)
		if err != nil {
			return "", nil, fmt.Errorf("%s %s: render request: %w", work, id, err)
		}
		ss = append(ss, s)
	}
	return work, ss, nil
}

// counter counts the calls that actually reach the model, to check calls.jsonl against.
type counter struct {
	c render.Completer
	n *atomic.Int64
}

func (k *counter) Complete(ctx context.Context, system, user string) (render.Result, error) {
	// DIEG_CALL_CAP=N: a hard stop for benchmark drivers; call N+1 and later fail before reaching the model.
	n, c := k.n.Add(1), int64(0) // add first, then check: concurrent callers cannot both pass the last slot
	if c, _ = strconv.ParseInt(os.Getenv("DIEG_CALL_CAP"), 10, 64); c > 0 && n > c {
		k.n.Add(-1)
		return render.Result{}, fmt.Errorf("DIEG_CALL_CAP=%d reached", c)
	}
	return k.c.Complete(ctx, system, user)
}

// cmdWrite runs `dieg write <idea.txt | "item sentence"> -o <dir> [--rounds N] [--backend id] [--no-render] [--allow-warn] [--scenes N] [--styles dir] [--code-model M] [--render-model M] [--extract-model M] [--code-effort low|medium|high] [--render-effort low|medium|high] [--force]`.
// Exit 1 when the code is not clean after the rounds (nothing rendered), 3 when a rendered scene failed verification.
// --scenes N renders only the first N shown scenes (to fit a call budget).
func cmdWrite(args []string) int {
	o := write.DefaultOpts()
	var idea, styles = "", "styles"
	limit, force := 0, false
	for i := 0; i < len(args); i++ {
		j, ok, code := commonFlag(args, i, &o, &styles, &limit)
		switch a := args[i]; {
		case code != 0:
			return code
		case ok:
			i = j
		case a == "--force":
			force = true
		case (a == "-o") && i+1 < len(args):
			i++
			o.Out = args[i]
		case idea == "" && !strings.HasPrefix(a, "-"):
			idea = a
		default:
			return usage()
		}
	}
	if idea == "" || o.Out == "" {
		return usage()
	}
	if b, err := os.ReadFile(idea); err == nil {
		idea = string(b)
	}
	code, err := runWrite(idea, styles, limit, o, force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return code
}

// commonFlag parses the write/continue flag at args[i]: ok=false when it is not one, code≠0 (usage 2) for a bad
// value, else the index of its last token. Bad models and efforts exit 2 before any call.
func commonFlag(args []string, i int, o *write.Opts, styles *string, limit *int) (int, bool, int) {
	a := args[i]
	switch {
	case a == "--no-digest":
		o.NoDigest = true
	case a == "--no-render":
		o.Render = false
	case a == "--allow-warn":
		o.AllowWarn = true
	case a == "--skeleton" || a == "--skeleton=true" || a == "--skeleton=false":
		o.Skeleton = a != "--skeleton=false"
	case (a == "--rounds" || a == "--scenes") && i+1 < len(args):
		n, err := strconv.Atoi(args[i+1])
		if err != nil || n < 1 {
			return i, false, usage()
		}
		*map[string]*int{"--rounds": &o.Rounds, "--scenes": limit}[a] = n
		return i + 1, true, 0
	case (a == "--code-model" || a == "--render-model" || a == "--extract-model") && i+1 < len(args):
		if err := render.ValidModel(args[i+1]); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return i, false, 2
		}
		*map[string]*string{"--code-model": &o.CodeModel, "--render-model": &o.RenderModel, "--extract-model": &o.ExtractModel}[a] = args[i+1]
		return i + 1, true, 0
	case (a == "--code-effort" || a == "--render-effort") && i+1 < len(args):
		if e := args[i+1]; e != "low" && e != "medium" && e != "high" {
			fmt.Fprintf(os.Stderr, "dieg: %s %q: want low, medium or high\n", a, e)
			return i, false, 2
		}
		*map[string]*string{"--code-effort": &o.CodeEffort, "--render-effort": &o.RenderEffort}[a] = args[i+1]
		return i + 1, true, 0
	case a == "--verifier" && i+1 < len(args):
		if v := args[i+1]; v != "card" && v != "open" {
			fmt.Fprintf(os.Stderr, "dieg: --verifier %q: want card or open\n", v)
			return i, false, 2
		}
		o.Verifier = args[i+1]
		return i + 1, true, 0
	case (a == "--backend" || a == "--styles") && i+1 < len(args):
		*map[string]*string{"--backend": &o.Backend, "--styles": styles}[a] = args[i+1]
		return i + 1, true, 0
	default:
		return i, false, 0
	}
	return i, true, 0
}

// meters builds the metered completers: coding on o.CodeModel at o.CodeEffort, render and extract on their own
// models, all logged into one Meter writing f and counted in one counter.
func meters(o write.Opts, f io.Writer) (*render.Meter, render.Completers, *counter) {
	k := &counter{c: render.NewCompleter(o.CodeModel, o.CodeEffort), n: new(atomic.Int64)}
	m := render.NewMeter(k, f)
	cs := render.Completers{
		Render:  m.With(&counter{c: render.NewCompleter(o.RenderModel, o.RenderEffort), n: k.n}),
		Extract: m.With(&counter{c: render.NewCompleter(o.ExtractModel, "low"), n: k.n}),
	}
	return m, cs, k
}

// summaryLine prints "<cmd>: rounds=… clean=… scenes=… failed=… calls=… tokens …".
func summaryLine(cmd string, m *render.Meter, k *counter, out string) func(write.Result, int, []string) {
	return func(res write.Result, scenes int, failed []string) {
		t := m.Total()
		fmt.Printf("%s: rounds=%d clean=%t scenes=%d failed=%d%v calls=%d tokens in=%d out=%d cache_read=%d cache_write=%d → %s\n",
			cmd, res.Rounds, res.Clean, scenes, len(failed), failed, k.n.Load(), t.Input, t.Output, t.CacheRead, t.CacheWrite, out)
	}
}

func runWrite(idea, styles string, limit int, o write.Opts, force bool) (int, error) {
	b, err := backend(styles, o.Backend) // fail before spending calls
	if err != nil {
		return 1, err
	}
	refs, err := write.LoadRefs()
	if err != nil {
		return 1, err
	}
	if err := notUnderCorpus(o.Out); err != nil {
		return 1, err
	}
	if _, err := os.Stat(filepath.Join(o.Out, "story")); err == nil && !force {
		return 1, fmt.Errorf("%s/story already exists: write would delete it (all episodes, calls.jsonl). Use dieg continue / dieg revise on it, or pass --force to start over", o.Out)
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return 1, err
	}
	if err := write.ResetRun(o.Out); err != nil {
		return 1, err
	}
	f, err := os.Create(filepath.Join(o.Out, "calls.jsonl"))
	if err != nil {
		return 1, err
	}
	defer f.Close()
	m, cs, k := meters(o, f)
	o.Reserved = corpusWorks()
	ctx := context.Background()
	summary := summaryLine("write", m, k, o.Out)
	res, err := write.Loop(ctx, m, writeChecker(o.Reserved), idea, refs, o)
	if err != nil {
		summary(res, 0, nil)
		return 1, err
	}
	if code, done := uncleanOrNoRender(res, o, summary); done {
		return code, nil
	}
	work, ss, err := shownScenes(ctx, res.Dir)
	if err != nil {
		summary(res, 0, nil)
		return 1, err
	}
	if limit > 0 && limit < len(ss) {
		ss = ss[:limit]
	}
	o, closeDigest, err := withDigest(ctx, work, o)
	if err != nil {
		summary(res, 0, nil)
		return 1, err
	}
	defer closeDigest()
	return renderAll(ctx, cs, work, ss, b, o, res, summary, nil)
}

// uncleanOrNoRender ends a run that is not clean (exit 1, blocking diags on stderr) or not rendering (exit 0).
func uncleanOrNoRender(res write.Result, o write.Opts, summary func(write.Result, int, []string)) (int, bool) {
	if !res.Clean {
		for _, d := range res.Diags {
			if d.Blocking() {
				fmt.Fprintln(os.Stderr, d)
			}
		}
		summary(res, 0, nil)
		return 1, true
	}
	if !o.Render {
		summary(res, 0, nil)
		return 0, true
	}
	return 0, false
}

// renderAll renders ss into o.Out, reports each scene on stderr, and when all is non-nil re-concatenates novel.md
// over all (every shown scene, discourse order). Exit 3 when a scene failed verification.
func renderAll(ctx context.Context, cs render.Completers, work string, ss []render.Scene, b render.Backend, o write.Opts,
	res write.Result, summary func(write.Result, int, []string), all []render.Scene) (int, error) {
	rs, err := write.Novel(ctx, cs, work, ss, b, o.Out, o)
	if err == nil && all != nil {
		err = write.Concat(o.Out, all, b.ID)
	}
	if err != nil {
		summary(res, len(rs), nil)
		return 1, err
	}
	var failed []string
	for _, r := range rs {
		if !r.Pass {
			failed = append(failed, r.ID)
		}
		fmt.Fprintf(os.Stderr, "%s: attempts=%d pass=%t missing=%v hallucinated=%v leaked=%v %s\n", r.ID, r.Attempts, r.Pass,
			r.Report.Missing, r.Report.Hallucinated, r.Report.Leaked, r.Report.ExtractError)
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "dieg: %s: %v\n", r.ID, r.Err)
		}
	}
	summary(res, len(rs), failed)
	if len(failed) > 0 {
		return 3, nil
	}
	return 0, nil
}

// notUnderCorpus refuses an output dir inside ./corpus, where generated story files would compile into a corpus work.
func notUnderCorpus(out string) error {
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	corpus, err := filepath.Abs("corpus")
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(corpus, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("-o %q is under the corpus directory; pick another output dir", out)
	}
	return nil
}

// withDigest wires o.Digest to a PG pool (pg.Digest for work) unless --no-digest; the returned func closes the pool.
func withDigest(ctx context.Context, work string, o write.Opts) (write.Opts, func(), error) {
	if o.NoDigest {
		return o, func() {}, nil
	}
	pool, err := pg.Connect(ctx)
	if err != nil {
		return o, nil, err
	}
	o.Digest = func(ctx context.Context, sceneID string) (string, string, error) { return pg.Digest(ctx, pool, work, sceneID) }
	return o, pool.Close, nil
}
