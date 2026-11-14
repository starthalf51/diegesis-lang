package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

// settlePrompt asks for the concrete details a manuscript fixed that later episodes must not contradict (plan 19).
const settlePrompt = `너는 연재 소설의 연속성 담당이다. 아래 원고 한 화가 정해 버린, 이후 화가 어기면 안 되는 구체적이고 확인 가능한 세부만 뽑는다.
뽑을 것: 사물의 상태·색·위치, 누가 어디에 있는지, 잠기거나 부서진 것, 숫자·시각, 쓰인 호칭·별명, 상처, 약속·위협, 원고가 새로 지어낸 장소·사물의 이름.
쓰지 않을 것: 줄거리 요약, 감정, 해석, 원고가 정하지 않은 추측.
형식: 한 줄에 하나, 각 줄은 "- "로 시작한다. 최대 25줄. 다른 말은 쓰지 않는다.`

const settleMax = 25

// settleLines asks c for the settled details of text and keeps only the "- " list lines, without the marker.
func settleLines(ctx context.Context, c render.Completer, text string) ([]string, error) {
	r, err := c.Complete(ctx, settlePrompt, text)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(r.Text, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "- ") && len(out) < settleMax {
			out = append(out, strings.TrimSpace(l[2:]))
		}
	}
	return out, nil
}

// settledSection reads dir's settle files (named <scene id>.txt) of scenes told before from, in telling order.
// A file not named after a scene of the work is skipped with a warning.
func settledSection(ctx context.Context, pool *pgxpool.Pool, work, from, dir string, warn io.Writer) (string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	lo := toldOrd(ctx, pool, work, from)
	type file struct {
		ord   int
		lines string
	}
	var fs []file
	for _, e := range es {
		if e.IsDir() {
			continue
		}
		id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		o := toldOrd(ctx, pool, work, id)
		if o == 1<<30 {
			fmt.Fprintf(warn, "dieg: --settled: %s is not named after a scene; skipped\n", e.Name())
			continue
		}
		if o >= lo {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, l := range strings.Split(string(data), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				fmt.Fprintf(&b, "- %s\n", l)
			}
		}
		fs = append(fs, file{o, b.String()})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].ord < fs[j].ord })
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(f.lines)
	}
	if b.Len() == 0 {
		return "", nil
	}
	return "\n## ① 앞 원고에서 정해진 것 — 이 세부는 바꾸지 않는다. 다시 설명하지도 않는다\n" + b.String(), nil
}

// cmdSettle: dieg settle <work|dir> --from <scene> --text <manuscript> -o <dir> [--model M] writes <dir>/<from>.txt.
func cmdSettle(args []string) int {
	var target, from, textFile, outDir string
	model := "sonnet"
	flags := map[string]*string{"--from": &from, "--text": &textFile, "-o": &outDir, "--model": &model}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case flags[a] != nil && i+1 < len(args):
			i++
			*flags[a] = args[i]
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" || from == "" || textFile == "" || outDir == "" {
		return usage()
	}
	fail := func(err error) int { fmt.Fprintln(os.Stderr, "dieg:", err); return 1 }
	text, err := os.ReadFile(textFile)
	if err != nil {
		return fail(err)
	}
	work := target
	if _, err := os.Stat(target); err == nil {
		if work, err = load(target, os.Stderr); err != nil {
			return fail(err)
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return fail(err)
	}
	defer pool.Close()
	if toldOrd(ctx, pool, work, from) == 1<<30 {
		return fail(fmt.Errorf("settle: no scene %q in %s", from, work))
	}
	lines, err := settleLines(ctx, render.NewClaudeCLIWith(model, ""), string(text))
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fail(err)
	}
	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	if err := os.WriteFile(filepath.Join(outDir, from+".txt"), []byte(out), 0o644); err != nil {
		return fail(err)
	}
	fmt.Fprintf(os.Stderr, "dieg: settle %s: %d lines\n", from, len(lines))
	return 0
}
