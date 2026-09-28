package db

import (
	"context"
	"path/filepath"
	"testing"
)

// マイグレーションが適用され、必要なテーブル・FTS・トリガーが存在すること。
// 開発DBではなく一時DBを使う。
func TestOpenAndMigrate(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "migrate.db")
	sqlDB, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()
	if err := Migrate(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"bookmarks", "tags", "bookmark_tags", "bookmarks_fts"} {
		var name string
		err := sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	var fk int
	if err := sqlDB.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys=%d, want 1", fk)
	}
	// 冪等性。
	if err := Migrate(ctx, sqlDB); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
