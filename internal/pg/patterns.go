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

	"diegesis/patterns"
)

// Pattern 은 patterns/*.sql 파일 하나. 헤더 주석 `-- pattern:` `-- layer:` `-- summary:` 에서 읽는다.
type Pattern struct {
	ID, Layer, Summary, Path, SQL string
}

// Match 는 pattern_match 행 하나. Span 은 discourse_ord 구간(nil = 보이는 장면에 걸리지 않음).
// Declared = 같은 패턴의 작가 선언이 이 매치를 덮는다(pattern_covers).
type Match struct {
	Pattern    string          `json:"pattern"`
	TargetKind string          `json:"target_kind"`
	TargetID   string          `json:"target_id"`
	SpanFrom   *int            `json:"span_from"`
	SpanTo     *int            `json:"span_to"`
	Data       json.RawMessage `json:"data"`
	Declared   bool            `json:"declared"`
}

var layers = map[string]bool{"foreshadow": true, "information": true, "deception": true, "identity": true}

// PatternDefs 는 내장된 패턴 라이브러리를 읽는다. 패턴 ID 는 파일 이름과 같아야 한다.
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
			return fmt.Errorf("%s: layer %q, want foreshadow|information|deception|identity", p, x.Layer)
		}
		out = append(out, x)
		return nil
	})
	return out, err
}

// evalPatterns 는 작품의 pattern_match 를 지우고 패턴을 돌려 다시 채운다. only 가 nil 이 아니면 그 ID 만 돌린다.
// 패턴 SQL 을 부분 질의로 감싸 PG 안에서 바로 INSERT 한다(열 이름이 다르면 패턴 파일 오류다).
func evalPatterns(ctx context.Context, tx pgx.Tx, work, run string, only map[string]bool) error {
	ps, err := PatternDefs()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pattern_match WHERE work_id = $1`, work); err != nil {
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

// declaredPatterns 는 작품에 선언된 패턴 ID 들(lint 는 이것만 돌린다 — 선언이 없으면 비용이 없다).
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

// Patterns 는 작품 하나에 패턴 라이브러리 전체를 돌려 pattern_match 에 넣고(이전 결과는 지운다) 매치를 돌려준다.
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
	if err := evalPatterns(ctx, tx, work, fmt.Sprintf("patterns-%d", time.Now().UnixNano()), nil); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
SELECT m.pattern_id, m.target_kind, m.target_id, m.span_from, m.span_to, coalesce(m.data, 'null'),
       EXISTS (SELECT 1 FROM pattern_decl d WHERE d.work_id = $1 AND d.pattern_id = m.pattern_id
                  AND pattern_covers($1, d.scene_id, m.target_id, m.span_from, m.span_to))
  FROM pattern_match m WHERE m.work_id = $1
 ORDER BY m.pattern_id, m.span_from NULLS LAST, m.target_id`, work)
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
