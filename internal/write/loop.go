package write

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// Diag is one compiler or lint diagnostic, File relative to the bundle root. Waived lints are not Diags.
type Diag struct {
	File      string
	Line, Col int
	Severity  string
	Rule, Msg string
}

func (d Diag) String() string {
	f := d.File
	if f == "" {
		f = "-"
	}
	return fmt.Sprintf("%s:%d:%d: %s %s: %s", f, d.Line, d.Col, d.Severity, d.Rule, d.Msg)
}

// Blocking says whether d keeps the code from being clean: errors and warns do, info does not.
func (d Diag) Blocking() bool { return d.Severity == "error" || d.Severity == "warn" }

// Checker compiles and lints the bundle in dir.
type Checker func(ctx context.Context, dir string) (diags []Diag, err error)

type Opts struct {
	Rounds                               int
	Backend                              string
	Render                               bool // not used by Loop (it never renders); the CLI reads it
	Out                                  string
	Reserved                             []string // work ids the model must not take (corpus works); listed in the prompt
	CodeModel, RenderModel, ExtractModel string   // model per role; DefaultOpts fills them
	Skeleton                             bool     // code carries events only; the renderer writes the dialogue
	CodeEffort                           string   // claude --effort for coding calls: low, medium or high
	RenderEffort                         string   // claude --effort for render calls (default low)
	NoDigest                             bool     // CLI: --no-digest, leave o.Digest nil
	Recap                                string   // ContinueLoop only: the PG state digest of the story so far (StateDigest)
	Digest                               func(ctx context.Context, sceneID string) (text, state string, err error) // render path: per-scene PG world-state digest; text is appended to the render prompt, only the state-only part is hashed for staleness; nil = none
	Skip                                 []string // Rerender: scene ids never rendered even if stale (CLI: revise --skip)
	Verifier                             string   // "card" (default, render.ScoreCard checklist) or "open" (open re-extraction)
	AllowWarn                            bool     // Loop: warn diagnostics do not block and are not fed back (errors still do)
	Finish                               render.Completer     // Loop: when the Rounds end unclean, this model continues from the best round (nil = none)
	FinishRounds                         int                  // Loop: finish rounds, 0 = 2
	FinishModel                          string               // CLI: --finish-model; the CLI builds Finish from it
	Coverage                             func(dir string) int // Loop: how much of the source a round's code covers; when set, an error-free round wins, then the higher coverage, so dropping scenes cannot buy fewer errors
}

func DefaultOpts() Opts {
	return Opts{Rounds: 4, Backend: "ko-haera", Render: true, Skeleton: true, CodeEffort: "medium", RenderEffort: "low", Verifier: "card",
		CodeModel: "claude-opus-5-5", RenderModel: "claude-haiku-5-5", ExtractModel: "claude-sonnet-5-5"}
}

// Result: Rounds used, whether the best code is clean, its diagnostics (the blocking ones and any info),
// and Dir = <out>/story holding the best round's code (fewest errors, then warns; the earlier round on ties).
type Result struct {
	Rounds int
	Clean  bool
	Diags  []Diag
	Dir    string
	Scenes []string // ContinueLoop only: scene ids the accepted episode files declare, in file order
}

// label tags c's calls when c is a Meter.
func label(c render.Completer, l string) render.Completer {
	if m, ok := c.(*render.Meter); ok {
		return m.Label(l)
	}
	return c
}

func bundleDiag(msg string) []Diag {
	return []Diag{{Severity: "error", Rule: "bundle", Msg: msg}}
}

