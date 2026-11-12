package main

import (
	"github.com/starthalf51/diegesis-lang/internal/write"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/accept"
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

// sceneRequest fetches a scene render request from PG for a work (an ID or a corpus directory — a directory is loaded first).
func sceneRequest(target, scene string) (string, render.Scene, error) {
	work := target
	if _, err := os.Stat(target); err == nil {
		if work, err = load(target, io.Discard); err != nil {
			return "", render.Scene{}, err
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return "", render.Scene{}, err
	}
	defer pool.Close()
	b, err := pg.RenderScene(ctx, pool, work, scene)
	if err != nil {
		return "", render.Scene{}, err
	}
	s, err := render.ParseScene(b)
	if err != nil {
		return "", s, fmt.Errorf("%s %s: render request: %w", work, scene, err)
	}
	return work, s, nil
}

func backend(dir, id string) (render.Backend, error) {
	bs, err := render.LoadBackends(dir)
	if err != nil {
		return render.Backend{}, err
	}
	for _, b := range bs {
		if b.ID == id {
			return b, nil
		}
	}
	return render.Backend{}, fmt.Errorf("no backend %q in %s", id, dir)
}

// cmdRender runs `dieg render <work|dir> <scene> --backend <id> [--verify] [--retries N] [--render-model M] [--extract-model M] [-o file] [--styles dir]`: emits prose.
// With --verify it goes through re-extraction, comparison and feedback re-rendering (at most 1+retries times); if it does not pass, the prose is still written and the exit code is 3.
func cmdRender(args []string) int {
	var pos []string
	id, out, styles, verify, retries := "", "", "styles", false, 2
	opening := false
	verifier, verifierSet := "card", false
	dw := write.DefaultOpts()
	rm, xm, re := dw.RenderModel, dw.ExtractModel, dw.RenderEffort
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--opening":
			opening = true
		case a == "--verify":
			verify = true
		case a == "--verifier" && i+1 < len(args):
			i++
			verifierSet = true
			if verifier = args[i]; verifier != "card" && verifier != "open" {
				fmt.Fprintf(os.Stderr, "dieg: --verifier %q: want card or open\n", verifier)
				return 2
			}
		case (a == "--render-model" || a == "--extract-model") && i+1 < len(args):
			i++
			if err := render.ValidModel(args[i]); err != nil {
				fmt.Fprintln(os.Stderr, "dieg:", err)
				return 2
			}
			*map[string]*string{"--render-model": &rm, "--extract-model": &xm}[a] = args[i]
		case a == "--render-effort" && i+1 < len(args):
			i++
			if re = args[i]; re != "low" && re != "medium" && re != "high" {
				fmt.Fprintf(os.Stderr, "dieg: --render-effort %q: want low, medium or high\n", re)
				return 2
			}
		case a == "--retries" && i+1 < len(args):
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return usage()
			}
			retries = n
		case (a == "--backend" || a == "-o" || a == "--styles") && i+1 < len(args):
			i++
			*map[string]*string{"--backend": &id, "-o": &out, "--styles": &styles}[a] = args[i]
		case !strings.HasPrefix(a, "-"):
			pos = append(pos, a)
		default:
			return usage()
		}
	}
	if len(pos) != 2 || id == "" {
		return usage()
	}
	if verifierSet && !verify {
		fmt.Fprintln(os.Stderr, "dieg: --verifier needs --verify")
		return 2
	}
	pass, err := renderOnce(pos[0], pos[1], id, out, styles, verify, opening, retries, rm, xm, re, verifier)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	if !pass {
		return 3
	}
	return 0
}

