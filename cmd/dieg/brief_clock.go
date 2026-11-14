package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

// span is a value at the episode start and at its end; maybe is an end value that holds only if the episode
// crosses a day boundary the timeline leaves open.
type span struct{ start, end, maybe string }

// nowSection compiles "where the story stands" (S14) and the series frame lines (S31, S33):
//   - clocks: `attr <term>.clock = <event>` — story days to that event, at the first scene's start and the last
//     scene's end, as day numbers (S27). A day passes for certain only when the end's earliest day is later; when
//     only its latest day is, the drop is offered as a possibility.
//   - counters: the work's project rules (--rules dir) that report per act or scene. Their numeric `data` values
//     (last report before the episode, last one inside it) are the counter values.
//   - frames: `attr <term>.frame_open` / `frame_close` templates with {key} placeholders (a clock's term id, a
//     counter's data key) are filled in, so the writer gets finished lines instead of a format example.
func nowSection(ctx context.Context, pool *pgxpool.Pool, work, from, to, rulesDir string, first bool) (string, error) {
	var b strings.Builder
	vals := map[string]span{}
	rows, err := pool.Query(ctx, epSQL+`, last AS (SELECT s.end_pt FROM scene s JOIN o ON o.scene_id = s.id
		    WHERE s.id IN (SELECT scene_id FROM ep) ORDER BY o.ord DESC LIMIT 1)
		SELECT render_name($1, a.entity_id, 'ko'), lint_short(a.entity_id), pe.earliest / 86400 - ps.earliest / 86400,
		       pe.earliest / 86400 - coalesce(pl.earliest, ps.earliest) / 86400,
		       pe.earliest / 86400 - coalesce(pl.latest, pl.earliest, ps.earliest) / 86400
		FROM attribute a JOIN point pe ON pe.work_id = $1 AND (pe.id = a.value || '.start' OR pe.id LIKE '%/' || a.value || '.start')
		JOIN scene s ON s.work_id = $1 AND (s.id = $2 OR lint_short(s.id) = $2) JOIN point ps ON ps.id = s.start_pt
		LEFT JOIN point pl ON pl.id = (SELECT * FROM last)
		WHERE a.work_id = $1 AND a.key = 'clock' AND pe.earliest >= ps.earliest ORDER BY 3`, work, from, to)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var n, key string
		var d, e, m int64
		if err := rows.Scan(&n, &key, &d, &e, &m); err != nil {
			rows.Close()
			return "", err
		}
		v := span{start: fmt.Sprint(d), end: fmt.Sprint(e)}
		switch {
		case e < d:
			fmt.Fprintf(&b, "- %s: 이 화 시작 때 마감까지 %d일 → 이 화 끝에는 %d일\n", n, d, e)
		case m < d:
			v.maybe = fmt.Sprint(m)
			fmt.Fprintf(&b, "- %s: 이 화 시작 때 마감까지 %d일. 이 화가 자정을 넘기면 끝에는 %d일(설계가 시각을 정하지 않았다 — 넘기는 쪽으로 쓰면 그 값으로 닫는다)\n", n, d, m)
		default:
			fmt.Fprintf(&b, "- %s: 이 화 시작 때 마감까지 %d일 (이 화 안에서 날이 바뀌지 않는다)\n", n, d)
		}
		vals[key] = v
	}
	rows.Close()
	if rulesDir != "" {
		if err := counters(ctx, pool, work, from, to, rulesDir, &b, vals); err != nil {
			return "", err
		}
	}
	if err := frames(ctx, pool, work, &b, vals, first); err != nil {
		return "", err
	}
	return b.String(), nil
}

