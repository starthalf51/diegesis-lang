// Package db 는 마이그레이션 SQL을 바이너리에 내장한다. sil 와 테스트가 같은 파일을 쓴다.
package db

import "embed"

//go:embed migrations/*.sql
var FS embed.FS
