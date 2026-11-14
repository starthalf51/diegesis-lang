package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

// nowSection compiles "where the story stands" at the episode start (S14): clocks and the work's own counters.
//   - clocks: `attr <term>.clock = <event>` — days from the episode's first scene to that event's start.
//   - counters: the work's project rules (--rules dir) that report per act or scene; the last message before the
//     episode and the last one inside it are the counter's value at the start and after this episode.
func nowSection(ctx context.Context, pool *pgxpool.Pool, work, from, to, rulesDir string) (string, error) {
	var b strings.Builder
	// S25/S27: days left at the start and at the last scene end, counted as story-day numbers (earliest / 86400), so a
	// few-hour episode does not lose a day and a day passes only when the end certainly falls on a later day.
	rows, err := pool.Query(ctx, epSQL+`, last AS (SELECT s.end_pt FROM scene s JOIN o ON o.scene_id = s.id
		    WHERE s.id IN (SELECT scene_id FROM ep) ORDER BY o.ord DESC LIMIT 1)
		SELECT render_name($1, a.entity_id, 'ko'), pe.earliest / 86400 - ps.earliest / 86400,
		       pe.earliest / 86400 - coalesce(pl.earliest, ps.earliest) / 86400 -- S27: day numbers, not seconds; a day passes only if certain
		FROM attribute a JOIN point pe ON pe.work_id = $1 AND (pe.id = a.value || '.start' OR pe.id LIKE '%/' || a.value || '.start')
		JOIN scene s ON s.work_id = $1 AND (s.id = $2 OR lint_short(s.id) = $2) JOIN point ps ON ps.id = s.start_pt
		LEFT JOIN point pl ON pl.id = (SELECT * FROM last)
		WHERE a.work_id = $1 AND a.key = 'clock' AND pe.earliest >= ps.earliest ORDER BY 2`, work, from, to)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var n string
		var d, e int64
		if err := rows.Scan(&n, &d, &e); err != nil {
			rows.Close()
			return "", err
		}
		if e < d {
			fmt.Fprintf(&b, "- %s: 이 화 시작 때 마감까지 %d일 → 이 화 끝에는 %d일\n", n, d, e)
		} else {
			fmt.Fprintf(&b, "- %s: 이 화 시작 때 마감까지 %d일 (이 화 안에서 날이 바뀌지 않는다)\n", n, d)
		}
	}
	rows.Close()
	if rulesDir == "" {
		return b.String(), nil
	}
	x := os.DirFS(rulesDir)
	mine, err := pg.Rules(x)
	if err != nil {
		return "", err
	}
	summary := map[string]string{}
	for _, r := range mine {
		summary[r.ID] = r.Summary
	}
	ds, err := pg.Lint(ctx, pool, work, rules.FS, []fs.FS{x}...)
	if err != nil {
		return "", err
	}
	ord := map[string]int{}
	o, err := pool.Query(ctx, `SELECT scene_id, ord FROM scene_told_ord($1)`, work)
	if err != nil {
		return "", err
	}
	for o.Next() {
		var s string
		var n int
		if err := o.Scan(&s, &n); err != nil {
			o.Close()
			return "", err
		}
		ord[s] = n
	}
	o.Close()
	lo, hi := toldOrd(ctx, pool, work, from), toldOrd(ctx, pool, work, to)
	type pick struct {
		ord int
		msg string
	}
	start, inside := map[string]pick{}, map[string]pick{}
	var ids []string
	for _, d := range ds {
		if _, ok := summary[d.Rule]; !ok || (d.TargetKind != "act" && d.TargetKind != "scene") {
			continue
		}
		sc, _, _ := strings.Cut(d.TargetID, "#")
		n, ok := ord[sc]
		if !ok {
			continue
		}
		m := start
		if n >= lo && n < hi {
			m = inside
		} else if n >= hi {
			continue
		}
		if p, seen := m[d.Rule]; !seen || n >= p.ord {
			m[d.Rule] = pick{n, d.Message}
		}
	}
	for id := range summary {
		_, a := start[id]
		_, c := inside[id]
		if a || c {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		s, in := start[id].msg, inside[id].msg
		if s == "" {
			s = "(아직 없음)"
		}
		if in == "" {
			in = "(이 화에서 바뀌지 않는다)"
		}
		fmt.Fprintf(&b, "- %s: 시작 %s → 이 화 끝 %s\n", summary[id], s, in)
	}
	return b.String(), nil
}

