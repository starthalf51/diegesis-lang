package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"story-lang/internal/accept"
	"story-lang/internal/pg"
	"story-lang/internal/render"
)

// sceneRequest 는 작품(ID 또는 코퍼스 디렉터리 — 디렉터리면 먼저 적재한다)의 장면 렌더 요청을 PG 에서 받는다.
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

// cmdRender runs `slc render <work|dir> <scene> --backend <id> [-o file] [--styles dir]`: 산문만 낸다(재추출 없음).
func cmdRender(args []string) int {
	var pos []string
	id, out, styles := "", "", "styles"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
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
	if err := renderOnce(pos[0], pos[1], id, out, styles); err != nil {
		fmt.Fprintln(os.Stderr, "slc:", err)
		return 1
	}
	return 0
}

func renderOnce(target, scene, id, out, styles string) error {
	b, err := backend(styles, id)
	if err != nil {
		return err
	}
	_, s, err := sceneRequest(target, scene)
	if err != nil {
		return err
	}
	sys, user, err := b.Prompt(s)
	if err != nil {
		return err
	}
	res, err := render.NewClaudeCLI().Complete(context.Background(), sys, user)
	if err != nil {
		return err
	}
	if res.StopReason != "end_turn" {
		return fmt.Errorf("render stopped: %q", res.StopReason)
	}
	if out == "" {
		_, err = fmt.Println(res.Text)
		return err
	}
	return os.WriteFile(out, []byte(res.Text+"\n"), 0o644)
}

// cmdRenderCheck runs `slc render-check <corpus...> [--force] [--styles dir]`: 코퍼스마다 expect.yaml 의 render_scene 을
// 모든 백엔드로 렌더 → 재추출 → diff 하고 ADR-012 로 판정한다. 보고서는 out/render-check.md, 산문은 out/render/.
// 합격이면 0.
func cmdRenderCheck(args []string) int {
	var dirs []string
	force, styles := false, "styles"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--force":
			force = true
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
	pass, err := renderCheck(dirs, styles, force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slc:", err)
		return 1
	}
	if !pass {
		return 1
	}
	return 0
}

func renderCheck(dirs []string, styles string, force bool) (bool, error) {
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
			r, err := render.RenderCached(context.Background(), c, filepath.Join("out", "render"), work, s, b, force)
			if err != nil { // 기록하지 않았으니 다음 실행이 다시 시도한다
				errs = append(errs, err.Error())
				fmt.Fprintln(os.Stderr, "slc:", err)
				continue
			}
			fmt.Printf("%s %s %s: missing %d, hallucinated %d, leaked %d, tones %v %s\n", work, s.ID, b.ID,
				len(r.Report.Missing), len(r.Report.Hallucinated), len(r.Report.Leaked), shares(r.Tones), r.Report.ExtractError)
			rs = append(rs, r)
		}
		vs = append(vs, render.Judge(work, s.ID, rs))
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
	fmt.Fprintf(&b, "# 렌더 판정 (ADR-012) — %s\n\n", map[bool]string{true: "PASS", false: "FAIL"}[pass])
	fmt.Fprintf(&b, "기준: 렌더마다 missing ≤ %d, hallucinated ≤ %d, leaked = 0. 같은 장면의 백엔드끼리 재추출 액트 키 Jaccard ≥ %.1f. "+
		"한국어 백엔드는 목표 어투 비율 ≥ %.1f, 1위 어투가 서로 다르다. 어투 비율은 따옴표 밖 서술 문장의 종결어미로 센다.\n",
		render.MaxMissing, render.MaxHallucinated, render.MinJaccard, render.MinToneShare)
	for _, v := range vs {
		fmt.Fprintf(&b, "\n## %s %s — %s\n\n", v.Work, v.Scene, map[bool]string{true: "PASS", false: "FAIL"}[v.Pass])
		b.WriteString("| 백엔드 | missing | halluc | leaked | haera | hapsho | gojeon | other | 1위 | 판정 | 산문 |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
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
		b.WriteString("\nJaccard(재추출 액트 키 멀티셋): ")
		for i, k := range ks {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s %.2f", k, v.Jaccard[k])
		}
		b.WriteString("\n")
		for _, r := range v.Rows {
			rep := r.Report
			for _, it := range []struct {
				n  string
				xs []string
			}{{"missing", rep.Missing}, {"hallucinated", rep.Hallucinated}, {"leaked", rep.Leaked}, {"실패", r.Why}} {
				if len(it.xs) > 0 {
					fmt.Fprintf(&b, "- %s %s: %s\n", r.Backend, it.n, strings.Join(it.xs, "; "))
				}
			}
		}
		for _, w := range v.Why {
			fmt.Fprintf(&b, "- 장면 실패: %s\n", w)
		}
	}
	for _, e := range errs {
		fmt.Fprintf(&b, "\n- 오류: %s\n", e)
	}
	return b.String()
}

func or(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
