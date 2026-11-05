// Package rules 는 린트 규칙 SQL 을 바이너리에 내장한다. 파일 하나가 규칙 하나다(헤더 형식은 vault checks.md).
// 장르 규칙은 하위 폴더에 두고 아래 embed 패턴에 추가한다.
package rules

import "embed"

//go:embed *.sql
var FS embed.FS
