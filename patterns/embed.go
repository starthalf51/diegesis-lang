// Package patterns 는 서사 패턴 SQL 을 바이너리에 내장한다. 파일 하나가 패턴 하나다(헤더 `-- pattern:` `-- layer:`
// `-- summary:`, 본문은 $1 = 작품인 SELECT target_kind, target_id, span_from, span_to, data). 러너는 internal/pg/patterns.go.
package patterns

import "embed"

//go:embed *.sql
var FS embed.FS
