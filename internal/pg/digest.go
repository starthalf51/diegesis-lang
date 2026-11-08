package pg

// Digest: 기존 표·함수만 읽어(SELECT 전용, 마이그레이션 없음) 한 장면 시점의 세계 상태 요약(한국어)을 만든다.
// tools/statespike 에서 옮겼다. "독자가 이미 아는 것" 절은 뺐다(스파이크: 효과 없이 길이만 늘림).

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"diegesis/internal/render"
)

const digestCap = 3600

type rowsFn func(sql string, args ...any) ([][]string, error)

func querier(ctx context.Context, p *pgxpool.Pool) rowsFn {
	return func(sql string, args ...any) ([][]string, error) {
		rs, err := p.Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		var out [][]string
		for rs.Next() {
			vs, err := rs.Values()
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
		return out, rs.Err()
	}
}

func hm(sec int64) string {
	d, h, m := sec/86400, sec%86400/3600, sec%3600/60
	switch {
	case d > 0:
		return fmt.Sprintf("%d일 %d시간", d, h)
	case h > 0:
		return fmt.Sprintf("%d시간 %d분", h, m)
	}
	return fmt.Sprintf("%d분", m)
}

// Digest 는 장면 sceneID(짧은 ID)의 시점에서 PG 가 아는 세계 상태. 절 제목부터 끝까지의 텍스트이고, 장면이 없으면 오류다.
func Digest(ctx context.Context, p *pgxpool.Pool, work string, sceneID string) (text, state string, err error) {
	rb, err := RenderScene(ctx, p, work, sceneID)
	if err != nil {
		return "", "", err
	}
	s, err := render.ParseScene(rb)
	if err != nil {
		return "", "", fmt.Errorf("scene %s: %w", sceneID, err)
	}
	q := querier(ctx, p)
	var sid, startPt, placeShort, placeName string
	var ord int
	rows, err := q(`SELECT id, discourse_ord, coalesce(start_pt,''), coalesce(lint_short(place_id),''), coalesce(render_name($1, place_id, 'ko'),'')
	                  FROM scene WHERE work_id=$1 AND lint_short(id)=$2 AND shown`, work, s.ID)
	if err != nil || len(rows) != 1 {
		return "", "", fmt.Errorf("scene %s: %v (%d rows)", s.ID, err, len(rows))
	}
	sid, startPt, placeShort, placeName = rows[0][0], rows[0][2], rows[0][3], rows[0][4]
	fmt.Sscan(rows[0][1], &ord)

	// 서술하면 안 되는 사실: 이 장면의 Withheld + 독자가 아직 모르는 withhold 대상.
	hidden := map[string]bool{}
	for _, w := range s.Withheld {
		hidden[w] = true
	}
	hid, err := q(`SELECT DISTINCT lint_short(z.base) FROM withhold wh CROSS JOIN LATERAL prop_norm($1, wh.prop_id) z
	                WHERE wh.work_id=$1 AND wh.prop_id IS NOT NULL
	                  AND NOT EXISTS (SELECT 1 FROM reader_state($1, $2) r WHERE r.fact_id = z.base)`, work, ord-1)
	if err != nil {
		return "", "", err
	}
	for _, h := range hid {
		hidden[h[0]] = true
	}

	var b strings.Builder
	// st collects only the STATE sections (1, 2, 4) the staleness hash covers; sections 3 and 5 are narrative.
	var st strings.Builder
	inState := true
	line := func(f string, a ...any) {
		fmt.Fprintf(&b, f+"\n", a...)
		if inState {
			fmt.Fprintf(&st, f+"\n", a...)
		}
	}
	line("## 이 시점의 세계 상태 (어기지 말 것)")
	line("아래는 이 이야기의 코드·그래프에서 뽑은 확정 상태다. 위 장면 명세와 겹치지 않는 한, 소품·장소·앎을 아래와 다르게 쓰지 않는다. 아래에 없는 소품 위치를 새로 옮기지 않는다.")

	// 1. 장소·시간
	tm, err := q(`SELECT coalesce((SELECT earliest FROM point WHERE id=$2),-1),
	                     coalesce((SELECT p.earliest FROM scene x JOIN point p ON p.id=x.start_pt WHERE x.work_id=$1 AND x.shown AND x.discourse_ord=$3-1),-1),
	                     coalesce((SELECT lint_short(x.id) FROM scene x WHERE x.work_id=$1 AND x.shown AND x.discourse_ord=$3-1),'')`, work, startPt, ord)
	if err != nil {
		return "", "", err
	}
	var t0, tp int64
	fmt.Sscan(tm[0][0], &t0)
	fmt.Sscan(tm[0][1], &tp)
	line("\n### 장소와 시간")
	line("- 이 장면의 장소: %s(%s)", placeName, placeShort)
	if t0 >= 0 {
		line("- 이야기가 시작된 뒤 최소 %s 지났다(1화 첫 장면 기준).", hm(t0))
		if tp >= 0 && tm[0][2] != "" {
			line("- 서술상 바로 앞 장면(%s)이 시작된 때부터 최소 %s 지났다.", tm[0][2], hm(t0-tp))
		}
	}

	// 2. 코드에 있는 속성: 사물·장소, 그리고 이미 떠난 인물
	at, err := q(`SELECT lint_short(a.entity_id), render_name($1, a.entity_id,'ko'), e.kind, a.key, a.value, a.kind,
	                     CASE WHEN a.kind='id' AND EXISTS (SELECT 1 FROM entity v WHERE v.id=a.value) THEN render_name($1, a.value,'ko') ELSE a.value END,
	                     count(*) OVER (PARTITION BY a.entity_id, a.key), coalesce(lint_short(a.at_pt),''),
	                     CASE WHEN a.at_pt IS NULL THEN 'yes' ELSE precedes($1, a.at_pt, $2) END
	                FROM attribute a JOIN entity e ON e.work_id=$1 AND e.id=a.entity_id
	               WHERE a.work_id=$1 AND (e.kind <> 'character' OR a.key='status')
	                 AND (e.kind <> 'thing' OR EXISTS (SELECT 1 FROM act x JOIN scene xs ON xs.id=x.scene_id WHERE x.work_id=$1 AND xs.discourse_ord <= $3 AND a.entity_id = ANY(x.objects)))
	               ORDER BY 1, 4`, work, startPt, ord)
	if err != nil {
		return "", "", err
	}
	line("\n### 코드에 적힌 속성")
	var elsewhere []string
	for _, r := range at {
		if r[2] == "character" {
			if r[9] == "yes" && r[4] == "gone" {
				line("- %s: 이미 이야기에서 떠났다(%s)", r[1], r[3])
			}
			continue
		}
		note := ""
		if r[7] == "1" && r[3] == "at" {
			note = " (코드에 옮긴 기록 없음)"
		}
		line("- %s(%s): %s = %s%s", r[1], r[0], r[3], r[6], note)
		if r[3] == "at" && r[5] == "id" && r[6] != placeName {
			elsewhere = append(elsewhere, fmt.Sprintf("%s(%s)", r[1], r[6]))
		}
	}
	if len(elsewhere) > 0 {
		line("- 이 장면의 장소(%s)에 없는 물건: %s. 장면에서 직접 옮기는 행위가 없으면 그 자리에 둔다.", placeName, strings.Join(elsewhere, ", "))
	}

	inState = false
	// 3. 이미 일어난 물건·동작 행위 (이 장면 이전, 서술 순서)
	ac, err := q(`
	  WITH last_here AS (SELECT id FROM scene WHERE work_id=$1 AND shown AND discourse_ord < $2 AND place_id = (SELECT place_id FROM scene WHERE id=$3)
	                      ORDER BY discourse_ord DESC LIMIT 2)
	  SELECT * FROM (
	    SELECT s.discourse_ord, a.ord, lint_short(s.id), render_name($1, a.actor_id,'ko'), a.verb, a.action,
	           coalesce((SELECT string_agg(render_name($1, o,'ko'), ', ') FROM unnest(a.objects) o), ''),
	           coalesce((SELECT string_agg(lint_short(o), ',') FROM unnest(a.objects) o JOIN entity e ON e.id=o AND e.kind='thing'), '')
	      FROM act a JOIN scene s ON s.id=a.scene_id
	     WHERE a.work_id=$1 AND s.shown AND s.discourse_ord < $2
	       AND (EXISTS (SELECT 1 FROM unnest(a.objects) o JOIN entity e ON e.id=o AND e.kind='thing')
	            OR (a.verb='does' AND s.id IN (SELECT id FROM last_here)))
	     ORDER BY s.discourse_ord DESC, a.ord DESC LIMIT 14) x ORDER BY 1, 2`, work, ord, sid)
	if err != nil {
		return "", "", err
	}
	if len(ac) > 0 {
		line("\n### 이미 일어난 물건·동작 (코드의 행위, 오래된 순)")
		for _, r := range ac {
			what := r[4]
			if r[5] != "" {
				what += " " + r[5]
			}
			if r[6] != "" {
				what += " → " + r[6]
			}
			line("- %s, %s: %s", r[2], r[3], what)
		}
	}

	inState = true
	// 4. 등장인물이 아는 것·믿는 것 (이 장면 이전에 얻은 것, 최근 순)
	var cast []string
	for _, e := range s.Cast {
		cast = append(cast, e.ID)
	}
	line("\n### 이 장면 인물이 이미 아는 것·믿는 것 (최근에 얻은 순)")
	for _, id := range cast {
		if k, _ := q(`SELECT 1 FROM entity WHERE work_id=$1 AND lint_short(id)=$2 AND kind='character'`, work, id); len(k) == 0 {
			continue // 사물은 알지 못한다
		}
		bl, err := q(`
		  SELECT * FROM (
		    SELECT DISTINCT ON (z.base) lint_short(z.base), render_fact_text($1, z.base), (z.val = bl.value), point_ord($1, bl.from_pt) o,
		           CASE WHEN truth_at($1, z.base, $4) = (z.val = bl.value) THEN '' ELSE 'x' END
		      FROM believes bl CROSS JOIN LATERAL prop_norm($1, bl.prop_id) z
		     WHERE bl.work_id=$1 AND lint_short(bl.entity_id)=$2 AND point_ord($1, bl.from_pt) < $3
		     ORDER BY z.base, point_ord($1, bl.from_pt) DESC) y
		   WHERE y.o IS NOT NULL ORDER BY y.o DESC LIMIT 5`, work, id, ord, startPt)
		if err != nil {
			return "", "", err
		}
		nm := id
		if len(s.Cast) > 0 {
			for _, e := range s.Cast {
				if e.ID == id && len(e.Names["ko"]) > 0 {
					nm = e.Names["ko"][0]
				}
			}
		}
		n := 0
		for _, r := range bl {
			if hidden[r[0]] {
				continue
			}
			v := "참이라고 안다/믿는다"
			if r[2] == "false" {
				v = "거짓이라고 안다/믿는다"
			}
			if r[4] == "x" {
				v += " (사실과 다른 믿음)"
			}
			line("- %s: 「%s」 — %s [s%s 이후 얻음]", nm, r[1], v, r[3])
			n++
		}
		if n == 0 {
			line("- %s: (기록된 앎 없음)", nm)
		}
	}

	inState = false
	// 5. 이 장면이 심거나 회수하는 복선의 원문
	su, err := q(`SELECT 'plant', coalesce(nullif(s.text,''), render_fact_text($1, s.prop_id), ''), coalesce(lint_short(s.prop_id),''), coalesce((SELECT lint_short(z.base) FROM prop_norm($1, s.prop_id) z LIMIT 1),'') FROM setup s WHERE s.work_id=$1 AND s.scene_id=$2
	               UNION ALL
	              SELECT 'payoff', coalesce(nullif(s.text,''), render_fact_text($1, s.prop_id), ''), coalesce(lint_short(s.prop_id),''), coalesce((SELECT lint_short(z.base) FROM prop_norm($1, s.prop_id) z LIMIT 1),'') FROM payoff p JOIN setup s ON s.id=p.setup_id WHERE p.work_id=$1 AND p.scene_id=$2`, work, sid)
	if err != nil {
		return "", "", err
	}
	var shown [][]string
	for _, r := range su {
		if !hidden[r[2]] && !hidden[r[3]] {
			shown = append(shown, r)
		}
	}
	if len(shown) > 0 {
		line("\n### 이 장면이 심거나 거두는 복선의 원문 (이 표현과 어긋나지 않게)")
		for _, r := range shown {
			k := "심음"
			if r[0] == "payoff" {
				k = "거둠"
			}
			line("- %s: %s", k, r[1])
		}
	}
	pl, err := q(`SELECT lint_short(sx.id), coalesce(nullif(su.text,''), render_fact_text($1, su.prop_id), ''), coalesce(lint_short(su.prop_id),''), coalesce((SELECT lint_short(z.base) FROM prop_norm($1, su.prop_id) z LIMIT 1),'')
	                FROM setup su JOIN scene sx ON sx.id=su.scene_id WHERE su.work_id=$1 AND sx.shown AND sx.discourse_ord < $2
	               ORDER BY sx.discourse_ord DESC LIMIT 8`, work, ord)
	if err != nil {
		return "", "", err
	}
	var pls []string
	for _, r := range pl {
		if r[1] != "" && !hidden[r[2]] && !hidden[r[3]] {
			pls = append(pls, fmt.Sprintf("- %s에서 심은 묘사: %s", r[0], r[1]))
		}
	}
	if len(pls) > 0 {
		line("\n### 앞서 심어 둔 묘사 복선의 원문 (최근 순; 이 표현과 어긋나지 않게)")
		for _, x := range pls {
			line("%s", x)
		}
	}
	out := b.String()
	if utf8.RuneCountInString(out) > digestCap {
		out = string([]rune(out)[:digestCap]) + "\n(이하 생략)\n"
	}
	return out, st.String(), nil
}