// Loop asks c for code, checks it, and feeds diagnostics back until clean or o.Rounds is spent. Round r's
// raw output is <out>/rounds/r<r>.raw.txt and its code <out>/rounds/r<r>/. A malformed bundle is one "bundle"
// diagnostic and consumes its round; the next prompt shows the last well-formed bundle. The best well-formed
// round (fewest errors, then warns; the earlier round on ties) is copied to <out>/story/ either way and
// <out>/diags.txt holds its diagnostics (a later worse or malformed round does not overwrite it; with no well-formed round it holds the last bundle diagnostic). Loop never renders.
func Loop(ctx context.Context, c render.Completer, check Checker, idea string, refs Refs, o Opts) (Result, error) {
	if o.Rounds <= 0 {
		o.Rounds = DefaultOpts().Rounds
	}
	if o.Out == "" {
		return Result{}, fmt.Errorf("write: no output dir")
	}
	res := Result{Dir: filepath.Join(o.Out, "story")}
	// blk: the diagnostics that keep a round from being clean; with AllowWarn only errors.
	blk := func(ds []Diag) (out []Diag) {
		for _, d := range blocking(ds) {
			if d.Severity == "error" || !o.AllowWarn {
				out = append(out, d)
			}
		}
		return
	}
	var best map[string]string // best well-formed bundle: fewest errors, then fewest warns, the earlier round on a tie
	var bestDiags []Diag       // its diagnostics
	var bestE, bestW, bestC int
	var bundle string
	var bestDir string // the round directory holding best
	fr := 0
	if o.Finish != nil {
		if fr = o.FinishRounds; fr <= 0 {
			fr = 2
		}
	}
	user := CodePrompt(idea, refs, o.Reserved, o.Skeleton)
	for r := 1; r <= o.Rounds+fr; r++ {
		cc, l := c, "code-r"+strconv.Itoa(r)
		if r > 1 {
			l = "fix-r" + strconv.Itoa(r)
		}
		if r > o.Rounds { // finish rounds: another model continues, from the best round's code
			cc, l = o.Finish, "finish-r"+strconv.Itoa(r-o.Rounds)
			if r == o.Rounds+1 && bestDir != "" {
				if b, err := PackFiles(bestDir); err == nil {
					user = FixPrompt(idea, refs, o.Reserved, o.Skeleton, b, GroupDiags(blk(bestDiags)))
				}
			}
		}
		out, err := label(cc, l).Complete(ctx, System, user)
		if err != nil {
			return res, fmt.Errorf("round %d: %w", r, err)
		}
		res.Rounds = r
		rdir := filepath.Join(o.Out, "rounds", "r"+strconv.Itoa(r))
		if err := os.MkdirAll(filepath.Dir(rdir), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(rdir+".raw.txt", []byte(out.Text), 0o644); err != nil {
			return res, err
		}
		files, perr := ParseFiles(out.Text)
		switch {
		case out.StopReason != "end_turn":
			res.Diags = bundleDiag(fmt.Sprintf("model output stopped early (%s); write every file and end with %s", out.StopReason, End))
		case perr != nil:
			res.Diags = bundleDiag(perr.Error())
		default:
			if err := WriteFiles(rdir, files); err != nil {
				return res, err
			}
			if res.Diags, err = check(ctx, rdir); err != nil {
				return res, fmt.Errorf("round %d check: %w", r, err)
			}
			if bundle, err = PackFiles(rdir); err != nil {
				return res, err
			}
			e, w := 0, 0
			for _, d := range res.Diags {
				switch d.Severity {
				case "error":
					e++
				case "warn":
					w++
				}
			}
			cov, better := 0, e < bestE || e == bestE && w < bestW
			if o.Coverage != nil {
				cov = o.Coverage(rdir)
				if (e == 0) != (bestE == 0) {
					better = e == 0
				} else if cov != bestC {
					better = cov > bestC
				}
			}
			if best == nil || better {
				best, bestDiags, bestE, bestW, bestC, bestDir = files, res.Diags, e, w, cov, rdir
			}
			res.Clean = len(blk(res.Diags)) == 0
		}
		if res.Clean {
			break
		}
		user = FixPrompt(idea, refs, o.Reserved, o.Skeleton, bundle, GroupDiags(blk(res.Diags)))
	}
	if best != nil {
		if err := WriteFiles(res.Dir, best); err != nil {
			return res, err
		}
		res.Diags, res.Clean = bestDiags, len(blk(bestDiags)) == 0
	}
	var sb strings.Builder
	for _, d := range res.Diags {
		sb.WriteString(d.String() + "\n")
	}
	return res, os.WriteFile(filepath.Join(o.Out, "diags.txt"), []byte(sb.String()), 0o644)
}

// GroupDiags collapses diagnostics with the same rule and message into one, in first-seen order: the first one's
// position, Msg suffixed with the count and that position. A weak model then sees root causes, not 18 copies.
func GroupDiags(ds []Diag) []Diag {
	type key struct{ rule, msg string }
	idx, n := map[key]int{}, map[key]int{}
	var out []Diag
	for _, d := range ds {
		k := key{d.Rule, d.Msg}
		if _, ok := idx[k]; !ok {
			idx[k] = len(out)
			out = append(out, d)
		}
		n[k]++
	}
	for k, i := range idx {
		if n[k] > 1 {
			d := out[i]
			out[i].Msg = fmt.Sprintf("%s (%d places; first at %s:%d:%d)", d.Msg, n[k], d.File, d.Line, d.Col)
		}
	}
	return out
}

func blocking(ds []Diag) []Diag {
	var out []Diag
	for _, d := range ds {
		if d.Blocking() {
			out = append(out, d)
		}
	}
	return out
}

var (
	actRef = regexp.MustCompile(`#a\d+`)
	posRef = regexp.MustCompile(`:\d+(:\d+)?`)
)

// diagKey identifies a diagnostic across edits: rule, file and message with act numbers (#a3) and line:col positions
// blanked, since inserting an act or a line renumbers them.
func diagKey(d Diag) string {
	return d.Rule + "|" + d.File + "|" + posRef.ReplaceAllString(actRef.ReplaceAllString(d.Msg, "#a"), "")
}

// baseline lints dir as it is (no model call) and returns fresh: the blocking diagnostics of a later check that the
// untouched story did not already have. The baseline is a multiset on diagKey, so a second identical warn is new.
func baseline(ctx context.Context, check Checker, dir string) (fresh func([]Diag) []Diag, err error) {
	base, err := check(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("baseline check: %w", err)
	}
	return func(ds []Diag) (out []Diag) {
		n := map[string]int{}
		for _, d := range base {
			n[diagKey(d)]++
		}
		for _, d := range ds {
			if !d.Blocking() {
				continue
			}
			if k := diagKey(d); n[k] > 0 {
				n[k]--
			} else {
				out = append(out, d)
			}
		}
		return
	}, nil
}
