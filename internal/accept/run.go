package accept

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunQueries runs each query's SQL with $1 = work ID and converts all values to strings (NULL → "").
// A failed query is absent from rows and its reason is in errs (Judge counts it as a mismatch).
func RunQueries(ctx context.Context, db *pgxpool.Pool, work string, qs []Query) (map[string][][]string, map[string]string) {
	out, errs := map[string][][]string{}, map[string]string{}
	for _, q := range qs {
		rows, err := strRows(ctx, db, q.SQL, work)
		if err != nil {
			errs[q.Name] = err.Error()
			continue
		}
		out[q.Name] = rows
	}
	return out, errs
}

func strRows(ctx context.Context, db *pgxpool.Pool, sql string, args ...any) ([][]string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // queries only read. Roll back whatever was written
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = public, ag_catalog`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := [][]string{}
	for rows.Next() {
		vs, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make([]string, len(vs))
		for i, v := range vs {
			if v != nil {
				row[i] = fmt.Sprint(v)
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