// povOf is the POV character of scene `from` (its `render pov` directive), or "".
func povOf(ctx context.Context, pool *pgxpool.Pool, work, from string) string {
	var id string
	pool.QueryRow(ctx, `SELECT e.id FROM directive d JOIN scene s ON s.id = d.scene_id JOIN entity e ON e.work_id = $1 AND lint_short(e.id) = d."values"[1]
		WHERE d.work_id = $1 AND d.key = 'pov' AND (s.id = $2 OR lint_short(s.id) = $2) LIMIT 1`, work, from).Scan(&id)
	return id
}

// spokenLines lists quoted speech in told positions [lo, hi): speaker → audience (place). A line spoken where the POV
// character was neither present nor addressed is marked, so the writer does not hand it to the narrator (S21).
// Addressees (`to`, else the characters the act is aimed at) and overhearers (the rest of the audience) are kept
// apart (S22); addressee lists above three are shortened to the first name and a count: "everyone" is the point.
func spokenLines(ctx context.Context, pool *pgxpool.Pool, work string, lo, hi int, pov string) (string, error) {
	rows, err := pool.Query(ctx, `WITH x AS (SELECT a.*, CASE WHEN cardinality(a."to") > 0 THEN a."to" ELSE ARRAY(SELECT y FROM unnest(a.objects) y
		    JOIN entity e ON e.id = y AND e.kind = 'character') END AS addr FROM act a WHERE a.work_id = $1)
		SELECT render_name($1, a.actor_id, 'ko'),
		  coalesce((SELECT array_agg(render_name($1, y, 'ko')) FROM unnest(a.addr) y), '{}'),
		  coalesce((SELECT array_agg(render_name($1, au.entity_id, 'ko') ORDER BY au.entity_id) FROM audience au WHERE au.act_id = a.id
		    AND au.entity_id <> ALL (a.addr) AND au.entity_id IS DISTINCT FROM a.actor_id), '{}'),
		  coalesce(render_name($1, s.place_id, 'ko'), ''), a.quote,
		  $4 = '' OR a.actor_id = $4 OR $4 = ANY (a."to") OR EXISTS (SELECT 1 FROM audience au WHERE au.act_id = a.id AND au.entity_id = $4)
		     OR EXISTS (SELECT 1 FROM presence p WHERE p.scene_id = a.scene_id AND p.entity_id = $4)
		FROM x a JOIN scene s ON s.id = a.scene_id JOIN scene_told_ord($1) o ON o.scene_id = a.scene_id
		WHERE a.work_id = $1 AND coalesce(a.quote, '') <> '' AND a.verb IN ('says', 'tells', 'asks', 'lies', 'decrees')
		  AND o.ord >= $2 AND o.ord < $3 ORDER BY o.ord, a.ord`, work, lo, hi, pov)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var who, place, q string
		var to, near []string
		var heard bool
		if err := rows.Scan(&who, &to, &near, &place, &q, &heard); err != nil {
			return "", err
		}
		aud := strings.Join(to, ", ")
		if len(to) > 3 {
			aud = fmt.Sprintf("%s 외 %d명 (%d명 모두에게 한꺼번에)", to[0], len(to)-1, len(to))
		}
		line := "- " + who
		if aud != "" {
			line += " → " + aud
		}
		if place != "" {
			line += " (" + place + ")"
		}
		if len(near) > 0 {
			line += " [곁에서 들음: " + strings.Join(near, ", ") + "]"
		}
		line += fmt.Sprintf(": \"%s\"", q)
		if !heard {
			line += " — 시점 인물은 이 자리에 없었다"
		}
		b.WriteString(line + "\n")
	}
	return b.String(), rows.Err()
}

