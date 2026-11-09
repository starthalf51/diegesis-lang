package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RenderScene returns the render request JSON (render_scene, 009). The scene is a full ID or a short ID (`1.3`).
func RenderScene(ctx context.Context, db *pgxpool.Pool, work, scene string) ([]byte, error) {
	var b []byte
	err := db.QueryRow(ctx, `SELECT render_scene($1, $2)::text`, work, scene).Scan(&b)
	return b, err
}
