package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"shiori/db/migrations"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// Open はSQLiteを開き、実用的なPRAGMAを設定する。
// SQLiteの書き込みは直列化のため最大接続は1に絞る。
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("db path must not be empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA synchronous=NORMAL;`,
	}
	for _, p := range pragmas {
		if _, err := sqlDB.Exec(p); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return sqlDB, nil
}

// Migrate は埋め込みマイグレーションを適用する。
// CLIのstdout(JSON)を汚さないようgooseのログは抑止する。
func Migrate(ctx context.Context, sqlDB *sql.DB) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
