package accept

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"story-lang/internal/ir"
	"story-lang/internal/mutate"
	"story-lang/internal/pg"
	"story-lang/rules"
)

// RunMutants 는 변이마다 `<work>~m<i>` 로 적재하고 린트한 뒤 지운다. base 는 이미 적재·린트된 원본이고
// baseDiags 는 그 진단이다. 탐지 = 원본에 없던 억제되지 않은 warn 중 Expect 규칙이 있다.
// 적재·파생·린트가 실패하면 도구 사슬이 잡은 것(Caught)이다. 탐지되지 않았는데 파생 행이 원본과 같으면
// 동등(Equivalent), 다르면 생존(Survived).
func RunMutants(ctx context.Context, db *pgxpool.Pool, base *ir.Work, baseDiags []pg.Diagnostic, seed int64, n int,
	log io.Writer) ([]Mutant, error) {
	work := base.Works[0].ID
	before := map[string]bool{}
	for _, d := range baseDiags {
		if d.Severity == "warn" && !d.Waived {
			before[d.Rule+" "+short(work, d.TargetID)] = true
		}
	}
	basePrint, err := fingerprint(ctx, db, work)
	if err != nil {
		return nil, err
	}
	out := []Mutant{}
	for i, m := range mutate.Generate(base, seed, n) {
		id := fmt.Sprintf("%s~m%d", work, i+1)
		res := Mutant{Mutation: m}
		ds, err := loadLint(ctx, db, mutate.Rename(mutate.Apply(base, m), id))
		if err != nil {
			res.Outcome, res.Note = Caught, err.Error()
		} else {
			for _, d := range ds {
				if d.Severity == "warn" && !d.Waived && !before[d.Rule+" "+short(id, d.TargetID)] && !slices.Contains(res.Rules, d.Rule) {
					res.Rules = append(res.Rules, d.Rule)
				}
			}
			slices.Sort(res.Rules)
			switch fp, err := fingerprint(ctx, db, id); {
			case slices.ContainsFunc(res.Rules, func(r string) bool { return slices.Contains(m.Expect, r) }):
				res.Outcome = Detected
			case err != nil:
				res.Outcome, res.Note = Survived, err.Error()
			case fp.equal(basePrint):
				res.Outcome, res.Note = Equivalent, "파생 행(believes·uses·role_span·공통 시점의 경계)이 원본과 같다"
			default:
				res.Outcome, res.Note = Survived, basePrint.diff(fp)
			}
		}
		drop(ctx, db, id)
		fmt.Fprintf(log, "m%d %s %s → %s %v\n", i+1, m.Op, shortAll(work, m.Target), res.Outcome, res.Rules)
		out = append(out, res)
	}
	return out, nil
}

func loadLint(ctx context.Context, db *pgxpool.Pool, w *ir.Work) ([]pg.Diagnostic, error) {
	if err := pg.Load(ctx, db, w); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return pg.Lint(ctx, db, w.Works[0].ID, rules.FS)
}

func drop(ctx context.Context, db *pgxpool.Pool, work string) {
	db.Exec(ctx, `DELETE FROM public.work WHERE id = $1`, work)
	db.Exec(ctx, `SELECT ag_catalog.drop_graph(name, true) FROM ag_catalog.ag_graph WHERE name = public.graph_name($1)`, work)
}

// print 는 작품의 파생 행 지문(작품 ID 접두사를 뺀 문자열). rows = believes·uses·role_span 다중집합,
// points = 시점 → 경계. 시점은 양쪽에 다 있는 것만 비교한다(지운 행위의 시점은 비교 대상이 아니다).
type print struct {
	rows   []string
	points map[string]string
}

const fingerprintSQL = `
WITH r(x) AS (
  SELECT concat_ws('|', 'believes', entity_id, prop_id, value, from_pt, cause_act, declared) FROM believes WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'uses', entity_id, prop_id, at_pt, act_id) FROM uses WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'role_span', role_id, entity_id, from_pt, until_pt, span, shared) FROM role_span WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'point', id, coalesce(earliest::text, '-'), coalesce(latest::text, '-')) FROM point WHERE work_id = $1)
SELECT replace(x, $1 || '/', '') FROM r ORDER BY 1`

func fingerprint(ctx context.Context, db *pgxpool.Pool, work string) (print, error) {
	rows, err := strRows(ctx, db, fingerprintSQL, work)
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
			return fmt.Sprintf("시점 %s 경계 %s → %s", id, p.points[id], b)
		}
	}
	return ""
}
