// corpus-ab 는 render-check 코퍼스 장면(corpus/*/expect.yaml 의 render_scene) × 백엔드 전부를 단일(RenderVerified2, 현 기본)
// 또는 계층형(narrate.RenderLayered)으로 렌더·검증하고 쌍마다 JSON·산문을 남긴다. 하드 호출 예산이 있다.
//
//	go run ./tools/corpus-ab -mode single -budget 100 -out examples/corpus-ab/p1
//	go run ./tools/corpus-ab -mode layered -budget 60 -out examples/corpus-ab/p2 -pairs macbeth/1.3.ko-haera,chibi/45.88.ko-haera
//
// 저장소 루트에서 돌린다. 렌더는 haiku/medium, 재추출은 sonnet/low (dieg render --verify 기본값과 같다).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"diegesis/internal/accept"
	"diegesis/internal/narrate"
	"diegesis/internal/pg"
	"diegesis/internal/render"
)

const (
	sonnet, haiku = "claude-sonnet-5-5", "claude-haiku-5-5"
)

var errBudget = errors.New("call budget reached")

type counter struct {
	c       render.Completer
	n, used *int
	budget  int
}

func (k counter) Complete(ctx context.Context, system, user string) (render.Result, error) {
	if *k.used >= k.budget {
		return render.Result{}, errBudget
	}
	*k.used++
	*k.n++
	return k.c.Complete(ctx, system, user)
}

type Run struct {
	Mode, Pair, Work, Scene, Backend string
	Beats, Acts                      int
	Passed                           bool
	Calls, RenderCalls, ExtractCalls int
	RenderUsage, ExtractUsage        render.Usage
	Attempts                         int    `json:",omitempty"`
	BeatAttempts                     []int  `json:",omitempty"`
	BeatFailed                       []int  `json:",omitempty"`
	StyleOK                          bool   `json:",omitempty"`
	FinalFrom                        string `json:",omitempty"`
	Chars                            int
	Missing, Hallucinated, Leaked    []string
	ExtractError, Err, Skipped       string `json:",omitempty"`
}

type scene struct {
	work string
	s    render.Scene
}

func acts(s render.Scene) int {
	n := 0
	for _, b := range s.Beats {
		n += len(b.Acts)
	}
	return n
}

func load(ctx context.Context) ([]scene, error) {
	files, _ := filepath.Glob("corpus/*/expect.yaml")
	pool, err := pg.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	var out []scene
	for _, f := range files {
		raw, _ := os.ReadFile(f)
		e, err := accept.Parse(raw)
		if err != nil || e.RenderScene == "" {
			continue
		}
		dir := filepath.Dir(f)
		cmd := exec.CommandContext(ctx, "go", "run", "./cmd/dieg", "load", dir)
		cmd.Stderr = os.Stderr
		if b, err := cmd.Output(); err != nil {
			return nil, fmt.Errorf("load %s: %w", dir, err)
		} else if w := strings.TrimSpace(string(b)); w == "" {
			return nil, fmt.Errorf("load %s: empty", dir)
		}
		work := filepath.Base(dir)
		b, err := pg.RenderScene(ctx, pool, work, e.RenderScene)
		if err != nil {
			return nil, err
		}
		s, err := render.ParseScene(b)
		if err != nil {
			return nil, err
		}
		out = append(out, scene{work, s})
	}
	return out, nil
}

