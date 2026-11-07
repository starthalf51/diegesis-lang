package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"diegesis/internal/pg"
	"diegesis/internal/write"
)

// cmdContinue runs `dieg continue <story-dir> --ep N [--brief "다음 화 방향" | --brief-file f] [--rounds N] [--scenes N]
// [--no-render] [--backend id] [--styles dir] [--code-model M] [--render-model M] [--extract-model M] [--code-effort e]`
// on a dir made by `dieg write -o <story-dir>`. It writes story/epNN/ from the PG state digest, renders only that
// episode's scenes (--scenes N: the first N of them), appends to calls.jsonl and re-concatenates novel.md.
// Exit codes as write.
func cmdContinue(args []string) int {
	o := write.DefaultOpts()
	styles, limit, ep := "styles", 0, 0
	var dir, brief string
	for i := 0; i < len(args); i++ {
		j, ok, code := commonFlag(args, i, &o, &styles, &limit)
		switch a := args[i]; {
		case code != 0:
			return code
		case ok:
			i = j
		case a == "--ep" && i+1 < len(args):
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 2 || n > 99 {
				return usage()
			}
			ep = n
		case a == "--brief" && i+1 < len(args):
			i++
			brief = args[i]
		case a == "--brief-file" && i+1 < len(args):
			i++
			b, err := os.ReadFile(args[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, "dieg:", err)
				return 1
			}
			brief = string(b)
		case dir == "" && !strings.HasPrefix(a, "-"):
			dir = a
		default:
			return usage()
		}
	}
	if dir == "" || ep == 0 {
		return usage()
	}
	o.Out = dir
	code, err := runContinue(ep, brief, styles, limit, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return code
}

func runContinue(ep int, brief, styles string, limit int, o write.Opts) (int, error) {
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
	if _, err := os.Stat(filepath.Join(story, "scenes")); err == nil {
		// file order is discourse order, so scenes/ would sort after every epNN/.
		return 1, fmt.Errorf("%s/scenes is the pre-serial layout; move it to %s/ep01 first", story, story)
	}
	ctx := context.Background()
	work, err := load(story, io.Discard)
	if err != nil {
		return 1, err
	}
	pool, err := pg.Connect(ctx)
	if err != nil {
		return 1, err
	}
	o.Digest, err = write.StateDigest(ctx, pool, work)
	pool.Close()
	if err != nil {
		return 1, err
	}
	if err := os.WriteFile(filepath.Join(o.Out, "digest-"+write.EpDir(ep)+".txt"), []byte(o.Digest), 0o644); err != nil {
		return 1, err
	}
	f, err := os.OpenFile(filepath.Join(o.Out, "calls.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 1, err
	}
	defer f.Close()
	m, cs, k := meters(o, f)
	o.Reserved = corpusWorks()
	summary := summaryLine("continue", m, k, o.Out)
	res, err := write.ContinueLoop(ctx, m, writeChecker(o.Reserved), story, ep, brief, refs, o)
	if err != nil {
		summary(res, 0, nil)
		return 1, err
	}
	if !res.Clean { // the checker loaded a rejected round; put PG back on the story as it is on disk
		if _, err := load(story, io.Discard); err != nil {
			fmt.Fprintln(os.Stderr, "dieg: reload:", err)
		}
	}
	if code, done := uncleanOrNoRender(res, o, summary); done {
		return code, nil
	}
	work, all, err := shownScenes(ctx, story)
	if err != nil {
		summary(res, 0, nil)
		return 1, err
	}
	ss := write.Only(all, res.Scenes)
	if limit > 0 && limit < len(ss) {
		ss = ss[:limit]
	}
	return renderAll(ctx, cs, work, ss, b, o, res, summary, all)
}
