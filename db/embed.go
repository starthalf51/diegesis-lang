// Package db embeds the migration SQL in the binary. dieg and the tests use the same files.
package db

import "embed"

//go:embed migrations/*.sql
var FS embed.FS
