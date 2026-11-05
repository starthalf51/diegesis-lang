package main

import (
	. "story-lang/internal/render"

	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	scenesDir := flag.String("scenes", "internal/render/testdata/scenes", "scene IR directory")
	stylesDir := flag.String("styles", "internal/render/testdata/styles", "style sample directory")
	out := flag.String("out", "experiments/m0-render/out", "output directory")
	variants := flag.String("variants", "a,b,c", "comma-separated variants")
	langs := flag.String("langs", "ko,en", "comma-separated languages")
	force := flag.Bool("force", false, "re-run jobs that already have output")
	only := flag.String("only", "", "run only jobs whose key contains this substring")
	backend := flag.String("backend", "cli", "cli (claude -p, subscription) or api")
	flag.Parse()
	if err := run(*scenesDir, *stylesDir, *out, *variants, *langs, *only, *backend, *force); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(scenesDir, stylesDir, out, variants, langs, only, backend string, force bool) error {
	var c Completer
	switch backend {
	case "cli":
		c = NewClaudeCLI()
	case "api":
		c = NewClaude()
	default:
		return fmt.Errorf("unknown -backend %q (want cli or api)", backend)
	}
	paths, err := filepath.Glob(filepath.Join(scenesDir, "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	var scenes []Scene
	for _, p := range paths {
		s, err := LoadScene(p)
		if err != nil {
			return err
		}
		scenes = append(scenes, s)
	}
	styles, err := LoadStyles(stylesDir)
	if err != nil {
		return err
	}
	var ls []string
	for _, l := range strings.Split(langs, ",") {
		ls = append(ls, strings.TrimSpace(l))
	}
	var vs []Variant
	for _, v := range strings.Split(variants, ",") {
		vs = append(vs, Variant(strings.TrimSpace(v)))
	}
	jobs, err := Jobs(scenes, styles, vs, ls)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	failedMsgs := map[string]string{}
	for _, j := range jobs {
		if !strings.Contains(j.Key(), only) {
			continue
		}
		r, err := RunJob(context.Background(), c, j, out, force)
		if err != nil {
			fmt.Printf("%s ERROR %v\n", j.Key(), err)
			failedMsgs[j.Key()] = err.Error()
			continue
		}
		note := fmt.Sprintf("cost=$%.4f", r.RenderCost+r.ExtractCost)
		if r.Skipped {
			note = "skipped"
		}
		fmt.Printf("%s %s stop=%s missing=%d halluc=%d leaked=%d err=%q\n", r.Key, note, r.StopReason,
			len(r.Report.Missing), len(r.Report.Hallucinated), len(r.Report.Leaked), r.Report.ExtractError)
	}
	recs, err := LoadRecords(out)
	if err != nil {
		return err
	}
	if err := WriteResults(recs, failedMsgs, filepath.Join(out, "results.md")); err != nil {
		return err
	}
	if len(failedMsgs) > 0 {
		return fmt.Errorf("%d job(s) failed", len(failedMsgs))
	}
	return nil
}
