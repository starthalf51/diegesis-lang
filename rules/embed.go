// Package rules embeds the lint rule SQL in the binary. One file is one rule (header format in the design note checks.md).
// Genre rules go in subfolders and are added to the embed pattern below.
package rules

import "embed"

//go:embed *.sql
var FS embed.FS
