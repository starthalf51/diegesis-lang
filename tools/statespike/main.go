// statespike: 스파이크(버릴 코드). run3 B 의 5개 장면을 PG 상태 요약 있음(D)/없음(0) 으로 다시 렌더한다.
//
//	go run ./tools/statespike -digest            호출 0: 5개 장면의 요약만 출력·저장
//	go run ./tools/statespike                    가짜 완성기 전체 드라이런(호출 0)
//	go run ./tools/statespike -real              진짜 호출(claude -p, 구독). 호출 상한 -budget
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

const work = "yeoul"

var scenes = []string{"s2", "s21", "s25", "s27", "s29"}

// guard 는 호출 직전에 상한을 확인하고 세는 완성기. 상한이면 호출하지 않고 오류.
type guard struct {
	inner render.Completer
	mu    *sync.Mutex
	used  *int
	cap   int
	log   *os.File
	label string
}

func (g *guard) Complete(ctx context.Context, sys, user string) (render.Result, error) {
	g.mu.Lock()
	if *g.used >= g.cap {
		g.mu.Unlock()
		return render.Result{}, fmt.Errorf("budget guard: %d/%d calls used", *g.used, g.cap)
	}
	*g.used++
	n := *g.used
	g.mu.Unlock()
	r, err := g.inner.Complete(ctx, sys, user)
	rec := map[string]any{"n": n, "label": g.label, "in": r.Usage.Input, "out": r.Usage.Output, "cache_read": r.Usage.CacheRead,
		"cache_write": r.Usage.CacheWrite, "stop": r.StopReason, "err": ""}
	if err != nil {
		rec["err"] = err.Error()
	}
	b, _ := json.Marshal(rec)
	g.log.Write(append(b, '\n'))
	return r, err
}

// withDigest 는 렌더 사용자 프롬프트 끝에 요약을 붙인다(재시도 피드백 뒤).
type withDigest struct {
	inner  render.Completer
	digest string
	save   string // 첫 호출의 전체 사용자 프롬프트를 여기에 쓴다
}

func (w withDigest) Complete(ctx context.Context, sys, user string) (render.Result, error) {
	return w.inner.Complete(ctx, sys, user+"\n"+w.digest)
}

type cell struct {
	Arm, Scene string
	Run        int
	Attempts   int
	Pass       bool
	Missing    []string
	Leaked     []string
	Warnings   []string
	Score      float64
	ExtractErr string
	Calls      int
}

