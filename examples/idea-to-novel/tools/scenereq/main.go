// scenereq 는 이미 적재한 작품(`dieg load <dir>`)의 장면별 렌더 요청(render_scene JSON, dieg render 가 렌더러에 넘기는 입력)을
// out-dir/<scene>.json 으로 쓰고 sha256 을 찍는다. 수정 전후 요청을 비교해 다시 렌더할 장면을 고르는 데만 쓴다.
// 사용: go run ./examples/idea-to-novel/tools/scenereq <work> <out-dir> <scene...>
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: scenereq <work> <out-dir> <scene...>")
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, s := range os.Args[3:] {
		b, err := pg.RenderScene(ctx, pool, os.Args[1], s)
		if err != nil {
			fmt.Fprintln(os.Stderr, s, err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(os.Args[2], s+".json"), b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s %x\n", s, sha256.Sum256(b))
	}
}
