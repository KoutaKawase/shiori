package main

import (
	"context"
	"fmt"
	"os"

	"shiori/internal/app"
	"shiori/internal/cli"
	shioridb "shiori/internal/db"
)

// DefaultDBPath は既定のDB配置。SHIORI_DBで上書き可能。
const DefaultDBPath = "data/bookmarks.db"

func dbPath() string {
	if v := os.Getenv("SHIORI_DB"); v != "" {
		return v
	}
	return DefaultDBPath
}

func main() {
	if code, err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "shiori: %v\n", err)
		os.Exit(code)
	} else if code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context, args []string) (int, error) {
	// --db はグローバルに先読みする (サブコマンドのflagと衝突させない)。
	filtered := args[:0:0]
	db := dbPath()
	rest := args
	for len(rest) > 0 {
		a := rest[0]
		if a == "--db" && len(rest) >= 2 {
			db = rest[1]
			rest = rest[2:]
			continue
		}
		filtered = append(filtered, a)
		rest = rest[1:]
	}
	sqlDB, err := shioridb.Open(db)
	if err != nil {
		return cli.ExitError, err
	}
	defer func() { _ = sqlDB.Close() }()
	if err := shioridb.Migrate(ctx, sqlDB); err != nil {
		return cli.ExitError, err
	}
	svc := app.New(sqlDB)
	return cli.Run(ctx, filtered, os.Stdout, os.Stderr, svc), nil
}
