package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"shiori/internal/app"
	shioridb "shiori/internal/db"
)

func testService(t *testing.T) *app.Service {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := shioridb.Open(filepath.Join(t.TempDir(), "cli.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := shioridb.Migrate(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	return app.New(sqlDB)
}

func run(t *testing.T, svc *app.Service, args ...string) (int, string, string) {
	t.Helper()
	var out, errs bytes.Buffer
	code := Run(context.Background(), args, &out, &errs, svc)
	return code, out.String(), errs.String()
}

func TestAddShowSearchJSON(t *testing.T) {
	svc := testService(t)
	if code, _, errs := run(t, svc, "add", "https://example.com", "--title", "T", "--tag", "Dev"); code != 0 {
		t.Fatalf("add code=%d errs=%s", code, errs)
	}
	code, out, errs := run(t, svc, "show", "1", "--json")
	if code != 0 {
		t.Fatalf("show code=%d errs=%s", code, errs)
	}
	var b app.Bookmark
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if b.ID != 1 || b.URL != "https://example.com" || len(b.Tags) != 1 || b.Tags[0] != "dev" {
		t.Fatalf("bookmark=%+v", b)
	}
	code, out, errs = run(t, svc, "search", "example", "--json")
	if code != 0 {
		t.Fatalf("search code=%d errs=%s", code, errs)
	}
	var items []app.Bookmark
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%v", items)
	}
}

func TestUsageAndErrors(t *testing.T) {
	svc := testService(t)
	if code, _, _ := run(t, svc); code != ExitUsage {
		t.Fatalf("empty args code=%d", code)
	}
	if code, _, _ := run(t, svc, "nope"); code != ExitUsage {
		t.Fatalf("unknown cmd code=%d", code)
	}
	if code, _, _ := run(t, svc, "add", "ftp://x"); code == 0 {
		t.Fatal("expected error")
	}
	if code, _, _ := run(t, svc, "show", "999"); code == 0 {
		t.Fatal("expected not found")
	}
	if code, _, _ := run(t, svc, "update", "1"); code != ExitUsage {
		t.Fatalf("update without flags code=%d", code)
	}
	if code, _, errs := run(t, svc, "add", "https://example.com/json-err", "--json"); code != 0 {
		t.Fatalf("code=%d errs=%s", code, errs)
	} else {
		// JSON stdoutに診断が混ざらないことの簡易確認は呼び出し側で行う。
		_ = errs
	}
}

func TestTagCommands(t *testing.T) {
	svc := testService(t)
	if code, _, errs := run(t, svc, "add", "https://example.com/t"); code != 0 {
		t.Fatalf("add errs=%s", errs)
	}
	if code, out, errs := run(t, svc, "tag", "add", "1", "Go", "--json"); code != 0 {
		t.Fatalf("tag add errs=%s out=%s", errs, out)
	}
	if code, _, errs := run(t, svc, "tag", "remove", "1", "go"); code != 0 {
		t.Fatalf("tag remove errs=%s", errs)
	}
}

func TestTagList(t *testing.T) {
	svc := testService(t)
	if code, _, errs := run(t, svc, "add", "https://example.com/t1", "--tag", "go", "--tag", "dev"); code != 0 {
		t.Fatalf("add errs=%s", errs)
	}
	if code, _, errs := run(t, svc, "add", "https://example.com/t2", "--tag", "go"); code != 0 {
		t.Fatalf("add errs=%s", errs)
	}
	code, out, errs := run(t, svc, "tag", "list")
	if code != 0 {
		t.Fatalf("tag list code=%d errs=%s", code, errs)
	}
	if !strings.Contains(out, "go\t2") || !strings.Contains(out, "dev\t1") {
		t.Fatalf("out=%q", out)
	}
	code, out, errs = run(t, svc, "tag", "list", "--json")
	if code != 0 {
		t.Fatalf("tag list --json code=%d errs=%s", code, errs)
	}
	var items []app.TagCount
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(items) != 2 || items[0].Name != "go" || items[0].Count != 2 {
		t.Fatalf("items=%+v", items)
	}
}

func TestFavoriteCommands(t *testing.T) {
	svc := testService(t)
	if code, _, errs := run(t, svc, "add", "https://example.com/f1"); code != 0 {
		t.Fatalf("add errs=%s", errs)
	}
	if code, _, errs := run(t, svc, "add", "https://example.com/f2"); code != 0 {
		t.Fatalf("add errs=%s", errs)
	}
	if code, out, errs := run(t, svc, "favorite", "1"); code != 0 {
		t.Fatalf("favorite errs=%s out=%s", errs, out)
	} else if !strings.Contains(out, "Favorite: true") {
		t.Fatalf("out=%q", out)
	}
	code, out, errs := run(t, svc, "list", "--favorites", "--json")
	if code != 0 {
		t.Fatalf("list --favorites code=%d errs=%s", code, errs)
	}
	var favs []app.Bookmark
	if err := json.Unmarshal([]byte(out), &favs); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(favs) != 1 || favs[0].ID != 1 || !favs[0].Favorite {
		t.Fatalf("favs=%+v", favs)
	}
	if code, _, errs := run(t, svc, "unfavorite", "1"); code != 0 {
		t.Fatalf("unfavorite errs=%s", errs)
	}
	code, out, errs = run(t, svc, "list", "--favorites", "--json")
	if code != 0 {
		t.Fatalf("code=%d errs=%s", code, errs)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected empty array, got %q", out)
	}
	if code, _, _ := run(t, svc, "favorite", "999"); code == 0 {
		t.Fatal("expected not found")
	}
	if code, _, _ := run(t, svc, "list", "--favorites", "--tag", "go"); code != ExitUsage {
		t.Fatalf("combo code=%d", code)
	}
}