func main() {
	real := flag.Bool("real", false, "진짜 호출")
	onlyDigest := flag.Bool("digest", false, "요약만 만들고 끝")
	budget := flag.Int("budget", 50, "진짜 호출 상한")
	reserve := flag.Int("reserve", 18, "한 쌍(D,0)을 시작하려면 남아야 하는 호출 수")
	runs := flag.Int("runs", 2, "장면·팔당 최대 실행 수")
	out := flag.String("out", "examples/state-digest", "출력 폴더")
	flag.Parse()
	ctx := context.Background()
	if !*real {
		d, _ := os.MkdirTemp("", "statespike-dry")
		defer os.RemoveAll(d)
		if !*onlyDigest {
			*out = d
		}
	}
	must(os.MkdirAll(*out, 0o755))
	pool, err := pg.Connect(ctx)
	must(err)
	defer pool.Close()

	bs, err := render.LoadBackends("styles")
	must(err)
	var be render.Backend
	for _, b := range bs {
		if b.ID == "ko-haera" {
			be = b
		}
	}
	reqs := map[string]render.Scene{}
	digests := map[string]string{}
	for _, id := range scenes {
		raw, err := pg.RenderScene(ctx, pool, work, id)
		must(err)
		s, err := render.ParseScene(raw)
		must(err)
		s.FreeDialogue = true // write.DefaultOpts().Skeleton
		reqs[id] = s
		digests[id], err = Digest(ctx, pool, work, s)
		must(err)
		must(os.WriteFile(filepath.Join(*out, "digest-"+id+".txt"), []byte(digests[id]), 0o644))
	}
	if *onlyDigest {
		for _, id := range scenes {
			fmt.Printf("=== %s\n%s\n", id, digests[id])
		}
		for _, id := range scenes {
			h, _ := render.RenderHash(reqs[id], be)
			old, _ := os.ReadFile("examples/serial/run3/b/novel/" + id + ".ko-haera.hash")
			fmt.Printf("hash %s same-as-run3=%t\n", id, strings.TrimSpace(string(old)) == h)
		}
		_, u, _ := be.Prompt(reqs["s2"])
		fmt.Println("=== FULL ARM-D USER PROMPT (s2) ===\n" + u + "\n" + digests["s2"])
		return
	}

	// 호출 기록·카운터. 이미 기록된 호출은 상한에 포함한다.
	logPath := filepath.Join(*out, "calls.jsonl")
	used := 0
	if f, err := os.Open(logPath); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			used++
		}
		f.Close()
	}
	lf, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	must(err)
	defer lf.Close()
	var mu sync.Mutex
	var base render.Completer
	if *real {
		base = nil
	} else {
		base = &fake{}
	}
	mk := func(model, effort, label string) render.Completer {
		inner := base
		if *real {
			inner = render.NewClaudeCLIWith(model, effort)
		}
		return &guard{inner: inner, mu: &mu, used: &used, cap: *budget, log: lf, label: label}
	}

	var cells []cell
	prev, _ := os.ReadFile(filepath.Join(*out, "cells.json"))
	json.Unmarshal(prev, &cells)
	done := func(arm, sc string, run int) bool {
		for _, c := range cells {
			if c.Arm == arm && c.Scene == sc && c.Run == run {
				return true
			}
		}
		return false
	}
	// 팔 순서는 장면·실행마다 번갈아(교차). 한 쌍(D,0)은 기대 호출의 여유(+18)가 있을 때만 시작한다.
	for run := 1; run <= *runs; run++ {
		for i, id := range scenes {
			arms := []string{"D", "0"}
			if (i+run)%2 == 1 {
				arms = []string{"0", "D"}
			}
			if used+*reserve > *budget {
				fmt.Printf("stop: used=%d, run %d scene %s 쌍을 시작할 여유 없음\n", used, run, id)
				goto end
			}
			for _, arm := range arms {
				if done(arm, id, run) {
					continue
				}
				before := used
				label := fmt.Sprintf("%s-%s-r%d", arm, id, run)
				var rc render.Completer = mk("claude-haiku-5-5", "low", label+"-render")
				if arm == "D" {
					rc = withDigest{rc, digests[id], filepath.Join(*out, "prompt-D-"+id+"-r"+fmt.Sprint(run)+".txt")}
				}
				xc := mk("claude-sonnet-5-5", "low", label+"-extract")
				r, n, err := render.RenderVerified2(ctx, render.Completers{Render: rc, Extract: xc}, work, reqs[id], be,
					render.VerifyOpts{K: 1, Retries: 2, Verifier: "card"})
				c := cell{Arm: arm, Scene: id, Run: run, Attempts: n, Calls: used - before}
				if err != nil {
					c.ExtractErr = err.Error()
				} else {
					c.Pass = render.Passed(r)
					c.Missing, c.Leaked, c.Warnings = r.Report.Missing, r.Report.Leaked, r.Report.Warnings
					c.ExtractErr = r.Report.ExtractError
					if r.Report.Scores != nil {
						c.Score = r.Report.Scores.Score
					}
					must(os.WriteFile(filepath.Join(*out, fmt.Sprintf("prose-%s-%s-r%d.md", arm, id, run)), []byte(r.Prose+"\n"), 0o644))
				}
				cells = append(cells, c)
				b, _ := json.MarshalIndent(cells, "", " ")
				must(os.WriteFile(filepath.Join(*out, "cells.json"), b, 0o644))
				fmt.Printf("%s attempts=%d pass=%t score=%.3f calls=%d used=%d %s\n", label, n, c.Pass, c.Score, c.Calls, used, c.ExtractErr)
			}
		}
	}
end:
	fmt.Printf("done: %d cells, %d calls used (cap %d)\n", len(cells), used, *budget)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "statespike:", err)
		os.Exit(1)
	}
}

// fake 는 드라이런용 가짜 완성기: 렌더에는 정해진 산문, 카드 채점에는 전부 'yes' 인 JSON.
type fake struct{}

var idRe = regexp.MustCompile(`(?m)^(E\d+)\. `)
var wRe = regexp.MustCompile(`(?m)^(W\d+)\. `)

func (f *fake) Complete(_ context.Context, sys, user string) (render.Result, error) {
	if !strings.Contains(user, "## Prose") {
		return render.Result{Text: "가짜 산문이다. 서윤은 방을 나섰다.", StopReason: "end_turn"}, nil
	}
	sec := user[:strings.Index(user, "## Prose")]
	var items, ws []string
	for _, m := range idRe.FindAllStringSubmatch(sec, -1) {
		items = append(items, fmt.Sprintf(`{"id":%q,"fact_present":"yes","as_stated":"yes","quote":"q"}`, m[1]))
	}
	for _, m := range wRe.FindAllStringSubmatch(sec, -1) {
		ws = append(ws, fmt.Sprintf(`{"id":%q,"revealed":"no","quote":""}`, m[1]))
	}
	return render.Result{Text: "```json\n{\"items\":[" + strings.Join(items, ",") + "],\"withheld\":[" + strings.Join(ws, ",") + "],\"other\":[]}\n```", StopReason: "end_turn"}, nil
}
