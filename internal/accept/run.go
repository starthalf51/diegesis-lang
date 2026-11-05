package accept

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunQueries 는 질의마다 SQL 을 $1 = 작품 ID 로 돌려 모든 값을 문자열로 바꾼다(NULL → "").
// 실패한 질의는 rows 에 없고 errs 에 사유가 있다(Judge 가 불일치로 센다).
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
	defer tx.Rollback(ctx) // 질의는 읽기만 한다. 무엇을 쓰든 되돌린다
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
