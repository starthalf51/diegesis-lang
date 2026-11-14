package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	// S16/S29: a known_as replaces the name and the code identifier alike; each unmet character gets its own tag,
	// so three hidden people stay three people.
	var pairs []string
	idTo := map[string]string{}
	unmet := 0
	for rows.Next() {
		var name, short, alias string
		var met bool
		if err := rows.Scan(&name, &short, &alias, &met); err != nil {
			return "", err
		}
		switch {
		case alias != "":
			pairs = append(pairs, name, alias)
			idTo[short] = alias
		case !met && !strings.Contains(held, name):
			unmet++
			tag := fmt.Sprintf("(이름을 모르는 인물 %d)", unmet)
			pairs = append(pairs, name, tag)
			idTo[short] = tag
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	// ponytail: names are plain substrings (a name inside another word is replaced too); ids are whole words.
	s = strings.NewReplacer(pairs...).Replace(s)
	if len(idTo) > 0 {
		var alt []string
		for id := range idTo {
			alt = append(alt, regexp.QuoteMeta(id))
		}
		sort.Strings(alt)
		s = regexp.MustCompile(`\b(`+strings.Join(alt, "|")+`)\b`).ReplaceAllStringFunc(s, func(m string) string { return idTo[m] })
	}
	return s, nil
}

// placeCards lists the places of the episode's scenes with `attr <place>.look` (what the senses meet there) and
// `attr <place>.talk` (what people there say to each other: the world's arithmetic in their mouths, P10).
func placeCards(ctx context.Context, pool *pgxpool.Pool, work, from, to string) (string, error) {
	rows, err := pool.Query(ctx, epSQL+`
		SELECT render_name($1, s.place_id, 'ko'), coalesce(min(a.value) FILTER (WHERE a.key = 'look'), ''),
		       coalesce(min(a.value) FILTER (WHERE a.key = 'talk'), ''),
		       EXISTS (SELECT 1 FROM scene sx JOIN o ox ON ox.scene_id = sx.id WHERE sx.place_id = s.place_id AND ox.ord < (SELECT min(o.ord) FROM o JOIN ep ON ep.scene_id = o.scene_id)),
		       array_agg(DISTINCT lint_short(s.id))
		  FROM scene s JOIN ep ON ep.scene_id = s.id
		  LEFT JOIN attribute a ON a.work_id = $1 AND a.entity_id = s.place_id AND a.key IN ('look', 'talk')
		 WHERE s.place_id IS NOT NULL GROUP BY s.place_id ORDER BY min(s.discourse_ord)`, work, from, to)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var n, look, talk string
		var again bool
		var scenes []string
		if err := rows.Scan(&n, &look, &talk, &again, &scenes); err != nil {
			return "", err
		}
		line := "- " + n
		switch {
		case again: // P7: a revisited place is not painted again from the card
			line += " — 다시 찾은 장소. 처음처럼 다시 그리지 않는다. 이번에 달라진 것 하나만 쓴다"
		case look != "":
			line += " — " + dealLook(look, scenes)
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

// newTerms lists the terms first explained in the episode with their gloss and `explain … via <mode>` (S28), the
// scene that carries each, and the event that shows it (`attr <term>.shown_by = <fact>`, S32). A term with no such
// event is flagged: nothing in the scene makes it mean anything, so it would arrive as a bare label.
func newTerms(ctx context.Context, pool *pgxpool.Pool, work, from, to string, before readerSt) (string, error) {
	had := map[string]bool{}
	for _, t := range before.Terms {
		had[t["surface"]] = true
	}
	rows, err := pool.Query(ctx, epSQL+`
		SELECT render_name($1, t.term, 'ko'), coalesce(g.value, ''), coalesce(min(v.mode), ''), lint_short(min(d.scene_id)),
		       coalesce((SELECT string_agg(render_fact_text($1, sb.value), ' / ') FROM attribute sb
		                  WHERE sb.work_id = $1 AND sb.entity_id = t.term AND sb.key = 'shown_by'), '')
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
		var n, gloss, mode, scene, shown string
		if err := rows.Scan(&n, &gloss, &mode, &scene, &shown); err != nil {
			return "", err
		}
		if had[n] {
			continue
		}
		if m := how[mode]; m != "" {
			n += " (" + m + ")"
		}
		ev := "⚠ 보여 줄 사건이 지정되지 않았다 — 장면의 다른 일(절차의 진행, 사람들의 반응)로 뜻을 세우거나, 뜻 없이 이름만 스친다"
		if shown != "" {
			ev = "보여 줄 사건: " + shown
		}
		fmt.Fprintf(&b, "- %s — 장면 %s · %s\n  풀이(참고): %s\n", n, scene, ev, gloss)
	}
	return b.String(), rows.Err()
}

// knowsLines is an off-POV character's knowledge that this episode's acts use (S34): each belief held by the
// episode end with its path — the act it came from, background (held before the story), or flagged when the code
// asserts it here with no path — plus what the character directly sees in the episode.
func knowsLines(ctx context.Context, pool *pgxpool.Pool, work, from, to, who string) (string, error) {
	rows, err := pool.Query(ctx, epSQL+`,
		endpt AS (SELECT s.end_pt FROM scene s JOIN o ON o.scene_id = s.id WHERE s.id IN (SELECT scene_id FROM ep) ORDER BY o.ord DESC LIMIT 1),
		used AS (SELECT DISTINCT a.prop_id FROM act a WHERE a.work_id = $1 AND a.scene_id IN (SELECT scene_id FROM ep) AND a.prop_id IS NOT NULL)
		SELECT render_name($1, $4, 'ko'), render_fact_text($1, bl.prop_id),
		       CASE WHEN ca.id IS NOT NULL THEN lint_short(ca.scene_id) || ' ' || coalesce(render_name($1, ca.actor_id, 'ko'), '') || ' ' || ca.verb
		            WHEN bl.from_pt IS NULL OR bl.from_pt NOT LIKE '%#%' AND regexp_replace(bl.from_pt, '\.(start|end)$', '') NOT IN (SELECT scene_id FROM ep) THEN ''
		            ELSE '!' END,
		       bl.prop_id IN (SELECT * FROM used) AND EXISTS (SELECT 1 FROM act s WHERE s.work_id = $1 AND s.actor_id = $4 AND s.verb IN ('sees', 'reads')
		         AND s.prop_id = bl.prop_id AND s.scene_id IN (SELECT scene_id FROM ep))
		  FROM believes bl LEFT JOIN act ca ON ca.id = bl.cause_act
		 WHERE bl.work_id = $1 AND bl.entity_id = $4 AND bl.value AND bl.prop_id IN (SELECT * FROM used)
		   AND (bl.from_pt IS NULL OR bl.from_pt = (SELECT * FROM endpt) OR precedes($1, bl.from_pt, (SELECT * FROM endpt)) = 'yes')
		 ORDER BY 2`, work, from, to, who)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type line struct{ n, t, path string }
	var ls []line
	sees := false
	for rows.Next() {
		var l line
		var seen bool
		if err := rows.Scan(&l.n, &l.t, &l.path, &seen); err != nil {
			return "", err
		}
		if seen {
			l.path, sees = "이 화에서 직접 본다", true
		}
		ls = append(ls, l)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	// S38: reading a document in the episode is a path too (a checker reading a forged record).
	if !sees {
		pool.QueryRow(ctx, epSQL+` SELECT EXISTS (SELECT 1 FROM act a WHERE a.work_id = $1 AND a.actor_id = $4
			AND a.verb IN ('sees', 'reads') AND a.scene_id IN (SELECT scene_id FROM ep))`, work, from, to, who).Scan(&sees)
	}
	var b strings.Builder
	for _, l := range ls {
		switch {
		case l.path == "":
			l.path = "처음부터 알던 것"
		case l.path == "!" && sees:
			l.path = "이 화에서 직접 보거나 읽는 것에서 안다 — 장면이 그 순간을 보여 준다"
		case l.path == "!":
			l.path = "⚠ 경로가 설계에 없다 — 장면이 어떻게 아는지 보여 주거나, 아는 척하지 않는다"
		case !strings.HasPrefix(l.path, "이 화에서"):
			l.path = "경로: " + l.path
		}
		fmt.Fprintf(&b, "  · %s 아는 것: %s — %s\n", subj(l.n), l.t, l.path)
	}
	return b.String(), nil
}

// subj attaches the Korean subject particle 이/가 by the last syllable's final consonant (가 after a vowel or a
// non-Hangul ending such as a number read as is).
func subj(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) == 0 {
		return name
	}
	if c := r[len(r)-1]; c >= 0xAC00 && c <= 0xD7A3 && (c-0xAC00)%28 != 0 {
		return name + "이"
	}
	return name + "가"
}

// sceneBeats compiles each scene's `design` block into one line (S36, P15): goal, turn (the before-state is shown
// in this scene first, then broken) and the scene's own clock (what must happen before what).
func sceneBeats(ctx context.Context, pool *pgxpool.Pool, work, from, to string) (string, error) {
	rows, err := pool.Query(ctx, epSQL+`
		SELECT lint_short(o.scene_id), coalesce(render_name($1, s.place_id, 'ko'), ''),
		       coalesce((SELECT render_name($1, d."values"[1], 'ko') || ': ' || d."values"[2] FROM directive d WHERE d.scene_id = o.scene_id AND d.key = 'design.goal' LIMIT 1), ''),
		       coalesce((SELECT '"' || d."values"[1] || '" → "' || d."values"[2] || '"' FROM directive d WHERE d.scene_id = o.scene_id AND d.key = 'design.turn' LIMIT 1), ''),
		       coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = o.scene_id AND d.key = 'design.clock' LIMIT 1), ''),
		       coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = o.scene_id AND d.key = 'design.risk' LIMIT 1), '')
		  FROM o JOIN ep ON ep.scene_id = o.scene_id JOIN scene s ON s.id = o.scene_id ORDER BY o.ord`, work, from, to)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var id, place, goal, turn, clock, risk string
		if err := rows.Scan(&id, &place, &goal, &turn, &clock, &risk); err != nil {
			return "", err
		}
		if goal == "" && turn == "" && clock == "" && risk == "" {
			continue
		}
		var parts []string
		if goal != "" {
			parts = append(parts, "목표 — "+goal)
		}
		if turn != "" {
			parts = append(parts, "전환 "+turn+" (앞 상태를 이 장면에서 먼저 보이고 나서 깬다)")
		}
		if clock != "" {
			parts = append(parts, "장면 시계 — "+clock+" (진행을 장면 안에서 두세 번 보인다)")
		}
		if risk != "" { // P20: the price of acting, so the key act takes more than one line
			parts = append(parts, "위험 — "+risk+" (손을 쓰는 동안 그 위험이 보이게 한 박자 더 머문다)")
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", id, place, strings.Join(parts, " / "))
	}
	return b.String(), rows.Err()
}

// factMechanisms maps a fact text to the terms whose `shown_by` names it: the world devices that fact happens through.
func factMechanisms(ctx context.Context, pool *pgxpool.Pool, work string) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT render_fact_text($1, sb.value), string_agg(render_name($1, sb.entity_id, 'ko'), '·' ORDER BY sb.entity_id)
		FROM attribute sb WHERE sb.work_id = $1 AND sb.key = 'shown_by' GROUP BY 1`, work)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var t, n string
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		m[t] = n
	}
	return m, rows.Err()
}

