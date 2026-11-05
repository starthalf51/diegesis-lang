// slc 는 story-lang 컴파일러 CLI. 판정은 PG 가 하고 여기서는 호출·출력만 한다.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"story-lang/db"
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
	fmt.Fprintf(os.Stderr, "slc %s: not implemented\n", args[0])
	os.Exit(2)
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
