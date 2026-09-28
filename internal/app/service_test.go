package app

import (
	"context"
	"path/filepath"
	"testing"

	shioridb "shiori/internal/db"
)

func openTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := shioridb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := shioridb.Migrate(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	// マイグレーション冪等性: 2回適用してもエラーにならない。
	if err := shioridb.Migrate(ctx, sqlDB); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	return New(sqlDB)
}

func TestAddGetList(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	b, err := s.Add(ctx, "https://example.com", "title", "comment", []string{"Go", "go"})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == 0 || len(b.Tags) != 1 || b.Tags[0] != "go" {
		t.Fatalf("unexpected add result: %+v", b)
	}
	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.com" {
		t.Fatalf("got %+v", got)
	}
	list, err := s.List(ctx, "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list len=%d", len(list))
	}
}

func TestAddInvalidAndDuplicate(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	if _, err := s.Add(ctx, "ftp://example.com", "", "", nil); err == nil {
		t.Fatal("expected url error")
	}
	if _, err := s.Add(ctx, "https://example.com/dup", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "https://example.com/dup", "", "", nil); err == nil {
		t.Fatal("expected duplicate error")
	}
	if _, err := s.Add(ctx, "https://example.com/x", "", "", []string{"bad/tag"}); err == nil {
		t.Fatal("expected tag error")
	}
}

func TestTagAddRemoveAndFilter(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	b, err := s.Add(ctx, "https://example.com/1", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.TagAdd(ctx, b.ID, "Docker")
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Tags) != 1 || after.Tags[0] != "docker" {
		t.Fatalf("tags=%v", after.Tags)
	}
	filtered, err := s.List(ctx, "docker", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered len=%d", len(filtered))
	}
	after, err = s.TagRemove(ctx, b.ID, "DOCKER")
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Tags) != 0 {
		t.Fatalf("tags=%v", after.Tags)
	}
	// 存在しないタグ外しは冪等成功。
	if _, err := s.TagRemove(ctx, b.ID, "docker"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	b, err := s.Add(ctx, "https://example.com/a", "old", "oldc", nil)
	if err != nil {
		t.Fatal(err)
	}
	newTitle := "new"
	upd, err := s.Update(ctx, b.ID, UpdateInput{Title: &newTitle})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Title != "new" || upd.URL != b.URL {
		t.Fatalf("upd=%+v", upd)
	}
	bad := "ftp://x"
	if _, err := s.Update(ctx, b.ID, UpdateInput{URL: &bad}); err == nil {
		t.Fatal("expected url error")
	}
	if err := s.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, b.ID); err == nil {
		t.Fatal("expected not found")
	}
	if err := s.Delete(ctx, b.ID); err == nil {
		t.Fatal("expected not found on second delete")
	}
}

func TestSearchFTS(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	if _, err := s.Add(ctx, "https://example.com/docker", "Docker intro", "containers", []string{"dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "https://example.com/go", "Go language", "concurrency", []string{"dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "https://example.com/other", "Other", "nothing", []string{"misc"}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Search(ctx, "docker", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("search docker len=%d %+v", len(res), res)
	}
	// タグ合成。
	res, err = s.Search(ctx, "language", "dev", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("search+tag len=%d", len(res))
	}
	res, err = s.Search(ctx, "language", "misc", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0, got %d", len(res))
	}
	// 特殊文字を含む入力でもSQLエラーにせず安全に扱う。
	res, err = s.Search(ctx, `" OR 1=1 --`, "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatalf("injection-like query should match 0, got %d", len(res))
	}
	if _, err := s.Search(ctx, "   ", "", 50, 0); err == nil {
		t.Fatal("expected empty query error")
	}
}

func TestSearchSyncOnUpdateDelete(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	b, err := s.Add(ctx, "https://example.com/sync", "hello", "world", nil)
	if err != nil {
		t.Fatal(err)
	}
	newComment := "goodbye world"
	if _, err := s.Update(ctx, b.ID, UpdateInput{Comment: &newComment}); err != nil {
		t.Fatal(err)
	}
	// titleで検索できること(トリガー同期の確認)。
	res, err := s.Search(ctx, "hello", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("after update search len=%d", len(res))
	}
	if err := s.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	res, err = s.Search(ctx, "hello", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatalf("after delete search len=%d", len(res))
	}
}

func TestListTags(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	if _, err := s.Add(ctx, "https://example.com/1", "", "", []string{"go", "dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "https://example.com/2", "", "", []string{"go"}); err != nil {
		t.Fatal(err)
	}
	tags, err := s.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("tags=%+v", tags)
	}
	// 件数降順: go(2), dev(1)。
	if tags[0].Name != "go" || tags[0].Count != 2 {
		t.Fatalf("tags=%+v", tags)
	}
	if tags[1].Name != "dev" || tags[1].Count != 1 {
		t.Fatalf("tags=%+v", tags)
	}
	// タグを外すと件数に反映される。
	b, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = b
	if _, err := s.TagRemove(ctx, 1, "dev"); err != nil {
		t.Fatal(err)
	}
	tags, err = s.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// devは孤児掃除で消えるためgoのみ残る。
	if len(tags) != 1 || tags[0].Name != "go" {
		t.Fatalf("tags=%+v", tags)
	}
}

func TestSetFavoriteAndListFavorites(t *testing.T) {
	ctx := context.Background()
	s := openTestService(t)
	b1, err := s.Add(ctx, "https://example.com/1", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "https://example.com/2", "", "", nil); err != nil {
		t.Fatal(err)
	}
	fav, err := s.SetFavorite(ctx, b1.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !fav.Favorite {
		t.Fatalf("fav=%+v", fav)
	}
	got, err := s.Get(ctx, b1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Favorite {
		t.Fatalf("got=%+v", got)
	}
	list, err := s.ListFavorites(ctx, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != b1.ID {
		t.Fatalf("list=%+v", list)
	}
	unfav, err := s.SetFavorite(ctx, b1.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if unfav.Favorite {
		t.Fatalf("unfav=%+v", unfav)
	}
	if _, err := s.SetFavorite(ctx, 999, true); err == nil {
		t.Fatal("expected not found")
	}
}
