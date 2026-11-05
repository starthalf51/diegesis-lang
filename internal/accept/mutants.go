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
// baseDiags 는 그 진단이다. 돌리기 전에 PG 로 변이를 확정한다(applicable): "받은 정보를 뒤에 쓰는" 쌍이
// precedes = yes 인 것만 남기고, move-before 는 출처 시점을 채운다. 남지 않은 변이는 NotApplicable 로
// 목록에만 남는다(비율에서 뺀다).
// 탐지 = 원본에 없던 억제되지 않은 warn 중 Expect 규칙이 있다. 탐지되지 않았는데 파생 행이 원본과 같으면
// 동등(Equivalent), 다르면 생존(Survived). 적재·린트·DB 오류는 변이 결과가 아니라 실행 실패다 — 오류를 돌려준다.
// IR 변이는 컴파일을 거치지 않으므로 Caught(컴파일러가 잡음)는 여기서 나오지 않는다.
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
		ok, why, err := applicable(ctx, db, work, base, &res.Mutation)
		if err != nil {
			return nil, fmt.Errorf("m%d %s: %w", i+1, m.Op, err)
		}
		if !ok {
			res.Outcome, res.Note = NotApplicable, why
			fmt.Fprintf(log, "m%d %s %s → %s (%s)\n", i+1, m.Op, shortAll(work, m.Target), res.Outcome, why)
			out = append(out, res)
			continue
		}
		ds, err := loadLint(ctx, db, mutate.Rename(mutate.Apply(base, res.Mutation), id))
		if err == nil {
			for _, d := range ds {
				if d.Severity == "warn" && !d.Waived && !before[d.Rule+" "+short(id, d.TargetID)] && !slices.Contains(res.Rules, d.Rule) {
					res.Rules = append(res.Rules, d.Rule)
				}
			}
			slices.Sort(res.Rules)
			var fp print
			fp, err = fingerprint(ctx, db, id)
			switch {
			case err != nil:
			case slices.ContainsFunc(res.Rules, func(r string) bool { return slices.Contains(m.Expect, r) }):
				res.Outcome = Detected
			case fp.equal(basePrint):
				res.Outcome, res.Note = Equivalent, "파생 행(believes·uses·role_span·공통 시점의 경계)이 원본과 같다"
			default:
				res.Outcome, res.Note = Survived, basePrint.diff(fp)
			}
		}
		if derr := drop(ctx, db, id); err == nil {
			err = derr
		}
		if err != nil {
			return nil, fmt.Errorf("m%d %s %s: %w", i+1, m.Op, shortAll(work, m.Target), err)
		}
		fmt.Fprintf(log, "m%d %s %s → %s %v\n", i+1, m.Op, shortAll(work, m.Target), res.Outcome, res.Rules)
		out = append(out, res)
	}
	return out, nil
}

// applicable 은 PG 로 변이를 확정한다. Pairs 가 있으면 precedes(받음, 씀) = yes 인 쌍이 하나라도 있어야 한다.
// move-before 는 화자(와 그룹)가 쓰는 명제를 얻은 시점들(believes.from_pt, 자신의 decrees)을 Before 로 채운다.
func applicable(ctx context.Context, db *pgxpool.Pool, work string, base *ir.Work, m *mutate.Mutation) (bool, string, error) {
	if len(m.Pairs) > 0 {
		var a, b []string
		for _, p := range m.Pairs {
			a, b = append(a, p[0]), append(b, p[1])
		}
		rows, err := strRows(ctx, db, `SELECT coalesce(bool_or(precedes($1, a, b) = 'yes'), false)::text
		  FROM unnest($2::text[], $3::text[]) t(a, b)`, work, a, b)
		if err != nil {
			return false, "", err
		}
		if rows[0][0] != "true" {
			return false, "받은 정보를 쓰는 행위가 모두 그 전이거나 순서가 정해지지 않았다", nil
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
// points = 시점 → 경계. 시점은 양쪽에 다 있는 것만 비교한다(지운 행위의 시점은 비교 대상이 아니다).
type print struct {
	rows   []string
	points map[string]string
}

const fingerprintSQL = `
WITH r(x) AS (
  SELECT concat_ws('|', 'believes', entity_id, prop_id, value, from_pt, cause_act, declared, until_pt) FROM believes WHERE work_id = $1
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