// dateFacts prefixes each "알려진 사실:" line of a reader-state text with the story day the reader learned it (S18).
func dateFacts(ctx context.Context, pool *pgxpool.Pool, work, text string) string {
	day := map[string]int64{}
	rows, err := pool.Query(ctx, `SELECT DISTINCT ON (k.fact_id) render_fact_text($1, k.fact_id), coalesce(p.earliest, 0) / 86400 + 1
		FROM reader_knows($1) k JOIN scene s ON s.work_id = $1 AND s.discourse_ord = k.from_ord JOIN point p ON p.id = s.start_pt
		ORDER BY k.fact_id, k.from_ord`, work)
	if err != nil {
		return text
	}
	for rows.Next() {
		var t string
		var d int64
		if rows.Scan(&t, &d) == nil {
			day[t] = d
		}
	}
	rows.Close()
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(l, "알려진 사실: "); ok {
			if d, ok := day[strings.TrimPrefix(rest, "(사실 아님) ")]; ok {
				l = fmt.Sprintf("알려진 사실(%d일째): %s", d, rest)
			}
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// maskNames applies what the reader can call each character at `from` (S20, S16):
//   - `attr <character>.known_as = "..."` (latest one set before the episode) replaces the author's name everywhere;
//   - a character the reader has neither met on stage nor seen named in `held` (the reader-state text) keeps no name
//     or identifier in the brief ("이름을 모르는 인물").
func maskNames(ctx context.Context, pool *pgxpool.Pool, work, from string, endOrd int, held, s string) (string, error) {
	rows, err := pool.Query(ctx, `WITH st AS (SELECT s.start_pt FROM scene s WHERE s.work_id = $1 AND (s.id = $2 OR lint_short(s.id) = $2))
		SELECT render_name($1, e.id, 'ko'), lint_short(e.id),
		  coalesce((SELECT a.value FROM attribute a LEFT JOIN point p ON p.id = a.at_pt WHERE a.work_id = $1 AND a.entity_id = e.id AND a.key = 'known_as'
		     AND (coalesce(a.at_pt, '') = '' OR precedes($1, a.at_pt, (SELECT * FROM st)) = 'yes') ORDER BY p.earliest DESC NULLS LAST LIMIT 1), ''),
		  EXISTS (SELECT 1 FROM presence p JOIN scene_told_ord($1) o ON o.scene_id = p.scene_id WHERE p.entity_id = e.id AND o.ord < $3)
		FROM entity e WHERE e.work_id = $1 AND e.kind = 'character'`, work, from, endOrd)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var pairs, ids []string
	for rows.Next() {
		var name, short, alias string
		var met bool
		if err := rows.Scan(&name, &short, &alias, &met); err != nil {
			return "", err
		}
		switch {
		case alias != "":
			pairs = append(pairs, name, alias)
		case !met && !strings.Contains(held, name):
			pairs = append(pairs, name, "(이름을 모르는 인물)")
			ids = append(ids, regexp.QuoteMeta(short))
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	// ponytail: names are plain substrings (a name inside another word is replaced too); ids are whole words.
	s = strings.NewReplacer(pairs...).Replace(s)
	if len(ids) > 0 {
		s = regexp.MustCompile(`\b(`+strings.Join(ids, "|")+`)\b`).ReplaceAllString(s, "(이름을 모르는 인물)")
	}
	return s, nil
}

// placeCards lists the places of the episode's scenes with `attr <place>.look` (what the senses meet there) and
// `attr <place>.talk` (what people there say to each other: the world's arithmetic in their mouths, P10).
func placeCards(ctx context.Context, pool *pgxpool.Pool, work, from, to string) (string, error) {
	rows, err := pool.Query(ctx, epSQL+`
		SELECT render_name($1, s.place_id, 'ko'), coalesce(min(a.value) FILTER (WHERE a.key = 'look'), ''),
		       coalesce(min(a.value) FILTER (WHERE a.key = 'talk'), '') FROM scene s JOIN ep ON ep.scene_id = s.id
		  LEFT JOIN attribute a ON a.work_id = $1 AND a.entity_id = s.place_id AND a.key IN ('look', 'talk')
		 WHERE s.place_id IS NOT NULL GROUP BY s.place_id ORDER BY min(s.discourse_ord)`, work, from, to)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var n, look, talk string
		if err := rows.Scan(&n, &look, &talk); err != nil {
			return "", err
		}
		line := "- " + n
		if look != "" {
			line += " — " + look
		}
		if talk != "" {
			line += " / 그 자리 사람들이 하는 말: " + talk
		}
		b.WriteString(line + "\n")
	}
	return b.String(), rows.Err()
}

// povLived is what the POV character came to believe in scenes told from their own point of view before `from`
// (S24): in those scenes the reader stood behind their eyes, so it is shared knowledge, not a secret to keep.
func povLived(ctx context.Context, pool *pgxpool.Pool, work, from, pov string) ([]string, error) {
	if pov == "" {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `SELECT DISTINCT render_fact_text($1, bl.prop_id) FROM believes bl
		JOIN scene_told_ord($1) o ON o.scene_id = regexp_replace(bl.from_pt, '(#.*|\.(start|end))$', '')
		JOIN directive d ON d.work_id = $1 AND d.scene_id = o.scene_id AND d.key = 'pov' AND d."values"[1] = lint_short($3)
		WHERE bl.work_id = $1 AND bl.entity_id = $3 AND bl.value AND o.ord < $2 ORDER BY 1`, work, toldOrd(ctx, pool, work, from), pov)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// forgedFacts is the set of fact texts tagged `forgery`: what a forged record says, not what is true.
func forgedFacts(ctx context.Context, pool *pgxpool.Pool, work string) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT render_fact_text($1, id) FROM fact WHERE work_id = $1 AND tag = 'forgery'`, work)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		m[t] = true
	}
	return m, rows.Err()
}

// forgedLabel marks a forged record's content once, the same way wherever it appears (S30).
func forgedLabel(forged map[string]bool, t string) string {
	if forged[t] {
		return "기록상: " + t + " (위조된 기록, 실제와 다르다)"
	}
	return t
}

// labelForgedLines applies forgedLabel to the "알려진 사실…: X" lines of a reader-state text.
func labelForgedLines(forged map[string]bool, text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if head, rest, ok := strings.Cut(l, ": "); ok && strings.HasPrefix(head, "알려진 사실") {
			lines[i] = head + ": " + forgedLabel(forged, rest)
		}
	}
	return strings.Join(lines, "\n")
}

// newTerms lists the terms first explained in the episode with their gloss and `explain … via <mode>` (S28).
func newTerms(ctx context.Context, pool *pgxpool.Pool, work, from, to string, before readerSt) (string, error) {
	had := map[string]bool{}
	for _, t := range before.Terms {
		had[t["surface"]] = true
	}
	rows, err := pool.Query(ctx, epSQL+`
		SELECT render_name($1, t.term, 'ko'), coalesce(g.value, ''), coalesce(min(v.mode), '')
		  FROM directive d JOIN ep ON ep.scene_id = d.scene_id CROSS JOIN LATERAL unnest(d."values") AS t(term)
		  LEFT JOIN attribute g ON g.id = t.term || '#gloss'
		  LEFT JOIN LATERAL (SELECT v."values"[1] AS mode FROM directive v WHERE v.work_id = $1 AND v.scene_id = d.scene_id
		       AND v.key = 'explain.via' AND t.term = ANY (v."values"[2:])) v ON true
		 WHERE d.work_id = $1 AND d.key = 'explain'
		 GROUP BY t.term, g.value ORDER BY min(d.scene_id), t.term`, work, from, to)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	how := map[string]string{"sense": "감각으로", "consequence": "결과로", "dialogue": "대사로"}
	var b strings.Builder
	for rows.Next() {
		var n, gloss, mode string
		if err := rows.Scan(&n, &gloss, &mode); err != nil {
			return "", err
		}
		if had[n] {
			continue
		}
		if m := how[mode]; m != "" {
			n += " (" + m + ")"
		}
		fmt.Fprintf(&b, "- %s: %s\n", n, gloss)
	}
	return b.String(), rows.Err()
}
