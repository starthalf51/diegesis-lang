package accept

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"silmari/internal/ir"
	"silmari/internal/mutate"
	"silmari/internal/pg"
	"silmari/rules"
)

// RunMutants 는 변이마다 `<work>~m<i>` 로 적재하고 린트한 뒤 지운다. base 는 이미 적재·린트된 원본이고
// baseDiags 는 그 진단, qs 는 expect.yaml 의 질의다. 돌리기 전에 PG 로 변이를 확정한다(applicable): "받은 정보를 뒤에
// 쓰는" 쌍이 precedes = yes 인 것만 남기고, move-before 는 출처 시점을 채운다. 남지 않은 변이는 NotApplicable 로
// 목록에만 남는다(비율에서 뺀다).
// 탐지 = 원본에 없던 억제되지 않은 warn 중 Expect 규칙이 있고 그 대상이 변이 지점(Site) 안이다(지점 밖은 비고에만 적는다).
// 그렇지 않아도 질의 결과가 원본과 다르면 DetectedByQuery(R1). 둘 다 아닌데 파생 행이 원본과 같으면 동등(Equivalent),
// 다르면 생존(Survived). 적재·린트·DB 오류는 변이 결과가 아니라 실행 실패다 — 오류를 돌려준다.
// IR 변이는 컴파일을 거치지 않으므로 Caught(컴파일러가 잡음)는 여기서 나오지 않는다.
func RunMutants(ctx context.Context, db *pgxpool.Pool, base *ir.Work, baseDiags []pg.Diagnostic, qs []Query, seed int64, n int,
	log io.Writer) ([]Mutant, error) {
	h, err := newHarness(ctx, db, base, baseDiags, qs, log)
	if err != nil {
		return nil, err
	}
	work := base.Works[0].ID
	out := []Mutant{}
	for i, m := range mutate.Generate(base, seed, n) {
		res, err := h.run(ctx, fmt.Sprintf("%s~m%d", work, i+1), m)
		if err != nil {
			return nil, fmt.Errorf("m%d %s %s: %w", i+1, m.Op, shortAll(work, m.Target), err)
		}
		fmt.Fprintf(log, "m%d %s %s → %s %v %s\n", i+1, m.Op, shortAll(work, m.Target), res.Outcome, res.Rules, res.Note)
		out = append(out, res)
	}
	return out, nil
}

// harness 는 원본 쪽 기준값(원본의 warn, 파생 행 지문, 질의 결과)을 한 번 구해 두고 변이마다 쓴다.
// qs 는 변이본에 옮겨 돌릴 수 있는 질의만이다(원본을 이름만 바꿔 적재한 복사본에서 결과가 같은 것 — 그래프 이름처럼
// 작품 상수를 쓰는 질의는 빠진다). baseRows 는 그 질의의 원본 결과.
type harness struct {
	db        *pgxpool.Pool
	work      string
	base      *ir.Work
	before    map[string]bool
	basePrint print
	qs        []Query
	baseRows  map[string][][]string
}

func newHarness(ctx context.Context, db *pgxpool.Pool, base *ir.Work, baseDiags []pg.Diagnostic, qs []Query,
	log io.Writer) (*harness, error) {
	h := &harness{db: db, work: base.Works[0].ID, base: base, before: map[string]bool{}}
	for _, d := range baseDiags {
		if d.Severity == "warn" && !d.Waived {
			h.before[d.Rule+" "+short(h.work, d.TargetID)] = true
		}
	}
	var err error
	if h.basePrint, err = fingerprint(ctx, db, h.work, base); err != nil || len(qs) == 0 {
		return h, err
	}
	rows, _ := RunQueries(ctx, db, h.work, qs)
	copyID := h.work + "~m0"
	if _, err := loadLint(ctx, db, mutate.Rename(base, copyID)); err != nil {
		return nil, fmt.Errorf("%s: %w", copyID, err)
	}
	got, _ := RunQueries(ctx, db, copyID, qs)
	if err := drop(ctx, db, copyID); err != nil {
		return nil, err
	}
	h.baseRows = map[string][][]string{}
	for _, q := range qs {
		r, ok := rows[q.Name]
		switch {
		case readsDiagnostics.MatchString(q.SQL):
			fmt.Fprintf(log, "query %s: excluded from query detection (reads the diagnostic table — lint detection, site-local only)\n", q.Name)
		case !ok || !rowsEqual(r, got[q.Name]):
			fmt.Fprintf(log, "query %s: excluded from query detection (result differs on the renamed unmutated copy %s — not transportable)\n", q.Name, copyID)
		default:
			h.qs, h.baseRows[q.Name] = append(h.qs, q), r
		}
	}
	return h, nil
}

