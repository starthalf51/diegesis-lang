package pg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/patterns"
)

// Pattern is one patterns/*.sql file. Read from the header comments `-- pattern:` `-- layer:` `-- summary:`.
type Pattern struct {
	ID, Layer, Summary, Path, SQL string
}

// Match is one pattern_match row. Span is a discourse_ord range (nil = does not touch a visible scene).
// Declared = the author's declaration of the same pattern covers this match (pattern_covers).
type Match struct {
	Pattern    string          `json:"pattern"`
	TargetKind string          `json:"target_kind"`
	TargetID   string          `json:"target_id"`
	SpanFrom   *int            `json:"span_from"`
	SpanTo     *int            `json:"span_to"`
	Data       json.RawMessage `json:"data"`
	Declared   bool            `json:"declared"`
}

var layers = map[string]bool{"foreshadow": true, "information": true, "deception": true, "identity": true, "structure": true}

// PatternDefs reads the embedded pattern library. The pattern ID must equal the file name.
func PatternDefs() ([]Pattern, error) { return readPatterns(patterns.FS) }

func readPatterns(dir fs.FS) ([]Pattern, error) {
	var out []Pattern
	err := fs.WalkDir(dir, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".sql") {
			return err
		}
		b, err := fs.ReadFile(dir, p)
		if err != nil {
			return err
		}
		x := Pattern{Path: p, SQL: strings.TrimRight(strings.TrimSpace(string(b)), ";")}
		sc := bufio.NewScanner(strings.NewReader(x.SQL))
		for sc.Scan() {
			m := headerLine.FindStringSubmatch(sc.Text())
			if m == nil {
				break
			}
			switch k, v := m[1], strings.TrimSpace(m[2]); k {
			case "pattern":
				x.ID = v
			case "layer":
				x.Layer = v
			case "summary":
				x.Summary = v
			}
		}
		switch {
		case x.ID != strings.TrimSuffix(path.Base(p), ".sql"):
			return fmt.Errorf("%s: header pattern %q must match the file name", p, x.ID)
		case !layers[x.Layer]:
			return fmt.Errorf("%s: layer %q, want foreshadow|information|deception|identity|structure", p, x.Layer)
		}
		out = append(out, x)
		return nil
	})
	return out, err
}

// evalPatterns deletes the rows of pattern_match of the same kind of run (run prefix lint- | patterns-) for the work, then runs the patterns and refills it.
// If only is not nil, only those IDs are run (lint). A partial lint run does not overwrite the full run (the pattern_result view).
// Wrap the pattern SQL as a subquery and INSERT directly inside PG (a mismatched column name is a pattern file error).
func evalPatterns(ctx context.Context, tx pgx.Tx, work, run string, only map[string]bool) error {
	ps, err := PatternDefs()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pattern_match WHERE work_id = $1 AND split_part(run_id, '-', 1) = split_part($2, '-', 1)`, work, run); err != nil {
		return err
	}
	for _, p := range ps {
		if only != nil && !only[p.ID] {
			continue
		}
		q := `INSERT INTO pattern_match(run_id, work_id, pattern_id, target_kind, target_id, span_from, span_to, data)
		      SELECT $2, $1, $3, q.target_kind, q.target_id, q.span_from, q.span_to, q.data FROM (` + p.SQL + "\n) q"
		if _, err := tx.Exec(ctx, q, work, run, p.ID); err != nil {
			return fmt.Errorf("pattern %s: %w", p.Path, err)
		}
	}
	return nil
}

// declaredPatterns are the pattern IDs declared in the work (lint runs only these — no declarations means no cost).
func declaredPatterns(ctx context.Context, tx pgx.Tx, work string) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT pattern_id FROM pattern_decl WHERE work_id = $1`, work)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

// Patterns runs the whole pattern library on a work, puts the results in pattern_match (previous results are deleted) and returns the matches.
func Patterns(ctx context.Context, db *pgxpool.Pool, work string) ([]Match, error) {
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
		return nil, fmt.Errorf("patterns: work %q is not loaded", work)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = public, ag_catalog`); err != nil {
		return nil, err
	}
	run := fmt.Sprintf("patterns-%d", time.Now().UnixNano())
	if err := evalPatterns(ctx, tx, work, run, nil); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
SELECT m.pattern_id, m.target_kind, m.target_id, m.span_from, m.span_to, coalesce(m.data, 'null'),
       EXISTS (SELECT 1 FROM pattern_decl d WHERE d.work_id = $1 AND d.pattern_id = m.pattern_id
                  AND pattern_covers($1, d.scene_id, m.target_id, m.span_from, m.span_to))
  FROM pattern_match m WHERE m.work_id = $1 AND m.run_id = $2
 ORDER BY m.pattern_id, m.span_from NULLS LAST, m.target_id`, work, run)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Match, error) {
		var m Match
		var data []byte
		err := r.Scan(&m.Pattern, &m.TargetKind, &m.TargetID, &m.SpanFrom, &m.SpanTo, &data, &m.Declared)
		m.Data = data
		return m, err
	})
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