func main() {
	mode := flag.String("mode", "single", "single|layered")
	budget := flag.Int("budget", 100, "max real calls")
	out := flag.String("out", "examples/corpus-ab/p1", "output dir")
	pairs := flag.String("pairs", "", "comma list work/scene.backend (default: all, in order)")
	spike := flag.String("spike", "", "h1|h2: spike.go")
	flag.Parse()
	ctx := context.Background()
	bs, err := render.LoadBackends("styles")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scs, err := load(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *spike != "" {
		switch *spike {
		case "report":
			runReport(*out, scs, bs, *pairs)
		case "dump", "s1", "s2":
			runVerifier(ctx, *spike, *budget, *out, scs, bs, *pairs)
		default:
			runSpike(ctx, *spike, *budget, *out, scs, bs, *pairs)
		}
		return
	}
	type job struct {
		sc scene
		b  render.Backend
	}
	var jobs []job
	if *pairs == "" {
		for _, sc := range scs {
			for _, b := range bs {
				jobs = append(jobs, job{sc, b})
			}
		}
	} else {
		for _, p := range strings.Split(*pairs, ",") {
			for _, sc := range scs {
				for _, b := range bs {
					if p == sc.work+"/"+sc.s.ID+"."+b.ID {
						jobs = append(jobs, job{sc, b})
					}
				}
			}
		}
	}
	_ = os.MkdirAll(*out, 0o755)
	lf, _ := os.OpenFile(filepath.Join(*out, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer lf.Close()
	used := 0
	for _, j := range jobs {
		p, _ := narrate.PlanScene(j.sc.s, narrate.DefaultOpts().Plan)
		r := Run{Mode: *mode, Work: j.sc.work, Scene: j.sc.s.ID, Backend: j.b.ID, Beats: len(p.Beats), Acts: acts(j.sc.s)}
		r.Pair = r.Work + "/" + r.Scene + "." + r.Backend
		need := 6
		if *mode == "layered" {
			need = r.Beats*2 + 8 // 비트당 생성+검증 최소 2회 + 문체·연결·대조
		}
		if used+need > *budget {
			r.Skipped = fmt.Sprintf("budget: used %d + est %d > %d", used, need, *budget)
			fmt.Fprintln(os.Stderr, r.Pair, r.Skipped)
			save(*out, r, "")
			continue
		}
		rm := render.NewMeter(counter{render.NewClaudeCLIWith(haiku, "medium"), &r.RenderCalls, &used, *budget}, lf)
		xm := render.NewMeter(counter{render.NewClaudeCLIWith(sonnet, "low"), &r.ExtractCalls, &used, *budget}, lf)
		var rr render.Render
		if *mode == "single" {
			rr, r.Attempts, err = render.RenderVerified2(ctx, render.Completers{Render: rm.Label("render-" + r.Pair), Extract: xm.Label("extract-" + r.Pair)},
				j.sc.work, j.sc.s, j.b, render.VerifyOpts{K: 1, Retries: 2})
		} else {
			roles := narrate.Roles{Beat: rm, Verify: xm, Style: rm, Stitch: rm}
			var st narrate.LayerStats
			rr, st, err = narrate.RenderLayered(ctx, roles, j.sc.work, j.sc.s, j.b, narrate.DefaultOpts())
			r.BeatAttempts, r.BeatFailed, r.StyleOK, r.FinalFrom = st.BeatAttempts, st.BeatFailed, st.StyleOK, st.FinalFrom
		}
		if err != nil {
			r.Err = err.Error()
		}
		r.Calls = r.RenderCalls + r.ExtractCalls
		r.RenderUsage, r.ExtractUsage = rm.Total(), xm.Total()
		r.Passed = err == nil && render.Passed(rr)
		rep := rr.Report
		r.Missing, r.Hallucinated, r.Leaked, r.ExtractError = rep.Missing, rep.Hallucinated, rep.Leaked, rep.ExtractError
		r.Chars = utf8.RuneCountInString(strings.Join(strings.Fields(rr.Prose), ""))
		fmt.Fprintf(os.Stderr, "%-34s beats=%d pass=%v calls=%d used=%d %s\n", r.Pair, r.Beats, r.Passed, r.Calls, used, r.Err)
		save(*out, r, rr.Prose)
	}
	fmt.Fprintln(os.Stderr, "total calls", used)
}

func save(dir string, r Run, prose string) {
	b, _ := json.MarshalIndent(r, "", "  ")
	n := strings.ReplaceAll(r.Pair, "/", "_")
	_ = os.WriteFile(filepath.Join(dir, n+".json"), append(b, '\n'), 0o644)
	if prose != "" {
		_ = os.WriteFile(filepath.Join(dir, n+".md"), []byte(strings.TrimSpace(prose)+"\n"), 0o644)
	}
}
