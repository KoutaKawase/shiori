package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"shiori/internal/app"
	shioridb "shiori/internal/db"
)

func testServer(t *testing.T) (*httptest.Server, *app.Service) {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := shioridb.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := shioridb.Migrate(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	svc := app.New(sqlDB)
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, svc
}

// noRedirect はリダイレクトを追跡しないクライアントを返す。
func noRedirect() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func postForm(t *testing.T, c *http.Client, target string, v url.Values) *http.Response {
	t.Helper()
	resp, err := c.PostForm(target, v)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp
}

func getBody(t *testing.T, target string) (int, string) {
	t.Helper()
	resp, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}

func TestListEmpty(t *testing.T) {
	srv, _ := testServer(t)
	code, body := getBody(t, srv.URL+"/")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(body, "No bookmarks") {
		t.Fatalf("body=%q", body)
	}
	if strings.Contains(body, "cdn.tailwindcss.com") {
		t.Fatalf("tailwind CDN must not be used: %q", body)
	}
	if !strings.Contains(body, "/static/style.css") {
		t.Fatalf("local stylesheet missing: %q", body)
	}
}

func TestFullCRUDFlow(t *testing.T) {
	srv, _ := testServer(t)
	c := noRedirect()

	// 登録 → PRG。
	resp := postForm(t, c, srv.URL+"/bookmarks", url.Values{
		"url":     {"https://example.com"},
		"title":   {"Example"},
		"comment": {"hello"},
		"tags":    {"dev, go"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create status=%d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/bookmarks/1" {
		t.Fatalf("location=%q", loc)
	}

	// 詳細。
	if code, body := getBody(t, srv.URL+"/bookmarks/1"); code != 200 || !strings.Contains(body, "https://example.com") {
		t.Fatalf("show code=%d body=%q", code, body)
	}

	// 検索。
	if code, body := getBody(t, srv.URL+"/?q=example"); code != 200 || !strings.Contains(body, "https://example.com") {
		t.Fatalf("search code=%d body=%q", code, body)
	}

	// 一覧にコメントが表示される。
	if code, body := getBody(t, srv.URL+"/"); code != 200 || !strings.Contains(body, "hello") {
		t.Fatalf("list comment code=%d body=%q", code, body)
	}

	// タグ絞込。
	if code, body := getBody(t, srv.URL+"/?tag=dev"); code != 200 || !strings.Contains(body, "https://example.com") {
		t.Fatalf("tag filter code=%d body=%q", code, body)
	}

	// 更新 → PRG。
	resp = postForm(t, c, srv.URL+"/bookmarks/1", url.Values{
		"url":     {"https://example.com"},
		"title":   {"New title"},
		"comment": {"hello"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("update status=%d", resp.StatusCode)
	}
	if _, body := getBody(t, srv.URL+"/bookmarks/1"); !strings.Contains(body, "New title") {
		t.Fatalf("updated body=%q", body)
	}

	// タグ追加 → PRG。
	resp = postForm(t, c, srv.URL+"/bookmarks/1/tags", url.Values{"tag": {"docker"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("tag add status=%d", resp.StatusCode)
	}
	if _, body := getBody(t, srv.URL+"/bookmarks/1"); !strings.Contains(body, "docker") {
		t.Fatalf("tag body=%q", body)
	}

	// タグ除去 → PRG。
	resp = postForm(t, c, srv.URL+"/bookmarks/1/tags/remove", url.Values{"tag": {"go"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("tag remove status=%d", resp.StatusCode)
	}

	// 削除 → PRG。
	resp = postForm(t, c, srv.URL+"/bookmarks/1/delete", url.Values{})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("delete status=%d loc=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if code, _ := getBody(t, srv.URL+"/bookmarks/1"); code != http.StatusNotFound {
		t.Fatalf("after delete code=%d", code)
	}
}

func TestValidationAndNotFound(t *testing.T) {
	srv, _ := testServer(t)
	c := noRedirect()

	// 不正URLは422でフォーム再描画。
	resp := postForm(t, c, srv.URL+"/bookmarks", url.Values{"url": {"ftp://x"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	// 存在しないIDは404、不正IDは400。
	if code, _ := getBody(t, srv.URL+"/bookmarks/999"); code != http.StatusNotFound {
		t.Fatalf("code=%d", code)
	}
	if code, _ := getBody(t, srv.URL+"/bookmarks/abc"); code != http.StatusBadRequest {
		t.Fatalf("code=%d", code)
	}
	if code, _ := getBody(t, srv.URL+"/bookmarks/999/edit"); code != http.StatusNotFound {
		t.Fatalf("code=%d", code)
	}
}

func TestHTMLEscaping(t *testing.T) {
	srv, svc := testServer(t)
	if _, err := svc.Add(context.Background(), "https://example.com/x", "<script>alert(1)</script>", "", nil); err != nil {
		t.Fatal(err)
	}
	_, body := getBody(t, srv.URL+"/bookmarks/1")
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("raw script leaked: %q", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("escaped script missing: %q", body)
	}
}

func TestStaticCSS(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("code=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Fatalf("content-type=%q", ct)
	}
}

func TestTagListPage(t *testing.T) {
	srv, svc := testServer(t)
	if _, err := svc.Add(context.Background(), "https://example.com/1", "T", "", []string{"go"}); err != nil {
		t.Fatal(err)
	}
	code, body := getBody(t, srv.URL+"/tags")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(body, "#go") || !strings.Contains(body, "/?tag=go") {
		t.Fatalf("body=%q", body)
	}
}

func TestFavoriteFlow(t *testing.T) {
	srv, svc := testServer(t)
	if _, err := svc.Add(context.Background(), "https://example.com/1", "T1", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(context.Background(), "https://example.com/2", "T2", "", nil); err != nil {
		t.Fatal(err)
	}
	c := noRedirect()

	// お気に入り設定 → 詳細に★表示。
	resp := postForm(t, c, srv.URL+"/bookmarks/1/favorite", url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	_, body := getBody(t, srv.URL+"/bookmarks/1")
	if !strings.Contains(body, "★") {
		t.Fatalf("star missing: %q", body)
	}

	// お気に入り絞込。
	if code, body := getBody(t, srv.URL+"/?fav=1"); code != 200 {
		t.Fatalf("code=%d", code)
	} else if !strings.Contains(body, "T1") || strings.Contains(body, "T2") {
		t.Fatalf("fav filter body=%q", body)
	}

	// 検索との複合は400。
	if code, _ := getBody(t, srv.URL+"/?fav=1&q=x"); code != http.StatusBadRequest {
		t.Fatalf("combo code=%d", code)
	}

	// 解除 → 存在しないIDは404。
	resp = postForm(t, c, srv.URL+"/bookmarks/1/unfavorite", url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	resp = postForm(t, c, srv.URL+"/bookmarks/999/favorite", url.Values{})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
