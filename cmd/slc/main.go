// slc 는 story-lang 컴파일러 CLI. 판정은 PG 가 하고 여기서는 호출·출력만 한다.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"story-lang/db"
	"story-lang/internal/ir"
	"story-lang/internal/pg"
)

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "db" && args[1] == "migrate" {
		if err := migrate(); err != nil {
			fmt.Fprintln(os.Stderr, "slc:", err)
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
	if args[0] == "load" && len(args) == 2 {
		if err := load(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "slc:", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "slc %s: not implemented\n", args[0])
	os.Exit(2)
}

// load 는 ir.json 을 읽거나, 디렉터리면 build 와 같은 컴파일을 거쳐 적재한다.
// 마이그레이션은 매번 확인한다(이미 적용됐으면 아무것도 하지 않는다).
func load(path string) error {
	var w ir.Work
	if st, err := os.Stat(path); err != nil {
		return err
	} else if st.IsDir() {
		got, diags, err := compile(path)
		if err != nil {
			return err
		}
		if failed, _ := report(os.Stderr, diags, false); failed {
			return fmt.Errorf("%s: compile errors, nothing loaded", path)
		}
		w = *got
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := migrate(); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pg.Load(ctx, pool, &w); err != nil {
		return err
	}
	fmt.Println("loaded", w.Works[0].ID)
	return nil
}

func migrate() error {
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
		fmt.Println("applied", n)
	}
	if err == nil && len(applied) == 0 {
		fmt.Println("up to date")
	}
	return err
}