// dealLook hands a place's sense words (" · " separated) out two per scene, so the card stops reading as a
// checklist to pour into the first paragraph (P7); words left over are optional. A sentence look is passed as is.
func dealLook(look string, scenes []string) string {
	words := strings.Split(look, " · ")
	if len(words) < 3 {
		return look
	}
	sort.Strings(scenes)
	var parts []string
	i := 0
	for _, s := range scenes {
		if i >= len(words) {
			break
		}
		j := min(i+2, len(words))
		parts = append(parts, s+"에 쓸 재료: "+strings.Join(words[i:j], " · "))
		i = j
	}
	if i < len(words) {
		parts = append(parts, "쓰지 않아도 되는 것: "+strings.Join(words[i:], " · "))
	}
	return strings.Join(parts, " / ")
}

// actedFacts is the set of fact texts some act of the episode carries (told, seen, written…).
func actedFacts(ctx context.Context, pool *pgxpool.Pool, work, from, to string) (map[string]bool, error) {
	rows, err := pool.Query(ctx, epSQL+` SELECT DISTINCT render_fact_text($1, a.prop_id) FROM act a
		WHERE a.work_id = $1 AND a.prop_id IS NOT NULL AND a.scene_id IN (SELECT scene_id FROM ep)`, work, from, to)
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

// renderName is a character's Korean display name.
func renderName(ctx context.Context, pool *pgxpool.Pool, work, id string) string {
	var n string
	pool.QueryRow(ctx, `SELECT render_name($1, $2, 'ko')`, work, id).Scan(&n)
	return n
}