// renderOnce writes the prose even when pass=false (--verify went to the limit and still did not pass).
func renderOnce(target, scene, id, out, styles string, verify, opening bool, retries int, rm, xm, re, verifier string) (bool, error) {
	b, err := backend(styles, id)
	if err != nil {
		return false, err
	}
	_, s, err := sceneRequest(target, scene)
	if err != nil {
		return false, err
	}
	s.Opening = opening // goes into the prompt, so the render hash differs from a render without it
	var text string
	pass := true
	if verify {
		r, n, err := render.RenderVerified2(context.Background(), render.Completers{Render: render.NewCompleter(rm, re), Extract: render.NewCompleter(xm, "low")}, target, s, b, render.VerifyOpts{K: 1, Retries: retries, Verifier: verifier})
		if err != nil {
			return false, err
		}
		pass = render.Passed(r)
		g2 := fmt.Sprintf("halluc=%d", len(r.Report.Hallucinated))
		if verifier == "card" {
			g2 = fmt.Sprintf("warnings=%d", len(r.Report.Warnings))
		}
		fmt.Fprintf(os.Stderr, "verify: attempts=%d missing=%d %s leaked=%d pass=%t\n", n,
			len(r.Report.Missing), g2, len(r.Report.Leaked), pass)
		if r.Report.ExtractError != "" {
			fmt.Fprintln(os.Stderr, "verify:", r.Report.ExtractError)
		}
		text = r.Prose
	} else {
		sys, user, err := b.Prompt(s)
		if err != nil {
			return false, err
		}
		res, err := render.NewCompleter(rm, re).Complete(context.Background(), sys, user)
		if err != nil {
			return false, err
		}
		if res.StopReason != "end_turn" {
			return false, fmt.Errorf("render stopped: %q", res.StopReason)
		}
		text = res.Text
	}
	if out == "" {
		_, err = fmt.Println(text)
	} else {
		err = os.WriteFile(out, []byte(text+"\n"), 0o644)
	}
	return pass && err == nil, err
}

// cmdRenderCheck runs `dieg render-check <corpus...> [--force | --force-extract] [--only <work>/<scene>.<backend>]... [--styles dir]`: for each corpus, renders the expect.yaml render_scene
// with every backend → re-extracts → diffs, and judges per ADR-012. The report is out/render-check.md, prose is in out/render/.
// Exit 0 on pass.
func cmdRenderCheck(args []string) int {
	var dirs []string
	var f render.Force
	only := map[string]bool{}
	styles := "styles"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--force":
			f.Render = true
		case a == "--force-extract":
			f.Extract = true
		case a == "--only" && i+1 < len(args):
			i++
			only[args[i]] = false
		case a == "--styles" && i+1 < len(args):
			i++
			styles = args[i]
		case !strings.HasPrefix(a, "-"):
			dirs = append(dirs, a)
		default:
			return usage()
		}
	}
	if len(dirs) == 0 {
		return usage()
	}
	pass, err := renderCheck(dirs, styles, f, only)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	if !pass {
		return 1
	}
	return 0
}

// If only is given, f applies only to the pairs listed there (<work>/<scene>.<backend>). The other pairs are judged from cache. An entry with no matching pair is an error.
func renderCheck(dirs []string, styles string, f render.Force, only map[string]bool) (bool, error) {
	bs, err := render.LoadBackends(styles)
	if err != nil {
		return false, err
	}
	c := render.NewClaudeCLI()
	var vs []render.SceneVerdict
	var errs []string
	for _, dir := range dirs {
		raw, err := os.ReadFile(filepath.Join(dir, "expect.yaml"))
		if err != nil {
			return false, err
		}
		e, err := accept.Parse(raw)
		if err != nil {
			return false, err
		}
		if e.RenderScene == "" {
			return false, fmt.Errorf("%s: expect.yaml has no render_scene", dir)
		}
		work, s, err := sceneRequest(dir, e.RenderScene)
		if err != nil {
			return false, err
		}
		var rs []render.Render
		for _, b := range bs {
			r, err := render.RenderCached(context.Background(), c, filepath.Join("out", "render"), work, s, b, scoped(f, only, work+"/"+s.ID+"."+b.ID))
			if err != nil { // nothing was recorded, so the next run retries
				errs = append(errs, err.Error())
				fmt.Fprintln(os.Stderr, "dieg:", err)
				continue
			}
			r = render.Score(s, r)
			fmt.Printf("%s %s %s: missing %d, hallucinated %d, leaked %d, tones %v %s\n", work, s.ID, b.ID,
				len(r.Report.Missing), len(r.Report.Hallucinated), len(r.Report.Leaked), shares(r.Tones), r.Report.ExtractError)
			rs = append(rs, r)
		}
		vs = append(vs, render.Judge(work, s, rs))
	}
	for k, hit := range only {
		if !hit {
			errs = append(errs, "--only "+k+": no such pair")
		}
	}
	pass := len(errs) == 0
	for _, v := range vs {
		pass = pass && v.Pass && len(v.Rows) == len(bs)
	}
	out := filepath.Join("out", "render-check.md")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(out, []byte(checkReport(vs, errs, pass)), 0o644); err != nil {
		return false, err
	}
	fmt.Printf("%s → %s\n", map[bool]string{true: "PASS", false: "FAIL"}[pass], out)
	return pass, nil
}