// counters reads the project rules' numeric data per act/scene into vals and prints one line per rule.
func counters(ctx context.Context, pool *pgxpool.Pool, work, from, to, rulesDir string, b *strings.Builder, vals map[string]span) error {
	x := os.DirFS(rulesDir)
	mine, err := pg.Rules(x)
	if err != nil {
		return err
	}
	summary := map[string]string{}
	for _, r := range mine {
		summary[r.ID] = r.Summary
	}
	ds, err := pg.Lint(ctx, pool, work, rules.FS, []fs.FS{x}...)
	if err != nil {
		return err
	}
	ord := map[string]int{}
	o, err := pool.Query(ctx, `SELECT scene_id, ord FROM scene_told_ord($1)`, work)
	if err != nil {
		return err
	}
	for o.Next() {
		var s string
		var n int
		if err := o.Scan(&s, &n); err != nil {
			o.Close()
			return err
		}
		ord[s] = n
	}
	o.Close()
	lo, hi := toldOrd(ctx, pool, work, from), toldOrd(ctx, pool, work, to)
	type pick struct {
		ord  int
		data map[string]any
	}
	start, inside, first := map[string]pick{}, map[string]pick{}, map[string]pick{}
	for _, d := range ds {
		if _, ok := summary[d.Rule]; !ok || (d.TargetKind != "act" && d.TargetKind != "scene") {
			continue
		}
		sc, _, _ := strings.Cut(d.TargetID, "#")
		n, ok := ord[sc]
		if !ok || n >= hi {
			continue
		}
		m := start
		if n >= lo {
			m = inside
		}
		var data map[string]any
		json.Unmarshal(d.Data, &data)
		if p, seen := m[d.Rule]; !seen || n >= p.ord {
			m[d.Rule] = pick{n, data}
		}
		if p, seen := first[d.Rule]; d.TargetKind == "scene" && n >= lo && (!seen || n < p.ord) { // state counters only; an act counter starts at none
			first[d.Rule] = pick{n, data}
		}
	}
	// A state counter (reported per scene) with no report before the episode starts at its first report inside it (an episode-1 status line
	// shows the value the reader first sees, not "none").
	for id, p := range first {
		if _, ok := start[id]; !ok {
			start[id] = p
		}
	}
	var ids []string
	for id := range summary {
		_, a := start[id]
		_, c := inside[id]
		if a || c {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		var parts []string
		keys := map[string]bool{}
		for k := range start[id].data {
			keys[k] = true
		}
		for k := range inside[id].data {
			keys[k] = true
		}
		var ks []string
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			s, sok := num(start[id].data[k])
			e, eok := num(inside[id].data[k])
			if !sok && !eok {
				continue // not a number (e.g. a list of bodies): not a counter
			}
			if !eok {
				e = s
			}
			if !sok {
				s = "없음"
			}
			vals[k] = span{start: s, end: e}
			if s == e {
				parts = append(parts, fmt.Sprintf("%s %s (이 화에서 바뀌지 않는다)", k, s))
			} else {
				parts = append(parts, fmt.Sprintf("%s %s → %s", k, s, e))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(b, "- %s: %s\n", summary[id], strings.Join(parts, ", "))
		}
	}
	return nil
}

// num renders a JSON number as an integer string.
func num(v any) (string, bool) {
	f, ok := v.(float64)
	if !ok {
		return "", false
	}
	return fmt.Sprint(int64(f)), true
}

// frames fills `frame_open` with start values and each `frame_close` with end values; a closing line whose values
// did not change in this episode is not offered as the closing line (S31).
func frames(ctx context.Context, pool *pgxpool.Pool, work string, b *strings.Builder, vals map[string]span, first bool) error {
	rows, err := pool.Query(ctx, `SELECT a.key, a.value FROM attribute a WHERE a.work_id = $1 AND a.key IN ('frame_open', 'frame_close') ORDER BY a.key DESC, a.id`, work)
	if err != nil {
		return err
	}
	defer rows.Close()
	fill := func(t string, end, maybe bool) (string, bool) {
		changed := false
		for k, v := range vals {
			x := v.start
			if end {
				x = v.end
				if maybe && v.maybe != "" {
					x = v.maybe
				}
				changed = changed || (strings.Contains(t, "{"+k+"}") && x != v.start)
			}
			t = strings.ReplaceAll(t, "{"+k+"}", x)
		}
		return t, changed
	}
	var closes []string
	certain := false
	haveClose := false
	for rows.Next() {
		var k, t string
		if err := rows.Scan(&k, &t); err != nil {
			return err
		}
		if k == "frame_open" {
			s, _ := fill(t, false, false)
			if first { // S39: the device's first appearance may withhold its values and reveal them in the episode
				fmt.Fprintf(b, "- 여는 줄: `%s` — 이 장치가 처음 나오는 화다. 값 칸을 ▒로 가려 열어도 된다 — 가린 칸은 화자도 아직 못 읽고 세지도 못한다(줄이 흔들리거나 덮여서). 이 화의 마지막 장면에서 화자가 처음 읽는다. 닫는 줄은 글자 그대로\n", s)
			} else {
				fmt.Fprintf(b, "- 여는 줄(그대로 쓴다): `%s`\n", s)
			}
			continue
		}
		haveClose = true
		if s, ch := fill(t, true, false); ch {
			closes = append(closes, fmt.Sprintf("`%s`", s))
			certain = true
		} else if s, ch := fill(t, true, true); ch {
			closes = append(closes, fmt.Sprintf("`%s` (자정을 넘기는 쪽으로 쓸 때)", s))
		}
	}
	switch {
	case len(closes) > 0:
		if !certain { // S35/P16: the timeline leaves the day open; the default branch comes first, the frame must not decide it
			b.WriteString("- 닫는 줄: 장면이 자정을 넘기지 않으면 족보 창 줄 없이 장면의 마지막 행동으로 닫는다(기본). 닫는 줄을 위해 자정을 끼워 넣지 않는다\n")
			fmt.Fprintf(b, "  · 장면 흐름이 자연히 자정을 넘길 때만: %s\n", strings.Join(closes, " / "))
		} else {
			fmt.Fprintf(b, "- 닫는 줄 후보(이 화에서 바뀐 값, 하나를 고른다): %s\n", strings.Join(closes, " / "))
		}
	case haveClose:
		b.WriteString("- 닫는 줄: 이 화에서 바뀌는 값이 없다(설계 경고: 작품 고정 장치는 매 화 한 값이 바뀌기를 요구한다). 닫는 줄을 여는 줄과 같은 값으로 되풀이하지 않는다 — 족보 창 줄 없이 장면의 마지막 행동으로 닫는다\n")
	}
	return rows.Err()
}
