package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DigestCap is the digest's length cap in characters.
const DigestCap = 8000

const hiddenMark = " [독자 비공개]"

// StateDigest is the story state at the last shown scene of the loaded work, in Korean, for writing the next
// episode: modules and scene order, who is alive, who believes something false, setups,
// and which withheld facts the reader still does not know ([독자 비공개], Review Focus 4). It only reads existing
// tables and SQL functions. Every list is ordered, so the same load gives the same text.
func StateDigest(ctx context.Context, pool *pgxpool.Pool, work string) (string, error) {
	var last, lastShort, title, pt, pov string
	var lastOrd int
	err := pool.QueryRow(ctx, `
		SELECT s.id, lint_short(s.id), s.title, s.discourse_ord, coalesce(s.end_pt, s.start_pt, ''),
		       coalesce((SELECT array_to_string(d."values", ' ') FROM directive d
		                  WHERE d.work_id = s.work_id AND d.scene_id = s.id AND d.key = 'pov' LIMIT 1), '')
		  FROM scene s WHERE s.work_id = $1 AND s.shown ORDER BY s.discourse_ord DESC, s.id DESC LIMIT 1`, work).
		Scan(&last, &lastShort, &title, &lastOrd, &pt, &pov)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("work %q: no shown scene loaded", work)
	}
	if err != nil {
		return "", err
	}
	q := func(sql string, args ...any) ([][]string, error) {
		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out [][]string
		for rows.Next() {
			vs, err := rows.Values()
			if err != nil {
				return nil, err
			}
			r := make([]string, len(vs))
			for i, v := range vs {
				if v != nil {
					r[i] = fmt.Sprint(v)
				}
			}
			out = append(out, r)
		}
		return out, rows.Err()
	}
	var head, tail []string
	add := func(dst *[]string, ls ...string) { *dst = append(*dst, ls...) }

	add(&head, "# 이야기 상태 요약 (작품 ID `"+work+"`)", "",
		fmt.Sprintf("마지막 장면: %s%s(서술 위치 %d, 시점 %s)", lastShort, quoted(lastShort, title), lastOrd, orDash(pov)))

	mods, err := q(`SELECT m.name, array_to_string(m.paths, ', '),
	                       coalesce((SELECT string_agg(CASE WHEN starts_with(i, $1 || '/') THEN substr(i, length($1) + 2) ELSE i END, ', ')
	                                   FROM unnest(m.imports) i), '')
	                  FROM module m WHERE m.work_id = $1 ORDER BY m.name`, work)
	if err != nil {
		return "", err
	}
	add(&head, "", "## 모듈과 파일 (이미 있는 파일은 고치지 않는다)")
	for _, m := range mods {
		add(&head, fmt.Sprintf("- %s: %s (import: %s)", m[0], m[1], orDash(m[2])))
	}
	scenes, err := q(`SELECT lint_short(id) FROM scene WHERE work_id = $1 AND shown ORDER BY discourse_ord, id`, work)
	if err != nil {
		return "", err
	}
	var ids []string
	for _, s := range scenes {
		ids = append(ids, s[0])
	}
	add(&head, "", "## 장면 순서", strings.Join(ids, " "))

	// withheld (base, value) the reader does not yet believe at the last scene: reveal and the until scene both count.
	hid, err := q(`
		SELECT DISTINCT lint_short(z.base), coalesce(lint_short(wh.until_pt), '')
		  FROM withhold wh CROSS JOIN LATERAL prop_norm($1, wh.prop_id) z
		 WHERE wh.work_id = $1 AND wh.prop_id IS NOT NULL
		   AND NOT EXISTS (SELECT 1 FROM reader_state($1, $2) r WHERE r.fact_id = z.base AND r.value = z.val)
		 ORDER BY 1, 2`, work, lastOrd)
	if err != nil {
		return "", err
	}
	hidden := map[string]bool{}
	add(&head, "", "## 독자에게 아직 숨긴 사실 (인물이 알아도 독자는 모른다 — 서술로 밝히지 않는다)")
	for _, h := range hid {
		hidden[h[0]] = true
		add(&head, fmt.Sprintf("- %s%s (until %s)", h[0], hiddenMark, orDash(strings.TrimSuffix(h[1], ".start"))))
	}
	if len(hid) == 0 {
		add(&head, "- 없음")
	}
	mark := func(id string) string {
		if hidden[id] {
			return hiddenMark
		}
		return ""
	}

	chars, err := q(`
		SELECT lint_short(e.id), render_name($1, e.id, 'ko'),
		       CASE WHEN p.id IS NULL THEN '생존'
		            WHEN $2 <> '' AND precedes($1, p.id, $2) = 'yes' THEN '사망'
		            ELSE '미정(사망 시점이 마지막 장면 뒤이거나 정해지지 않음)' END
		  FROM entity e LEFT JOIN point p ON p.work_id = $1 AND p.id = e.id || '.died' AND p.kind = 'died'
		 WHERE e.work_id = $1 AND e.kind = 'character' ORDER BY e.id`, work, pt)
	if err != nil {
		return "", err
	}
	add(&head, "", "## 인물 (마지막 장면 기준)")
	for _, c := range chars {
		add(&head, fmt.Sprintf("- %s %s: %s", c[0], c[1], c[2]))
	}

	// a false belief still held at the last scene's point: char_knows says yes, truth_at says the opposite.
	// The liar of that proposition is not deceived by it (as metric_active_deceptions).
	bel, err := q(`
		WITH b AS (
		  SELECT DISTINCT bl.entity_id, z.base, (z.val = bl.value) bv
		    FROM believes bl JOIN entity e ON e.work_id = $1 AND e.id = bl.entity_id AND e.kind = 'character'
		    CROSS JOIN LATERAL prop_norm($1, bl.prop_id) z
		   WHERE bl.work_id = $1)
		SELECT lint_short(b.entity_id), lint_short(b.base),
		       coalesce((SELECT CASE WHEN f.def ? 'pred' THEN lint_short(f.id) ELSE f.text END FROM fact f
		                  WHERE f.work_id = $1 AND f.id = b.base), ''),
		       CASE WHEN b.bv THEN '참' ELSE '거짓' END,
		       coalesce((SELECT string_agg(DISTINCT lint_short(d.liar_id), ', ' ORDER BY lint_short(d.liar_id))
		                   FROM deception d JOIN believes x ON x.work_id = $1 AND x.cause_act = d.act_id AND x.entity_id = b.entity_id
		                  WHERE d.work_id = $1 AND (SELECT base FROM prop_norm($1, d.prop_id)) = b.base), '')
		  FROM b
		 WHERE $2 <> '' AND truth_at($1, b.base, $2) = NOT b.bv AND char_knows($1, b.entity_id, b.base, b.bv, $2) = 'yes'
		   AND NOT EXISTS (SELECT 1 FROM deception d WHERE d.work_id = $1 AND d.liar_id = b.entity_id
		                      AND (SELECT base FROM prop_norm($1, d.prop_id)) = b.base)
		 ORDER BY 1, 2`, work, pt)
	if err != nil {
		return "", err
	}
	add(&head, "", "## 거짓 믿음 (마지막 장면에서 인물이 사실과 반대로 믿는 것)")
	for _, b := range bel {
		l := fmt.Sprintf("- %s: %s%s 가 %s이라고 믿는다(실제는 반대)", b[0], b[1], quoted(b[1], b[2]), b[3])
		if b[4] != "" {
			l += " ← " + b[4] + " 의 기만"
		}
		add(&head, l+mark(b[1]))
	}
	if len(bel) == 0 {
		add(&head, "- 없음")
	}

	var open string
	if err := pool.QueryRow(ctx, `SELECT n::text FROM metric_open_setups($1) ORDER BY ord DESC LIMIT 1`, work).Scan(&open); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	sus, err := q(`
		SELECT substr(s.id, length($1) + 2), coalesce(nullif(s.text, ''), (SELECT f.text FROM fact f WHERE f.work_id = $1 AND f.id = s.prop_id), ''),
		       coalesce(lint_short(s.scene_id), ''),
		       coalesce((SELECT lint_short(p.scene_id) FROM payoff p LEFT JOIN scene x ON x.id = p.scene_id
		                  WHERE p.work_id = $1 AND p.setup_id = s.id ORDER BY x.discourse_ord NULLS LAST, p.scene_id LIMIT 1), ''),
		       (SELECT count(*) FROM payoff p WHERE p.work_id = $1 AND p.setup_id = s.id)
		  FROM setup s WHERE s.work_id = $1
		 ORDER BY (SELECT min(x.discourse_ord) FROM payoff p JOIN scene x ON x.id = p.scene_id
		            WHERE p.work_id = $1 AND p.setup_id = s.id) DESC NULLS FIRST, s.src_id, s.id`, work)
	if err != nil {
		return "", err
	}
	add(&head, "", "## 열린 복선 (`payoff <모듈>/<id>` 로 회수; 마지막 위치의 열린 수: "+orDash(open)+")")
	var resolved []string
	for _, s := range sus {
		if s[4] == "0" {
			add(&head, fmt.Sprintf("- %s「%s」(심은 장면 %s) — 열림", s[0], s[1], orDash(s[2])))
		} else {
			resolved = append(resolved, fmt.Sprintf("- %s「%s」(심은 장면 %s) — 회수됨: %s", s[0], s[1], orDash(s[2]), orDash(s[3])))
		}
	}

	// Facts and their truth are in the cast files the prompt carries, so the digest does not repeat them.
	// Resolved setups are the tail, newest payoff first (the query order), so trimming drops the oldest first.
	add(&tail, "", "## 회수된 복선")
	add(&tail, resolved...)

	return capDigest(head, tail), nil
}

// capDigest joins head and tail within DigestCap characters.
// ponytail: over the cap, tail lines (resolved setups) go from the end, oldest first;
// head (hidden facts, deaths, false beliefs, open setups) is never trimmed except by a final hard cut. If big serials
// hit the hard cut, rank head lines by recency too.
func capDigest(head, tail []string) string {
	const more = "\n… (상한 초과로 생략)\n"
	n := func(ls []string) int { return utf8.RuneCountInString(strings.Join(ls, "\n") + "\n") }
	cut := false
	for len(tail) > 0 && n(head)+n(tail) > DigestCap {
		tail, cut = tail[:len(tail)-1], true
	}
	s := strings.Join(head, "\n") + "\n"
	if len(tail) > 0 {
		s += strings.Join(tail, "\n") + "\n"
	}
	if !cut && utf8.RuneCountInString(s) <= DigestCap {
		return s
	}
	r := []rune(s)
	if max := DigestCap - utf8.RuneCountInString(more); len(r) > max {
		r = r[:max]
	}
	return string(r) + more
}

// quoted is 「text」, or nothing when text is empty or just repeats id.
func quoted(id, text string) string {
	if text == "" || text == id {
		return ""
	}
	return "「" + text + "」"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
