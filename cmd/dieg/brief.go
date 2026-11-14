package main

import (
	"context"
	"errors"
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
func briefText(ctx context.Context, pool *pgxpool.Pool, work, from, to, code, style string) (string, error) {
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
	// The brief is the writer's only source of knowledge (S12): no concept document rides beside it.
	b.WriteString("너는 연재 소설 작가다. 아래 회차 설계서만으로 이 화 원고를 쓴다. 설계서에 없는 설정·과거·비밀을 지어내지 않는다. 감각·몸짓·짧은 대사·생활 세부는 자유롭게 쓴다.\n\n")
	if style != "" {
		b.WriteString("## 문체\n" + style + "\n\n")
	}
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
	// Only truths this episode touches: believed by someone on stage, or carried by an act in its scenes. Far-future
	// truths are not given to the writer at all; a writer who does not know cannot leak (S11).
	// Beliefs count only once held by the episode end (from_pt scene told no later than the last scene of the range).
	b.WriteString("\n## ③ 밝히지 않는다 — 이 화 인물이 쥐고 있지만 독자는 아직 모르는 진실. 인물은 이것을 알고 숨기며 움직인다\n")
	rows, err := pool.Query(ctx, `WITH o AS (SELECT * FROM scene_told_ord($1)),
		ep AS (SELECT o.scene_id FROM o WHERE o.ord >= (SELECT o2.ord FROM o o2 JOIN scene x ON x.id = o2.scene_id
		         WHERE x.id = $2 OR lint_short(x.id) = $2)
		       AND ($3 = '' OR o.ord < (SELECT o3.ord FROM o o3 JOIN scene y ON y.id = o3.scene_id WHERE y.id = $3 OR lint_short(y.id) = $3))),
		cast_ AS (SELECT DISTINCT p.entity_id FROM presence p JOIN entity e ON e.id = p.entity_id
		          WHERE p.work_id = $1 AND e.kind = 'character' AND p.scene_id IN (SELECT scene_id FROM ep)),
		bl_ AS (SELECT bl.* FROM believes bl WHERE bl.work_id = $1 AND bl.entity_id IN (SELECT * FROM cast_)
		        AND coalesce((SELECT o.ord FROM o WHERE o.scene_id = split_part(bl.from_pt, '#', 1)), 0) <= (SELECT max(o.ord) FROM o WHERE o.scene_id IN (SELECT scene_id FROM ep)))
		SELECT render_fact_text($1, f.id), coalesce((SELECT string_agg(DISTINCT render_name($1, bl.entity_id, 'ko'), ', ')
		         FROM bl_ bl WHERE bl.prop_id = f.id AND bl.value), '')
		  FROM fact f WHERE f.work_id = $1 AND render_fact_true($1, f.id)
		   AND (EXISTS (SELECT 1 FROM bl_ bl WHERE bl.prop_id = f.id)
		     OR EXISTS (SELECT 1 FROM act a WHERE a.work_id = $1 AND a.prop_id = f.id AND a.scene_id IN (SELECT scene_id FROM ep)))
		 ORDER BY f.id`, work, from, to)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var t, who string
		if err := rows.Scan(&t, &who); err != nil {
			return "", err
		}
		if known[t] {
			continue
		}
		if who != "" {
			fmt.Fprintf(&b, "- %s — 아는 인물: %s\n", t, who)
		} else {
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

// cmdBrief runs `dieg brief <work|dir> --from <scene> [--to <scene>] [--code <file>] [--style <file>]`.
func cmdBrief(args []string) int {
	var target, from, to, codeFile, styleFile string
	flags := map[string]*string{"--from": &from, "--to": &to, "--code": &codeFile, "--style": &styleFile}
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
	code, err1 := readOpt(codeFile)
	style, err2 := readOpt(styleFile)
	if err := errors.Join(err1, err2); err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
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
	s, err := briefText(ctx, pool, work, from, to, code, style)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	fmt.Print(s)
	return 0
}

// readOpt reads path, or returns "" when path is empty.
func readOpt(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	return string(b), err
}
