package pg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rule is one rules/*.sql file. Read from the header comment `-- key: value`.
type Rule struct {
	ID, Severity, Layer, Summary, Path, SQL string
}

// Diagnostic is a diagnostic row with the source position and suppression reason attached.
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

// Rules reads *.sql under dir (subfolders included). The rule ID must equal the file name.
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
		// Header = the consecutive `-- key: value` lines at the very top. It ends at the first line not of that form.
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

// MergeRules appends extra (project-local rules) to base (built-in rules). An ID that already exists in base,
// or repeats inside extra, is an error: built-in rules cannot be overridden.
func MergeRules(base, extra []Rule) ([]Rule, error) {
	seen := map[string]bool{}
	for _, r := range base {
		seen[r.ID] = true
	}
	out := slices.Clone(base)
	for _, r := range extra {
		if seen[r.ID] {
			return nil, fmt.Errorf("rules: duplicate rule id %q (%s)", r.ID, r.Path)
		}
		seen[r.ID] = true
		out = append(out, r)
	}
	return out, nil
}

// targetSrc is the source position of a diagnostic target. Used when the rule did not give src_id directly.
// The suppression target of a statement inside a scene (when belief presence …) is that scene (sema).
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

// Suppression: only waivers whose rule and target (kind, id) match exactly (Review Focus 1). Also applied to derive diagnostics.
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

// Lint runs every lint-layer rule on a work and inserts the results into diagnostic under a new run_id (previous lint results are deleted),
// attaches the targets' src, marks suppression with exactly matching waivers, then returns them together with the derive diagnostics.
// The verdict is made by the rule SQL. This only copies the columns (ADR-010).
// extra are project rule directories, merged after the built-in ones (IDs must not collide).
func Lint(ctx context.Context, db *pgxpool.Pool, work string, rulesDir fs.FS, extra ...fs.FS) ([]Diagnostic, error) {
	rules, err := Rules(rulesDir)
	if err != nil {
		return nil, err
	}
	for _, x := range extra {
		more, err := Rules(x)
		if err != nil {
			return nil, err
		}
		if rules, err = MergeRules(rules, more); err != nil {
			return nil, err
		}
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
	// run only the declared patterns first: the pattern-missing rule reads pattern_match
	only, err := declaredPatterns(ctx, tx, work)
	if err != nil {
		return nil, err
	}
	if err := evalPatterns(ctx, tx, work, run, only); err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.Layer != "lint" {
			continue // derive-layer rules (time-cycle etc.) are already in diagnostic from load time
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
