package write

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"diegesis/internal/render"
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
	RenderEffort                         string   // claude --effort for render calls (default medium)
	Digest                               string   // ContinueLoop only: the PG state digest of the story so far
	Verifier                             string   // "card" (default, render.ScoreCard checklist) or "open" (open re-extraction)
}

func DefaultOpts() Opts {
	return Opts{Rounds: 4, Backend: "ko-haera", Render: true, Skeleton: true, CodeEffort: "medium", RenderEffort: "medium", Verifier: "card",
		CodeModel: "claude-opus-5-5", RenderModel: "claude-haiku-5-5", ExtractModel: "claude-sonnet-5-5"}
}

// Result: Rounds used, whether the last code is clean, its diagnostics (the blocking ones and any info),
// and Dir = <out>/story holding the last well-formed code.
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
// diagnostic and consumes its round; the next prompt shows the last well-formed bundle. The last well-formed
// code is copied to <out>/story/ either way and <out>/diags.txt holds that code's diagnostics (a later malformed
// round does not overwrite it; with no well-formed round it holds the last bundle diagnostic). Loop never renders.
func Loop(ctx context.Context, c render.Completer, check Checker, idea string, refs Refs, o Opts) (Result, error) {
	if o.Rounds <= 0 {
		o.Rounds = DefaultOpts().Rounds
	}
	if o.Out == "" {
		return Result{}, fmt.Errorf("write: no output dir")
	}
	res := Result{Dir: filepath.Join(o.Out, "story")}
	var last map[string]string // last well-formed bundle
	var lastDiags []Diag       // its diagnostics
	var bundle string
	user := CodePrompt(idea, refs, o.Reserved, o.Skeleton)
	for r := 1; r <= o.Rounds; r++ {
		l := "code-r" + strconv.Itoa(r)
		if r > 1 {
			l = "fix-r" + strconv.Itoa(r)
		}
		out, err := label(c, l).Complete(ctx, System, user)
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
			last, lastDiags = files, res.Diags
			if bundle, err = PackFiles(rdir); err != nil {
				return res, err
			}
			res.Clean = true
			for _, d := range res.Diags {
				res.Clean = res.Clean && !d.Blocking()
			}
		}
		if res.Clean {
			break
		}
		user = FixPrompt(idea, refs, o.Reserved, o.Skeleton, bundle, blocking(res.Diags))
	}
	if last != nil {
		if err := WriteFiles(res.Dir, last); err != nil {
			return res, err
		}
	}
	if last == nil {
		lastDiags = res.Diags
	}
	var sb strings.Builder
	for _, d := range lastDiags {
		sb.WriteString(d.String() + "\n")
	}
	return res, os.WriteFile(filepath.Join(o.Out, "diags.txt"), []byte(sb.String()), 0o644)
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