// readsDiagnostics: diagnostic 표를 읽는 질의는 린트 결과를 다시 보는 것이라(지점 한정을 우회한다) 질의 탐지에 쓰지 않는다.
var readsDiagnostics = regexp.MustCompile(`(?i)\bdiagnostic\b`)

// run 은 변이 하나를 id 로 적재·린트·판정하고 지운다.
func (h *harness) run(ctx context.Context, id string, m mutate.Mutation) (Mutant, error) {
	res := Mutant{Mutation: m}
	ok, why, err := applicable(ctx, h.db, h.work, h.base, &res.Mutation)
	if err != nil {
		return res, err
	}
	if !ok {
		res.Outcome, res.Note = NotApplicable, why
		return res, nil
	}
	hit, off := false, []string(nil)
	ds, err := loadLint(ctx, h.db, mutate.Rename(mutate.Apply(h.base, res.Mutation), id))
	if err == nil {
		for _, d := range ds {
			if d.Severity != "warn" || d.Waived || h.before[d.Rule+" "+short(id, d.TargetID)] {
				continue
			}
			if !slices.Contains(res.Rules, d.Rule) {
				res.Rules = append(res.Rules, d.Rule)
			}
			if hits(res.Mutation, h.work, id, d) {
				hit = true
			} else if slices.Contains(m.Expect, d.Rule) {
				off = append(off, d.Rule+"@"+short(id, d.TargetID))
			}
		}
		slices.Sort(res.Rules)
		// R1: 작가가 원문에서 적은 질의의 결과가 바뀌면 지점과 무관하게 탐지다(따로 센다)
		rows, _ := RunQueries(ctx, h.db, id, h.qs)
		for _, q := range h.qs {
			if r, ok := rows[q.Name]; ok && !rowsEqual(r, h.baseRows[q.Name]) { // 실행 실패는 탐지로 치지 않는다
				res.Queries = append(res.Queries, q.Name)
			}
		}
		var fp print
		fp, err = fingerprint(ctx, h.db, id, h.base)
		switch {
		case err != nil:
		case hit:
			res.Outcome = Detected
		case len(res.Queries) > 0:
			res.Outcome, res.Note = DetectedByQuery, "질의: "+strings.Join(res.Queries, ", ")
		case fp.equal(h.basePrint):
			res.Outcome, res.Note = Equivalent, "파생 행(believes·uses·role_span·공통 시점의 경계·간선 끝점의 선후)이 원본과 같다"
		default:
			res.Outcome, res.Note = Survived, h.basePrint.diff(fp)
		}
		if len(off) > 0 {
			res.Note = strings.TrimSpace(res.Note + " · 지점 밖(세지 않음): " + strings.Join(off, ", "))
		}
	}
	if derr := drop(ctx, h.db, id); err == nil {
		err = derr
	}
	return res, err
}

