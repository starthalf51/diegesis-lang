package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/write"
)

// renderOnly lists the commonFlag flags that matter only once scenes are rendered; convert would ignore them, so it rejects them.
var renderOnly = map[string]bool{"--scenes": true, "--render-model": true, "--extract-model": true, "--render-effort": true,
	"--no-digest": true, "--verifier": true, "--backend": true, "--styles": true}

// cmdConvert runs `dieg convert <file> -o <dir> [--rounds N] [--code-model M] [--code-effort E] [--allow-warn] [--force]`:
// a public-domain story text becomes Diegesis code through the write loop; nothing is rendered.
func cmdConvert(args []string) int {
	o := write.DefaultOpts()
	o.Render = false
	var file, styles string
	var limit int
	force := false
	for i := 0; i < len(args); i++ {
		j, ok, code := commonFlag(args, i, &o, &styles, &limit)
		switch a := args[i]; {
		case code != 0:
			return code
		case ok && renderOnly[a]:
			fmt.Fprintf(os.Stderr, "dieg: convert does not render: %s is not accepted\n", a)
			return 2
		case ok:
			i = j
		case a == "--force":
			force = true
		case a == "-o" && i+1 < len(args):
			i++
			o.Out = args[i]
		case file == "" && !strings.HasPrefix(a, "-"):
			file = a
		default:
			return usage()
		}
	}
	if file == "" || o.Out == "" {
		return usage()
	}
	if err := runConvert(file, o, force); err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return 0
}

func runConvert(file string, o write.Opts, force bool) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	title := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	idea, err := write.ConvertIdea(title, string(b))
	if err != nil {
		return err
	}
	refs, err := write.LoadRefs()
	if err != nil {
		return err
	}
	if err := notUnderCorpus(o.Out); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(o.Out, "story")); err == nil && !force {
		return fmt.Errorf("%s/story already exists: convert would delete it. Pass --force to start over", o.Out)
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	if err := write.ResetRun(o.Out); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(o.Out, "calls.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	m, _, k := meters(o, f)
	o.Reserved = corpusWorks()
	summary := summaryLine("convert", m, k, o.Out)
	res, err := write.Loop(context.Background(), m, writeChecker(o.Reserved), idea, refs, o)
	if err != nil {
		summary(res, 0, nil)
		return err
	}
	if code, _ := uncleanOrNoRender(res, o, summary); code != 0 {
		return fmt.Errorf("code not clean after %d rounds", res.Rounds)
	}
	return nil
}
