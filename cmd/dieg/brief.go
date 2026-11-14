package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// briefText compiles the writer's brief for one episode: what the reader already holds before `from` (do not
// re-explain), what this episode must hand over (facts first known by `to`, exclusive; omitted when to == ""), and
// what stays hidden (facts true in the work the reader still lacks afterwards). The design code, if given, follows.
// The brief is for an episode writer, not a renderer: it fixes knowledge, never sentences.
func briefText(ctx context.Context, pool *pgxpool.Pool, work, from, to, code string) (string, error) {
	before, err := readerState(ctx, pool, work, from)
	if err != nil {
		return "", err
	}
	after := before
	if to != "" {
		if after, err = readerState(ctx, pool, work, to); err != nil {
			return "", err
		}
	}
	known := map[string]bool{}
	for _, f := range after.Facts {
		known[f.Text] = true
	}
	had := map[string]bool{}
	for _, f := range before.Facts {
		had[f.Text] = true
	}
	var b strings.Builder
	b.WriteString("# 회차 설계서\n\n")
	b.WriteString("## ① 독자가 이미 아는 것 — 다시 설명하지 않는다\n")
	if t := before.Text(); t != "" {
		b.WriteString(t)
	} else {
		b.WriteString("(없음: 첫 화다)\n")
	}
	if to != "" {
		b.WriteString("\n## ② 이 화가 독자에게 넘길 것 — 장면으로 짐작하게 한다. 문장을 그대로 옮기지 않는다\n")
		for _, f := range after.Facts {
			if !had[f.Text] {
				fmt.Fprintf(&b, "- %s\n", f.Text)
			}
		}
	}
	b.WriteString("\n## ③ 밝히지 않는다 — 이 화 끝까지 독자가 모르는 진실\n")
	rows, err := pool.Query(ctx, `SELECT render_fact_text($1, id) FROM fact WHERE work_id = $1 AND render_fact_true($1, id) ORDER BY id`, work)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return "", err
		}
		if !known[t] {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	b.WriteString("\n## ④ 쓰기 규칙\n- 회차 번호·'지난 화' 같은 연재 바깥 말을 원고에 쓰지 않는다.\n- 설계서의 사실 문장·용어 풀이를 원고에 옮겨 적지 않는다. 인물의 행동·대사·결과로 보인다.\n- ①에 없는 것은 시점 인물도 독자도 모른다. 추측은 추측으로만 쓴다.\n")
	if code != "" {
		b.WriteString("\n## ⑤ 장면 설계 (디제시스 코드) — 장면 순서·장소·행위·등장/퇴장·누가 무엇을 아는지를 따른다. 감각·몸짓·세부는 자유\n")
		b.WriteString(code)
	}
	return b.String(), nil
}

// cmdBrief runs `dieg brief <work|dir> --from <scene> [--to <scene>] [--code <file>]`.
func cmdBrief(args []string) int {
	var target, from, to, codeFile string
	flags := map[string]*string{"--from": &from, "--to": &to, "--code": &codeFile}
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
	if target == "" || from == "" {
		return usage()
	}
	code := ""
	if codeFile != "" {
		c, err := os.ReadFile(codeFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
		code = string(c)
	}
	work := target
	if _, err := os.Stat(target); err == nil {
		var err error
		if work, err = load(target, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer pool.Close()
	s, err := briefText(ctx, pool, work, from, to, code)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	fmt.Print(s)
	return 0
}