// hits: 새 warn d 가 변이 m 을 탐지한 것인가 — 기대 규칙이고 대상이 m.Site 안이다. time-cycle 은 대상이 순환의
// 대표 문장 하나뿐이라 data.points(순환이 지나는 시점들, 002/008) 중 하나가 Site 에 있어도 된다(변이한 시점을 지나는 간선).
func hits(m mutate.Mutation, work, id string, d pg.Diagnostic) bool {
	if !slices.Contains(m.Expect, d.Rule) {
		return false
	}
	in := func(x string) bool {
		return slices.ContainsFunc(m.Site, func(s string) bool { return short(work, s) == short(id, x) })
	}
	if in(d.TargetID) {
		return true
	}
	var data struct {
		Points []string `json:"points"`
	}
	if d.Rule == "time-cycle" && json.Unmarshal(d.Data, &data) == nil {
		return slices.ContainsFunc(data.Points, in)
	}
	return false
}

// applicable 은 PG 로 변이를 확정한다. Pairs 가 있으면 precedes(받음, 씀) = yes 인 쌍이 하나라도 있어야 한다.
// move-before 는 화자(와 그룹)가 쓰는 명제를 얻은 시점들(believes.from_pt, 자신의 decrees)을 Before 로 채운다.
func applicable(ctx context.Context, db *pgxpool.Pool, work string, base *ir.Work, m *mutate.Mutation) (bool, string, error) {
	if len(m.Pairs) > 0 {
		var a, b []string
		for _, p := range m.Pairs {
			a, b = append(a, p[0]), append(b, p[1])
		}
		// 쓰는 사람이 받은 믿음(그 행위에서 온 행)을 쓰기 전에 모두 의심으로 거뒀으면 그 쌍은 결함을 못 만든다(006).
		rows, err := strRows(ctx, db, `SELECT coalesce(bool_or(precedes($1, a, b) = 'yes' AND NOT coalesce((
		    SELECT bool_and(bl.until_pt IS NOT NULL AND bl.until_pt <> b AND precedes($1, bl.until_pt, b) = 'yes')
		      FROM believes bl JOIN act u ON u.id = b
		     WHERE bl.work_id = $1 AND bl.cause_act = a AND bl.entity_id = u.resolved_actor_id AND bl.value), false)), false)::text
		  FROM unnest($2::text[], $3::text[]) t(a, b)`, work, a, b)
		if err != nil {
			return false, "", err
		}
		if rows[0][0] != "true" {
			return false, "받은 정보를 쓰는 행위가 모두 그 전이거나 순서가 정해지지 않았거나, 쓰기 전에 의심으로 거뒀다", nil
		}
	}
	if m.Op != "move-before" {
		return true, "", nil
	}
	actor := ""
	for _, a := range base.Acts {
		if a.ID == m.Target {
			actor = a.ActorID
		}
	}
	// Uses 는 Go 의 한 단계 별칭 풀이다. PG 정규형(008: 부정 안의 별칭)으로 다시 맞춘다.
	rows, err := strRows(ctx, db, `
	  WITH c AS (SELECT lint_canon($1, u) AS c FROM unnest($4::text[]) u)
	  SELECT DISTINCT b.from_pt FROM believes b JOIN lint_self_and_groups($1, $2) u ON u.id = b.entity_id
	   WHERE b.work_id = $1 AND b.value AND b.from_pt IS NOT NULL AND b.from_pt <> $3 AND lint_canon($1, b.prop_id) IN (SELECT c FROM c)
	  UNION
	  SELECT x.pt FROM act_content x JOIN lint_self_and_groups($1, $2) u ON u.id = x.actor_id
	   WHERE x.work_id = $1 AND x.verb = 'decrees' AND x.pt <> $3 AND lint_canon($1, x.prop_id) IN (SELECT c FROM c)
	  ORDER BY 1`, work, actor, m.Target, m.Uses)
	if err != nil {
		return false, "", err
	}
	for _, r := range rows {
		m.Before = append(m.Before, r[0])
	}
	if len(m.Before) == 0 {
		return false, "쓰는 명제의 출처 시점이 없다(배경 지식뿐)", nil
	}
	return true, "", nil
}

