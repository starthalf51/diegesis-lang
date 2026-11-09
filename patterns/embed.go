// Package patterns embeds the narrative pattern SQL in the binary. One file is one pattern (header `-- pattern:` `-- layer:`
// `-- summary:`, body is a SELECT target_kind, target_id, span_from, span_to, data with $1 = work). The runner is internal/pg/patterns.go.
package patterns

import "embed"

//go:embed *.sql
var FS embed.FS