// scoped uses f if only is empty, otherwise only when key is in only (marking the matched entries).
func scoped(f render.Force, only map[string]bool, key string) render.Force {
	if len(only) == 0 {
		return f
	}
	if _, ok := only[key]; !ok {
		return render.Force{}
	}
	only[key] = true
	return f
}

func shares(t map[string]float64) string {
	if len(t) == 0 {
		return "-"
	}
	var ks []string
	for k := range t {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var p []string
	for _, k := range ks {
		p = append(p, fmt.Sprintf("%s %.2f", k, t[k]))
	}
	return strings.Join(p, ", ")
}

func checkReport(vs []render.SceneVerdict, errs []string, pass bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Render verdict (ADR-012) — %s\n\n", map[bool]string{true: "PASS", false: "FAIL"}[pass])
	fmt.Fprintf(&b, "Criteria: per render missing ≤ %d, hallucinated ≤ %d, leaked = 0. Jaccard between backends of the same scene ≥ %.1f — over the re-extracted acts, "+
		"the multiset of actor|verb-class|canonical-fact keys of the non-entry/exit acts that convey facts (the all-acts Jaccard is listed for reference only). "+
		"Korean backends need a target-tone ratio ≥ %.1f, and the top tones must differ from each other. The tone ratio is counted from the sentence endings of narration outside quotes.\n",
		render.MaxMissing, render.MaxHallucinated, render.MinJaccard, render.MinToneShare)
	for _, v := range vs {
		fmt.Fprintf(&b, "\n## %s %s — %s\n\n", v.Work, v.Scene, map[bool]string{true: "PASS", false: "FAIL"}[v.Pass])
		b.WriteString("| backend | missing | halluc | leaked | haera | hapsho | gojeon | other | top | verdict | prose |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, r := range v.Rows {
			tone := func(k string) string {
				if r.Tones == nil {
					return "-"
				}
				return fmt.Sprintf("%.2f", r.Tones[k])
			}
			p := render.ProsePath(r.Work, r.Scene, r.Backend)
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %s | %s | %s | %s | %s | %s | [%s](render/%s) |\n", r.Backend,
				len(r.Report.Missing), len(r.Report.Hallucinated), len(r.Report.Leaked),
				tone("haera"), tone("hapsho"), tone("gojeon"), tone("other"), or(r.Top, "-"),
				map[bool]string{true: "PASS", false: "FAIL"}[r.Pass], p, p)
		}
		var ks []string
		for k := range v.Jaccard {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, j := range []struct {
			label string
			m     map[string]float64
		}{{"Jaccard (non-entry/exit acts that convey facts — judged)", v.Jaccard}, {"Jaccard (all acts — reference only, not judged)", v.JaccardAll}} {
			b.WriteString("\n" + j.label + ": ")
			for i, k := range ks {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s %.2f", k, j.m[k])
			}
			b.WriteString("\n")
		}
		for _, r := range v.Rows {
			rep := r.Report
			for _, it := range []struct {
				n  string
				xs []string
			}{{"missing", rep.Missing}, {"hallucinated", rep.Hallucinated}, {"leaked", rep.Leaked}, {"failure", r.Why}} {
				if len(it.xs) > 0 {
					fmt.Fprintf(&b, "- %s %s: %s\n", r.Backend, it.n, strings.Join(it.xs, "; "))
				}
			}
		}
		for _, w := range v.Why {
			fmt.Fprintf(&b, "- scene failure: %s\n", w)
		}
	}
	for _, e := range errs {
		fmt.Fprintf(&b, "\n- error: %s\n", e)
	}
	return b.String()
}

func or(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