func loadLint(ctx context.Context, db *pgxpool.Pool, w *ir.Work) ([]pg.Diagnostic, error) {
	if err := pg.Load(ctx, db, w); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return pg.Lint(ctx, db, w.Works[0].ID, rules.FS)
}

func drop(ctx context.Context, db *pgxpool.Pool, work string) error {
	if _, err := db.Exec(ctx, `DELETE FROM public.work WHERE id = $1`, work); err != nil {
		return fmt.Errorf("drop %s: %w", work, err)
	}
	_, err := db.Exec(ctx, `SELECT ag_catalog.drop_graph(name, true) FROM ag_catalog.ag_graph WHERE name = public.graph_name($1)`, work)
	if err != nil {
		return fmt.Errorf("drop graph %s: %w", work, err)
	}
	return nil
}

// print 는 작품의 파생 행 지문(작품 ID 접두사를 뺀 문자열). rows = believes·uses·role_span 다중집합,
// points = 시점 → 경계, 그리고 원본 IR 의 간선마다 두 끝점의 선후(`order a b` → precedes(a,b)|precedes(b,a), PG) —
// 순서만 바꾼 변이도 파생 결과가 다르다. 시점·쌍은 양쪽에 다 있는 것만 비교한다(지운 행위의 시점은 비교 대상이 아니다).
type print struct {
	rows   []string
	points map[string]string
}

const fingerprintSQL = `
WITH r(x) AS (
  SELECT concat_ws('|', 'believes', entity_id, prop_id, value, from_pt, cause_act, declared, until_pt) FROM believes WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'uses', entity_id, prop_id, at_pt, act_id) FROM uses WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'role_span', role_id, entity_id, from_pt, until_pt, span, shared) FROM role_span WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'point', id, coalesce(earliest::text, '-'), coalesce(latest::text, '-')) FROM point WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'point', 'order ' || a || ' ' || b, precedes($1, $1 || '/' || a, $1 || '/' || b),
                            precedes($1, $1 || '/' || b, $1 || '/' || a))
    FROM unnest($2::text[], $3::text[]) e(a, b)
   WHERE EXISTS (SELECT 1 FROM point WHERE id = $1 || '/' || a) AND EXISTS (SELECT 1 FROM point WHERE id = $1 || '/' || b))
SELECT replace(x, $1 || '/', '') FROM r ORDER BY 1`

func fingerprint(ctx context.Context, db *pgxpool.Pool, work string, base *ir.Work) (print, error) {
	var a, b []string
	for _, e := range base.TimeEdges {
		a, b = append(a, short(base.Works[0].ID, e.From)), append(b, short(base.Works[0].ID, e.To))
	}
	rows, err := strRows(ctx, db, fingerprintSQL, work, a, b)
	if err != nil {
		return print{}, err
	}
	p := print{points: map[string]string{}}
	for _, r := range rows {
		if rest, ok := strings.CutPrefix(r[0], "point|"); ok {
			id, bounds, _ := strings.Cut(rest, "|")
			p.points[id] = bounds
			continue
		}
		p.rows = append(p.rows, r[0])
	}
	return p, nil
}

func (p print) equal(q print) bool { return p.diff(q) == "" }

// diff 는 첫 차이 한 줄(같으면 "").
func (p print) diff(q print) string {
	if !slices.Equal(p.rows, q.rows) {
		for _, r := range p.rows {
			if !slices.Contains(q.rows, r) {
				return "원본에만: " + r
			}
		}
		for _, r := range q.rows {
			if !slices.Contains(p.rows, r) {
				return "변이본에만: " + r
			}
		}
		return "행 수가 다르다"
	}
	for _, id := range slices.Sorted(maps.Keys(p.points)) {
		if b, ok := q.points[id]; ok && b != p.points[id] {
			return fmt.Sprintf("시점 %s: %s → %s", id, p.points[id], b)
		}
	}
	return ""
}
