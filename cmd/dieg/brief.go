package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// briefText compiles the writer's brief for one episode: what the reader already holds before `from` (do not
// re-explain), what this episode must hand over (facts first known by `to`, exclusive; omitted when to == ""), and
// what stays hidden (facts true in the work the reader still lacks afterwards). The design code, if given, follows.
// The brief is for an episode writer, not a renderer: it fixes knowledge, never sentences.
func briefText(ctx context.Context, pool *pgxpool.Pool, work string, o briefOpts) (string, error) {
	from, to, since, code, style, rulesDir := o.from, o.to, o.since, o.code, o.style, o.rules
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
		b.WriteString(strings.TrimSpace(style) + "\n\n") // the style file carries its own heading
	}
	b.WriteString("# 회차 설계서\n\n")
	// ⓪ now and the world's rules (S14, S15). Story time comes from the first scene's start point (seconds since start).
	// S37: whether the previous told scene's day is certainly earlier decides "yesterday"; the header says so only
	// when it is (prevHi < lo), and warns when the two may share a day.
	var lo, hi, prevHi int64 = 0, 0, -1
	if err := pool.QueryRow(ctx, `WITH o AS (SELECT * FROM scene_told_ord($1)), me AS (SELECT o.ord FROM o JOIN scene x ON x.id = o.scene_id
		  WHERE x.id = $2 OR lint_short(x.id) = $2)
		SELECT coalesce(p.earliest, 0), coalesce(p.latest, p.earliest, 0),
		       coalesce((SELECT coalesce(pe.latest, pe.earliest) FROM o JOIN scene y ON y.id = o.scene_id JOIN point pe ON pe.id = y.end_pt
		                  WHERE o.ord < (SELECT * FROM me) ORDER BY o.ord DESC LIMIT 1), -1)
		FROM scene s JOIN point p ON p.id = s.start_pt WHERE s.work_id = $1 AND (s.id = $2 OR lint_short(s.id) = $2)`, work, from).Scan(&lo, &hi, &prevHi); err == nil {
		d1, d2 := lo/86400+1, hi/86400+1
		day := fmt.Sprintf("%d일째", d1)
		if d2 > d1 {
			day = fmt.Sprintf("%d~%d일째", d1, d2)
		}
		switch {
		case prevHi < 0:
			fmt.Fprintf(&b, "## ⓪ 지금 — 이야기 시작 후 %s\n", day)
		case prevHi/86400+1 < d1:
			fmt.Fprintf(&b, "## ⓪ 지금 — 이야기 시작 후 %s. 앞 화의 '오늘'은 이미 지난 날이다\n", day)
		default:
			fmt.Fprintf(&b, "## ⓪ 지금 — 이야기 시작 후 %s. 앞 화와 같은 날일 수 있다 — 앞 화의 일을 '어제'로 쓰지 않는다\n", day)
		}
	}
	// Rules are `attr <term>.rule = "..."`; only rules of terms the reader meets by the episode end are given.
	rules, err := pool.Query(ctx, `SELECT render_name($1, a.entity_id, 'ko'), a.value FROM attribute a
		WHERE a.work_id = $1 AND a.key = 'rule' AND (NOT EXISTS (SELECT 1 FROM term_explained_at($1) t WHERE t.term_id = a.entity_id)
		  OR EXISTS (SELECT 1 FROM term_explained_at($1) t WHERE t.term_id = a.entity_id AND t.ord < $2))
		ORDER BY a.entity_id, a.id`, work, toldOrd(ctx, pool, work, to))
	if err != nil {
		return "", err
	}
	var rb strings.Builder
	for rules.Next() {
		var n, v string
		if err := rules.Scan(&n, &v); err != nil {
			rules.Close()
			return "", err
		}
		fmt.Fprintf(&rb, "- %s: %s\n", n, v)
	}
	rules.Close()
	if rb.Len() > 0 {
		b.WriteString("\n## ⓪ 세계 규칙 — 어기지 않는다. 설명문으로 옮기지 않고 인물이 규칙에 맞게 움직이게 한다\n" + rb.String() + "\n")
	}
	if now, err := nowSection(ctx, pool, work, from, to, rulesDir, before.Text() == ""); err != nil {
		return "", err
	} else if now != "" {
		b.WriteString("\n## ⓪ 시계와 카운터 — 숫자는 정확히 이 값으로 쓴다\n" + now + "\n")
	}
	forged, err := forgedFacts(ctx, pool, work)
	if err != nil {
		return "", err
	}
	b.WriteString("## ① 독자가 이미 아는 것 — 다시 설명하지 않는다. 괄호는 독자가 그것을 안 날(S18)\n")
	if t := before.Text(); t != "" {
		b.WriteString(labelForgedLines(forged, dateFacts(ctx, pool, work, t)))
	} else {
		b.WriteString("(없음: 첫 화다)\n")
	}
	// Character cards (S13): who is on stage, from `attr <character>.role/want/look/voice/creed`. A creed is said
	// openly, so it is never listed as a hidden truth below (S19). want/look make the person visible (user 11/14:
	// "인물이 그려지지 않음").
	pov := povOf(ctx, pool, work, from)
	openly := map[string]bool{}
	var ids []string
	cards, err := pool.Query(ctx, epSQL+`
		SELECT c.entity_id, render_name($1, c.entity_id, 'ko'), coalesce(string_agg(CASE a.key WHEN 'role' THEN '역할' WHEN 'want' THEN '원하는 것'
		           WHEN 'look' THEN CASE WHEN c.entity_id = $4 THEN '겉모습(남이 보는 모습 — 1인칭 화자는 자기를 못 본다. 남의 눈·말·반사면으로만 보인다)' ELSE '겉모습' END WHEN 'target' THEN '겨누는 대상(신조·원하는 것은 이 대상에만 쓴다)' WHEN 'voice' THEN '말투' WHEN 'unaware' THEN '모르는 것(아는 척하지 않는다)' ELSE '신조(이 문장을 그대로 읊지 않는다. 자기 말로 한 번 비치거나 행동의 이유로)' END || ': ' || a.value, ' / '
		         ORDER BY array_position(ARRAY['role', 'want', 'target', 'look', 'voice', 'unaware', 'creed'], a.key)), ''),
		       coalesce(string_agg(a.value, '|') FILTER (WHERE a.key = 'creed'), '')
		  FROM cast_ c LEFT JOIN attribute a ON a.work_id = $1 AND a.entity_id = c.entity_id AND a.key IN ('role', 'want', 'target', 'look', 'voice', 'unaware', 'creed')
		 GROUP BY c.entity_id ORDER BY 1`, work, from, to, pov)
	if err != nil {
		return "", err
	}
	b.WriteString("\n## ① 이 화의 인물 — 카드는 재료다. 문구를 그대로 옮기지 않고 한두 가지를 골라 자기 말로 쓴다. 처음 나올 때 몸으로 세우고, 원하는 것은 행동으로 보인다. 시점 밖 인물은 적힌 경로대로만 안다\n")
	var lines []string
	for cards.Next() {
		var id, n, card, cr string
		if err := cards.Scan(&id, &n, &card, &cr); err != nil {
			cards.Close()
			return "", err
		}
		for _, c := range strings.Split(cr, "|") {
			openly[c] = c != ""
		}
		l := "- " + n + "\n"
		if card != "" {
			l = fmt.Sprintf("- %s — %s\n", n, card)
		}
		ids, lines = append(ids, id), append(lines, l)
	}
	cards.Close()
	// S34/S38: each off-POV character's card is followed by what they know of this episode's matter and how.
	for i, id := range ids {
		b.WriteString(lines[i])
		if id == pov {
			// P19: the narrator's first episode needs one moment where another's eyes or a surface shows them.
			if !slices.Contains(before.Cast, renderName(ctx, pool, work, pov)) {
				b.WriteString("  · 화자가 처음 나오는 화다: 화자를 비출 눈이나 반사면을 한 번 둔다(다른 인물이 화자를 보는 순간, 유리·화면). 겉모습 칸은 그 순간에 쓴다\n")
			}
			continue
		}
		k, err := knowsLines(ctx, pool, work, from, to, id)
		if err != nil {
			return "", err
		}
		b.WriteString(k)
	}
	places, err := placeCards(ctx, pool, work, from, to)
	if err != nil {
		return "", err
	}
	if places != "" {
		b.WriteString("\n## ① 이 화의 장소 — 재료다. 한두 가지를 인물의 행동에 붙여 장면 속에 흩는다. 다시 찾은 장소는 바뀐 것 하나만 쓴다\n" + places)
	}
	lived, err := povLived(ctx, pool, work, from, pov)
	if err != nil {
		return "", err
	}
	if len(lived) > 0 {
		b.WriteString("\n## ① 화자가 직접 겪어 아는 것 — 독자도 화자와 함께 봤다. 숨길 것이 아니다\n")
		for _, t := range lived {
			openly[t] = true
			// S30: "it is not true that <forged record>" is already said by the forged-record label in ① and ②.
			if known[t] || forged[strings.TrimPrefix(t, "It is not true that ")] {
				continue
			}
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	// Spoken lines since `since` (S18): what the reader heard said, word for word, so names and threats carry over.
	if since != "" {
		// Who heard it matters (S21): a line the POV character was not there for is the reader's alone.
		lines, err := spokenLines(ctx, pool, work, toldOrd(ctx, pool, work, since), toldOrd(ctx, pool, work, from), pov)
		if err != nil {
			return "", err
		}
		b.WriteString("\n## ① 앞에서 독자가 들은 말 — 별칭·약속·위협은 이 말 그대로 이어진다. 시점 인물이 없던 자리의 말은 시점 인물이 모른다\n" + lines)
	}
	if to != "" {
		// S28: the terms this episode establishes are the world's premise for a first-time reader; give their gloss
		// (as reference) and how the code says they should come across.
		terms, err := newTerms(ctx, pool, work, from, to, before)
		if err != nil {
			return "", err
		}
		if terms != "" {
			b.WriteString("\n## ② 이 화가 세울 세계 — 처음 읽는 독자가 이 화를 덮을 때 알아야 하는 전제. 풀이는 참고용이고 옮기지 않는다. 괄호의 방식으로 보인다\n" + terms)
		}
		// S5: a fact that a term's mechanism carries (`attr <term>.shown_by = <fact>`) says so on its own line, so the
		// writer stages the device instead of summarising the fact.
		via, err := factMechanisms(ctx, pool, work)
		if err != nil {
			return "", err
		}
		acted, err := actedFacts(ctx, pool, work, from, to)
		if err != nil {
			return "", err
		}
		b.WriteString("\n## ② 이 화가 독자에게 넘길 것 — 장면으로 짐작하게 한다. 문장을 그대로 옮기지 않는다\n")
		for _, f := range after.Facts {
			if !had[f.Text] {
				l := "- " + forgedLabel(forged, f.Text)
				if m := via[f.Text]; m != "" {
					l += " — " + m + "의 작동으로 일어난다: 그 장치가 장면에서 움직이게 보인다"
				} else if !acted[f.Text] && !forged[f.Text] { // S5: a bare background fact, carried by no act of this episode
					l += " — 배경: 족보 읽듯 요약하지 않는다. 이 화의 사건·말에 비친 한 조각으로만"
				}
				b.WriteString(l + "\n")
			}
		}
	}
	// Only truths this episode touches: believed by someone on stage, or carried by an act in its scenes. Far-future
	// truths are not given to the writer at all; a writer who does not know cannot leak (S11).
	// A belief counts only if its point certainly precedes the episode end; an unordered point is dropped, not passed.
	b.WriteString("\n## ③ 밝히지 않는다 — 이 화 인물이 쥐고 있지만 독자는 아직 모르는 진실. 인물은 이것을 알고 숨기며 움직인다\n")
	rows, err := pool.Query(ctx, epSQL+`,
		endpt AS (SELECT s.end_pt FROM scene s JOIN o ON o.scene_id = s.id WHERE s.id IN (SELECT scene_id FROM ep) ORDER BY o.ord DESC LIMIT 1),
		bl_ AS (SELECT bl.* FROM believes bl WHERE bl.work_id = $1 AND bl.entity_id IN (SELECT * FROM cast_)
		        AND (bl.from_pt IS NULL OR bl.from_pt = (SELECT * FROM endpt) OR precedes($1, bl.from_pt, (SELECT * FROM endpt)) = 'yes'))
		SELECT render_fact_text($1, f.id), coalesce((SELECT string_agg(DISTINCT render_name($1, bl.entity_id, 'ko'), ', ')
		         FROM bl_ bl WHERE bl.prop_id = f.id AND bl.value), '')
		  FROM fact f WHERE f.work_id = $1 AND render_fact_true($1, f.id)
		   AND (EXISTS (SELECT 1 FROM bl_ bl WHERE bl.prop_id = f.id)
		     OR EXISTS (SELECT 1 FROM act a WHERE a.work_id = $1 AND a.prop_id = f.id AND a.scene_id IN (SELECT scene_id FROM ep)))
		   -- S24: a negation of something the reader already holds (a forged record they watched being made) is not hidden.
		   AND NOT EXISTS (SELECT 1 FROM reader_knows($1) k JOIN scene sk ON sk.work_id = $1 AND sk.discourse_ord = k.from_ord
		         JOIN o ok ON ok.scene_id = sk.id WHERE k.fact_id IN (f.id, f.def->>'not') AND ok.ord < $4)
		 ORDER BY f.id`, work, from, to, toldOrd(ctx, pool, work, to))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var t, who string
		if err := rows.Scan(&t, &who); err != nil {
			return "", err
		}
		if known[t] || openly[t] || ruleSays(rb.String(), t) {
			continue
		}
		if who != "" && pov != "" && !strings.Contains(who, renderName(ctx, pool, work, pov)) {
			// S26: the narrator's boundary next to a secret they sit beside: not even a hint in their thoughts.
			fmt.Fprintf(&b, "- %s — 아는 인물: %s. 화자는 모른다 — 화자의 생각·기억·추측에도 비치지 않는다(화자는 ①만큼만 안다)\n", t, who)
		} else if who != "" {
			fmt.Fprintf(&b, "- %s — 아는 인물: %s\n", t, who)
		} else {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	b.WriteString("\n## ④ 쓰기 규칙\n- 회차 번호·'지난 화' 같은 연재 바깥 말을 원고에 쓰지 않는다.\n- 설계서의 사실 문장·용어 풀이를 원고에 옮겨 적지 않는다. 인물의 행동·대사·결과로 보인다.\n- ①에 없는 것은 독자가 모른다. 시점 인물은 ①과 ③에서 자기 이름이 붙은 것만 더 안다. 그것을 서술로 털어놓지 않고 행동의 이유로만 비친다.\n- 모르는 것은 쓰지 않거나 행동으로 남긴다. 화자가 추측을 풀어 쓰거나 '추측이었다'고 꼬리표를 달지 않는다.\n- 마지막 단락은 이 화가 남긴 위협이나 질문을 사물 하나·행동 하나에 실어 닫는다(다시 찾은 장소의 바뀐 것이 좋은 후보). 예측문('…있을 것이다')이나 수사 의문으로 닫지 않는다. 잠·휴식으로 가라앉히지 않는다.\n- 전환의 앞 상태(기대·안도 등)를 세울 근거는 ①에서만 찾는다. ③에서 근거를 끌어오지 않는다.\n- 설명하지 않는 것과 그리지 않는 것은 다르다. 규칙과 사연은 다시 풀지 않지만, 장소와 인물은 매 장면 눈앞에 보이게 그린다. 무대 위 단역 한두 명의 짧은 말로 이 세계의 셈(값·빚·두려움)을 들려준다.\n")
	beats, err := sceneBeats(ctx, pool, work, from, to)
	if err != nil {
		return "", err
	}
	if beats != "" {
		b.WriteString("\n## ⑤ 장면 비트 — 장면마다 목표·전환·장면 시계\n" + beats)
	}
	// S22: who each line is addressed to is often the scene's point; compile it instead of leaving it in raw code.
	aud, err := spokenLines(ctx, pool, work, toldOrd(ctx, pool, work, from), toldOrd(ctx, pool, work, to), pov)
	if err != nil {
		return "", err
	}
	if aud != "" {
		b.WriteString("\n## ⑤ 이 화의 말과 청중 — 누가 누구에게 말하는지가 장면의 일이다\n" + aud)
	}
	if code != "" {
		b.WriteString("\n## ⑤ 장면 설계 (디제시스 코드) — 장면 순서·장소·행위·등장/퇴장·누가 무엇을 아는지를 따른다. 감각·몸짓·세부는 자유\n")
		b.WriteString(stripCodeComments(code))
	}
	// ponytail: the SQL fact renderer spells negation in English; rewrite here until render_fact_text takes a language.
	s := strings.ReplaceAll(b.String(), "It is not true that ", "(거짓) ")
	return maskNames(ctx, pool, work, from, toldOrd(ctx, pool, work, to), after.Text(), s)
}

type briefOpts struct{ from, to, since, code, style, rules string }

// epSQL opens a WITH clause: o (telling order), ep (scenes from $2 up to, not including, $3; to the end when $3 = ”),
// cast_ (characters present in ep). $1 is the work.
const epSQL = `WITH o AS (SELECT * FROM scene_told_ord($1)),
	ep AS (SELECT o.scene_id FROM o WHERE o.ord >= (SELECT o2.ord FROM o o2 JOIN scene x ON x.id = o2.scene_id
	         WHERE x.id = $2 OR lint_short(x.id) = $2)
	       AND ($3 = '' OR o.ord < (SELECT o3.ord FROM o o3 JOIN scene y ON y.id = o3.scene_id WHERE y.id = $3 OR lint_short(y.id) = $3))),
	cast_ AS (SELECT DISTINCT p.entity_id FROM presence p JOIN entity e ON e.id = p.entity_id
	          WHERE p.work_id = $1 AND e.kind = 'character' AND p.scene_id IN (SELECT scene_id FROM ep))`

// ruleSays reports whether the world-rule lines already state fact t (its clause before " — "), so it is not hidden twice.
func ruleSays(rules, t string) bool {
	head, _, _ := strings.Cut(t, " — ")
	return len(head) > 10 && strings.Contains(rules, head)
}

// stripCodeComments drops `#` comment lines inside ```sl blocks: they are the author's notes, often the very
// explanation the scene should leave to the reader (P3). Markdown headings outside the blocks stay.
func stripCodeComments(md string) string {
	var out []string
	in := false
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") {
			in = !in
		} else if in && strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// cmdBrief runs `dieg brief <work|dir> --from <scene> [--to <scene>] [--since <scene>] [--code <file>] [--style <file>] [--rules <dir>]`.
func cmdBrief(args []string) int {
	var target, from, to, since, codeFile, styleFile, rulesDir string
	flags := map[string]*string{"--from": &from, "--to": &to, "--since": &since, "--code": &codeFile, "--style": &styleFile, "--rules": &rulesDir}
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
	s, err := briefText(ctx, pool, work, briefOpts{from, to, since, code, style, rulesDir})
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

// toldOrd is the telling position of scene id, or a position past every scene when id is "" or unknown.
func toldOrd(ctx context.Context, pool *pgxpool.Pool, work, id string) int {
	o := 1 << 30
	if id != "" {
		pool.QueryRow(ctx, `SELECT o.ord FROM scene_told_ord($1) o JOIN scene x ON x.id = o.scene_id
			WHERE x.id = $2 OR lint_short(x.id) = $2`, work, id).Scan(&o)
	}
	return o
}
