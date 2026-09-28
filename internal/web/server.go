package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"shiori/internal/app"
)

//go:embed templates/*.html static/style.css
var assetsFS embed.FS

// Server はHTTPアダプタ。業務ロジックは持たずapp.Serviceに委譲する。
type Server struct {
	svc  *app.Service
	tmpl *template.Template
}

// NewHandler はテスト可能なhttp.Handlerを組み立てる。
func NewHandler(svc *app.Service) (http.Handler, error) {
	tmpl, err := template.ParseFS(assetsFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	s := &Server{svc: svc, tmpl: tmpl}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleList)
	mux.HandleFunc("GET /tags", s.handleTagList)
	mux.HandleFunc("GET /bookmarks/new", s.handleNew)
	mux.HandleFunc("POST /bookmarks", s.handleCreate)
	mux.HandleFunc("GET /bookmarks/{id}", s.handleShow)
	mux.HandleFunc("GET /bookmarks/{id}/edit", s.handleEdit)
	mux.HandleFunc("POST /bookmarks/{id}", s.handleUpdate)
	mux.HandleFunc("POST /bookmarks/{id}/delete", s.handleDelete)
	mux.HandleFunc("POST /bookmarks/{id}/tags", s.handleTagAdd)
	mux.HandleFunc("POST /bookmarks/{id}/tags/remove", s.handleTagRemove)
	mux.HandleFunc("POST /bookmarks/{id}/favorite", s.handleFavoriteSet)
	mux.HandleFunc("POST /bookmarks/{id}/unfavorite", s.handleFavoriteUnset)
	staticFS, err := fs.Sub(assetsFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	return mux, nil
}

// Serve は指定アドレスでリッスンする。ctx完了時はgracefulに停止する。
func Serve(ctx context.Context, addr string, svc *app.Service) error {
	h, err := NewHandler(svc)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: addr, Handler: h}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}
