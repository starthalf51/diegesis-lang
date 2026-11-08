package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"diegesis/internal/render"
	"diegesis/internal/write"
)

// cmdRevise runs `dieg revise <story-dir> "요청" [--rounds N] [--scenes N] [--no-render] [--backend id] [--styles dir]
// [--code-model M] [--render-model M] [--extract-model M] [--code-effort e]` on a dir made by `dieg write -o <story-dir>`.
// The AI edits story/ to satisfy the request (checker + PG lint loop on a copy; story/ changes only when clean), then only
// the scenes whose render request changed or that are new are re-rendered (--scenes N caps that), removed scenes lose
// their prose files, and novel.md is rebuilt. Exit codes as write.
func cmdRevise(args []string) int {
	o := write.DefaultOpts()
	styles, limit := "styles", 0
	var dir, request string
	for i := 0; i < len(args); i++ {
		j, ok, code := commonFlag(args, i, &o, &styles, &limit)
		switch a := args[i]; {
		case code != 0:
			return code
		case ok:
			i = j
		case dir == "" && !strings.HasPrefix(a, "-"):
			dir = a
		case request == "" && !strings.HasPrefix(a, "--"):
			request = a
		default:
			return usage()
		}
	}
	if dir == "" || strings.TrimSpace(request) == "" {
		return usage()
	}
	o.Out = dir
	code, err := runRevise(request, styles, limit, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return code
}

func runRevise(request, styles string, limit int, o write.Opts) (int, error) {
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
	story := filepath.Join(o.Out, "story")
	if st, err := os.Stat(story); err != nil || !st.IsDir() {
		return 1, fmt.Errorf("%s: no story/ dir (make one with dieg write -o %s)", o.Out, o.Out)
	}
	ctx := context.Background()
	f, err := os.OpenFile(filepath.Join(o.Out, "calls.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 1, err
	}
	defer f.Close()
	m, cs, k := meters(o, f)
	o.Reserved = corpusWorks()
	summary := func(res write.Result, rr write.RerenderResult, failed []string) {
		t, patched := m.Total(), 0
		for _, s := range rr.Scenes {
			if s.Patched {
				patched++
			}
		}
		fmt.Printf("revise: rounds=%d clean=%t changed=%s added=%s removed=%s patched=%d failed=%d%v calls=%d tokens in=%d out=%d cache_read=%d cache_write=%d → %s\n",
			res.Rounds, res.Clean, strings.Join(rr.Changed, ","), strings.Join(rr.Added, ","), strings.Join(rr.Removed, ","), patched,
			len(failed), failed, k.n.Load(), t.Input, t.Output, t.CacheRead, t.CacheWrite, o.Out)
	}
	var old map[string]render.Scene // pre-revision requests, for sentence-level patches (compile only, no model call)
	if o.Render {
		if _, prev, err := shownScenes(ctx, story); err == nil {
			old = make(map[string]render.Scene, len(prev))
			for _, s := range prev {
				old[s.ID] = s
			}
		}
	}
	res, err := write.ReviseLoop(ctx, m, writeChecker(o.Reserved), story, request, refs, o)
	if err != nil {
		summary(res, write.RerenderResult{}, nil)
		return 1, err
	}
	if !res.Clean { // the checker loaded a rejected candidate; put PG back on the story as it is on disk
		if _, err := load(story, io.Discard); err != nil {
			fmt.Fprintln(os.Stderr, "dieg: reload:", err)
		}
		for _, d := range res.Diags {
			if d.Blocking() {
				fmt.Fprintln(os.Stderr, d)
			}
		}
		summary(res, write.RerenderResult{}, nil)
		return 1, nil
	}
	if !o.Render {
		summary(res, write.RerenderResult{}, nil)
		return 0, nil
	}
	work, all, err := shownScenes(ctx, story)
	if err != nil {
		summary(res, write.RerenderResult{}, nil)
		return 1, err
	}
	o, closeDigest, err := withDigest(ctx, work, o)
	if err != nil {
		summary(res, write.RerenderResult{}, nil)
		return 1, err
	}
	defer closeDigest()
	rr, err := write.Rerender(ctx, cs, work, all, b, o.Out, o, limit, old)
	if err != nil {
		summary(res, rr, nil)
		return 1, err
	}
	return reportRerender(res, rr, summary), nil
}

func reportRerender(res write.Result, rr write.RerenderResult, summary func(write.Result, write.RerenderResult, []string)) int {
	var failed []string
	for _, r := range rr.Scenes {
		if !r.Pass {
			failed = append(failed, r.ID)
		}
		fmt.Fprintf(os.Stderr, "%s: attempts=%d pass=%t missing=%v hallucinated=%v leaked=%v %s\n", r.ID, r.Attempts, r.Pass,
			r.Report.Missing, r.Report.Hallucinated, r.Report.Leaked, r.Report.ExtractError)
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "dieg: %s: %v\n", r.ID, r.Err)
		}
	}
	summary(res, rr, failed)
	if len(failed) > 0 {
		return 3
	}
	return 0
}
