// Package pg handles the PostgreSQL connection and running migrations. Judgment logic lives in SQL.
package pg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://sl:sl@localhost:55432/sl?sslmode=disable"

// Connect opens a pool with SL_PG_DSN (or the default if unset).
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("SL_PG_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// ALTER DATABASE settings do not apply to already open connections, so load them directly per connection.
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, `LOAD 'age'; SET search_path = ag_catalog, "$user", public`)
		return err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// Migrate applies the *.sql files in dir in name order, once each, one transaction per file.
func Migrate(ctx context.Context, db *pgxpool.Pool, dir fs.FS) (applied []string, err error) {
	if _, err = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations(
		name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	files, err := fs.Glob(dir, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, name := range files {
		var done bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.schema_migrations WHERE name=$1)`, name).Scan(&done); err != nil {
			return applied, err
		}
		if done {
			continue
		}
		sql, err := fs.ReadFile(dir, name)
		if err != nil {
			return applied, err
		}
		if err := apply(ctx, db, name, string(sql)); err != nil {
			return applied, fmt.Errorf("migration %s: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}

func apply(ctx context.Context, db *pgxpool.Pool, name, sql string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.schema_migrations(name) VALUES($1)`, name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
