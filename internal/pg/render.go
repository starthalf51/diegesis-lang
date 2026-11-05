package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RenderScene 은 렌더 요청 JSON(render_scene, 009)을 돌려준다. 장면은 전체 ID 또는 짧은 ID(`1.3`).
func RenderScene(ctx context.Context, db *pgxpool.Pool, work, scene string) ([]byte, error) {
	var b []byte
	err := db.QueryRow(ctx, `SELECT render_scene($1, $2)::text`, work, scene).Scan(&b)
	return b, err
}
