// dieg 는 Diegesis 컴파일러 CLI. 판정은 PG 가 하고 여기서는 호출·출력만 한다.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"

	"diegesis/db"
	"diegesis/internal/ir"
	"diegesis/internal/pg"
)

// version 은 SemVer. 릴리스할 때 git 태그(v0.2.0)와 같이 올린다. 1.0 전까지 문법은 바뀔 수 있다.
const version = "0.5.0"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Println("dieg", version)
		return
	}
	if len(args) >= 2 && args[0] == "db" && args[1] == "migrate" {
		if err := migrate(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 0 {
		os.Exit(usage())
	}
	if args[0] == "check" || args[0] == "build" {
		os.Exit(cmdCompile(args))
	}
	if args[0] == "test" {
		os.Exit(cmdTest(args[1:]))
	}
	if args[0] == "render" {
		os.Exit(cmdRender(args[1:]))
	}
	if args[0] == "render-check" {
		os.Exit(cmdRenderCheck(args[1:]))
	}
	if args[0] == "write" {
		os.Exit(cmdWrite(args[1:]))
	}
	if args[0] == "continue" {
		os.Exit(cmdContinue(args[1:]))
	}
	if args[0] == "revise" {
		os.Exit(cmdRevise(args[1:]))
	}
	if args[0] == "lint" {
		os.Exit(cmdLint(args[1:]))
	}
	if args[0] == "patterns" {
		os.Exit(cmdPatterns(args[1:]))
	}
	if args[0] == "stats" {
		os.Exit(cmdStats(args[1:]))
	}
	if args[0] == "load" && len(args) == 2 {
		if _, err := load(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "dieg %s: not implemented\n", args[0])
	os.Exit(2)
}

// load 는 ir.json 을 읽거나, 디렉터리면 build 와 같은 컴파일을 거쳐 적재하고 작품 ID 를 돌려준다.
// 마이그레이션은 매번 확인한다(이미 적용됐으면 아무것도 하지 않는다). 진행 메시지는 out 으로.
func load(path string, out io.Writer) (string, error) {
	var w ir.Work
	if st, err := os.Stat(path); err != nil {
		return "", err
	} else if st.IsDir() {
		got, diags, err := compile(path)
		if err != nil {
			return "", err
		}
		if failed, _ := report(os.Stderr, diags, false); failed {
			return "", fmt.Errorf("%s: compile errors, nothing loaded", path)
		}
		w = *got
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := migrate(out); err != nil {
		return "", err
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer pool.Close()
	if err := pg.Load(ctx, pool, &w); err != nil {
		return "", err
	}
	fmt.Fprintln(out, "loaded", w.Works[0].ID)
	return w.Works[0].ID, nil
}

func migrate(out io.Writer) error {
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	dir, err := fs.Sub(db.FS, "migrations")
	if err != nil {
		return err
	}
	applied, err := pg.Migrate(ctx, pool, dir)
	for _, n := range applied {
		fmt.Fprintln(out, "applied", n)
	}
	if err == nil && len(applied) == 0 {
		fmt.Fprintln(out, "up to date")
	}
	return err
}
