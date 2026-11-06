package pg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rule 은 rules/*.sql 파일 하나. 헤더 주석 `-- key: value` 에서 읽는다.
type Rule struct {
	ID, Severity, Layer, Summary, Path, SQL string
}

// Diagnostic 은 diagnostic 행 하나에 소스 위치와 억제 사유를 붙인 것.
type Diagnostic struct {
	RunID      string          `json:"run_id"`
	Rule       string          `json:"rule"`
	Severity   string          `json:"severity"`
	TargetKind string          `json:"target_kind"`
	TargetID   string          `json:"target_id"`
	File       string          `json:"file"`
	Line       int             `json:"line"`
	Col        int             `json:"col"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	Waived     bool            `json:"waived"`
	Reason     string          `json:"reason,omitempty"`
}

func (d Diagnostic) String() string {
	s := fmt.Sprintf("%s:%d:%d: %s %s: %s", d.File, d.Line, d.Col, d.Severity, d.Rule, d.Message)
	if d.Waived {
		s += " [waived: " + d.Reason + "]"
	}
	return s
}

var headerLine = regexp.MustCompile(`^--\s*([a-z]+):(.*)$`)

// Rules 는 dir 아래(하위 폴더 포함) *.sql 을 읽는다. 규칙 ID 는 파일 이름과 같아야 한다.
func Rules(dir fs.FS) ([]Rule, error) {
	var out []Rule
	err := fs.WalkDir(dir, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".sql") {
			return err
		}
		b, err := fs.ReadFile(dir, p)
		if err != nil {
			return err
		}
		r := Rule{Path: p, SQL: string(b), Layer: "lint"}
		// 헤더 = 맨 위에 이어지는 `-- key: value` 줄. 처음으로 그 꼴이 아닌 줄에서 끝난다.
		sc := bufio.NewScanner(strings.NewReader(r.SQL))
		for sc.Scan() {
			m := headerLine.FindStringSubmatch(sc.Text())
			if m == nil {
				break
			}
			switch k, v := m[1], strings.TrimSpace(m[2]); k {
			case "rule":
				r.ID = v
			case "severity":
				r.Severity = v
			case "layer":
				r.Layer = v
			case "summary":
				r.Summary = v
			}
		}
		switch {
		case r.ID != strings.TrimSuffix(path.Base(p), ".sql"):
			return fmt.Errorf("%s: header rule %q must match the file name", p, r.ID)
		case r.Severity != "warn" && r.Severity != "info":
			return fmt.Errorf("%s: severity %q, want warn|info", p, r.Severity)
		case r.Layer != "lint" && r.Layer != "derive":
			return fmt.Errorf("%s: layer %q, want lint|derive", p, r.Layer)
		}
		out = append(out, r)
		return nil
	})
	return out, err
}

// targetSrc 는 진단 대상의 원문 위치. 규칙이 src_id 를 직접 주지 않았을 때 쓴다.
// 장면 안 문장(when belief presence …) 의 억제 대상은 그 장면이다(sema).
const targetSrc = `
UPDATE diagnostic d SET src_id = t.src_id FROM (
  SELECT 'act' k, id, src_id FROM act WHERE work_id = $1
  UNION ALL SELECT 'setup', id, src_id FROM setup WHERE work_id = $1
  UNION ALL SELECT 'payoff', setup_id, min(src_id) FROM payoff WHERE work_id = $1 GROUP BY setup_id
  UNION ALL SELECT 'entity', id, src_id FROM entity WHERE work_id = $1
  UNION ALL SELECT 'role', id, src_id FROM role WHERE work_id = $1
  UNION ALL SELECT 'fact', id, src_id FROM fact WHERE work_id = $1
  UNION ALL SELECT 'artifact', id, src_id FROM artifact WHERE work_id = $1
  UNION ALL SELECT 'account', id, src_id FROM account WHERE work_id = $1
  UNION ALL SELECT 'beat', id, src_id FROM point WHERE work_id = $1
  UNION ALL SELECT 'work', id, src_id FROM work WHERE id = $1
  UNION ALL SELECT k, id, src_id FROM scene,
    unnest('{scene,event,when,belief,presence,reveal,withhold,directive}'::text[]) k WHERE work_id = $1
) t WHERE d.work_id = $1 AND d.run_id = $2 AND d.src_id IS NULL AND t.k = d.target_kind AND t.id = d.target_id`

// 억제: 규칙과 대상(kind, id)이 정확히 같은 waiver 만(Review Focus 1). derive 진단에도 적용한다.
const applyWaivers = `
UPDATE diagnostic d SET waived = true
 WHERE d.work_id = $1 AND d.run_id IN ($2, 'derive') AND NOT d.waived AND EXISTS (
   SELECT 1 FROM waiver v WHERE v.work_id = $1 AND v.rule = d.rule_id
      AND v.target_kind = d.target_kind AND v.target_id = d.target_id)`

const selectDiags = `
SELECT d.run_id, d.rule_id, d.severity, coalesce(d.target_kind, ''), coalesce(d.target_id, ''),
       coalesce(s.file, ''), coalesce(s.line, 0), coalesce(s.col, 0), d.message, coalesce(d.data, 'null'), d.waived,
       coalesce((SELECT v.reason FROM waiver v WHERE v.work_id = $1 AND v.rule = d.rule_id
                    AND v.target_kind = d.target_kind AND v.target_id = d.target_id LIMIT 1),
                (SELECT e.waiver_reason FROM time_edge e WHERE e.work_id = $1 AND e.waived AND e.waiver_rule = d.rule_id
                    AND d.data ? 'edges' AND e.ord IN (SELECT jsonb_array_elements_text(d.data->'edges')::int)
                  ORDER BY e.ord LIMIT 1), '')
  FROM diagnostic d LEFT JOIN src s ON s.work_id = d.work_id AND s.id = d.src_id
 WHERE d.work_id = $1 AND d.run_id IN ($2, 'derive')
 ORDER BY s.file, s.line, s.col, d.rule_id, d.target_id`

// Lint 는 작품 하나에 lint 층 규칙을 전부 돌려 diagnostic 에 새 run_id 로 넣고(이전 lint 결과는 지운다),
// 대상의 src 를 붙이고, 정확히 일치하는 waiver 로 억제를 표시한 뒤, derive 진단과 함께 돌려준다.
// 판정은 규칙 SQL 이 한다. 여기서는 열을 옮겨 담을 뿐이다(ADR-010).
func Lint(ctx context.Context, db *pgxpool.Pool, work string, rulesDir fs.FS) ([]Diagnostic, error) {
	rules, err := Rules(rulesDir)
	if err != nil {
		return nil, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.work WHERE id = $1)`, work).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("lint: work %q is not loaded", work)
	}
	run := fmt.Sprintf("lint-%d", time.Now().UnixNano())
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = public, ag_catalog`); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM diagnostic WHERE work_id = $1 AND run_id <> 'derive'`, work); err != nil {
		return nil, err
	}
	// 선언된 패턴만 먼저 돌린다: 규칙 pattern-missing 이 pattern_match 를 읽는다
	only, err := declaredPatterns(ctx, tx, work)
	if err != nil {
		return nil, err
	}
	if err := evalPatterns(ctx, tx, work, run, only); err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.Layer != "lint" {
			continue // derive 층(time-cycle 등)은 적재 때 이미 diagnostic 에 있다
		}
		rows, err := runRule(ctx, tx, r, work)
		if err != nil {
			return nil, err
		}
		for _, x := range rows {
			if _, err := tx.Exec(ctx, `INSERT INTO diagnostic(run_id, work_id, rule_id, severity, target_kind, target_id,
				src_id, message, data) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				run, work, r.ID, x.severity, x.kind, x.id, x.src, x.msg, x.data); err != nil {
				return nil, fmt.Errorf("%s: %w", r.Path, err)
			}
		}
	}
	for _, q := range []string{targetSrc, applyWaivers} {
		if _, err := tx.Exec(ctx, q, work, run); err != nil {
			return nil, err
		}
	}
	out, err := collect(ctx, tx, work, run)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

type ruleRow struct {
	kind, id, msg, severity string
	src                     any
	data                    []byte
}

func runRule(ctx context.Context, tx pgx.Tx, r Rule, work string) ([]ruleRow, error) {
	rows, err := tx.Query(ctx, r.SQL, work)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.Path, err)
	}
	defer rows.Close()
	col := map[string]int{}
	for i, f := range rows.FieldDescriptions() {
		col[f.Name] = i
	}
	for _, c := range []string{"target_kind", "target_id", "message", "data"} {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("%s: result has no %s column", r.Path, c)
		}
	}
	str := func(v []any, c string) string {
		if i, ok := col[c]; ok && v[i] != nil {
			return fmt.Sprint(v[i])
		}
		return ""
	}
	var out []ruleRow
	for rows.Next() {
		v, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.Path, err)
		}
		x := ruleRow{kind: str(v, "target_kind"), id: str(v, "target_id"), msg: str(v, "message"), severity: r.Severity}
		if s := str(v, "severity"); s != "" {
			x.severity = s
		}
		if x.severity != "warn" && x.severity != "info" {
			return nil, fmt.Errorf("%s: row severity %q, want warn|info", r.Path, x.severity)
		}
		if i, ok := col["src_id"]; ok {
			x.src = v[i]
		}
		if x.data, err = json.Marshal(v[col["data"]]); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func collect(ctx context.Context, tx pgx.Tx, work, run string) ([]Diagnostic, error) {
	rows, err := tx.Query(ctx, selectDiags, work, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Diagnostic
	for rows.Next() {
		var d Diagnostic
		var data []byte
		if err := rows.Scan(&d.RunID, &d.Rule, &d.Severity, &d.TargetKind, &d.TargetID, &d.File, &d.Line, &d.Col,
			&d.Message, &data, &d.Waived, &d.Reason); err != nil {
			return nil, err
		}
		d.Data = data
		out = append(out, d)
	}
	return out, rows.Err()
}
