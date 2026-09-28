package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"shiori/internal/app"
	"shiori/internal/domain"
)

type listData struct {
	Query     string
	Tag       string
	FavOnly   bool
	Limit     int
	Offset    int
	Bookmarks []app.Bookmark
	HasPrev   bool
	HasNext   bool
	PrevOffset int
	NextOffset int
}

type showData struct {
	Bookmark app.Bookmark
	Error    string
}

type formData struct {
	IsNew   bool
	Action  string
	URL     string
	Title   string
	Comment string
	Tags    string
	Error   string
}

func parseID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid ID %q", raw)
	}
	return id, nil
}

func parsePaging(r *http.Request) (limit, offset int, err error) {
	limit = domain.DefaultLimit
	offset = 0
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return 0, 0, fmt.Errorf("invalid limit %q", v)
		}
		limit = n
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 0 {
			return 0, 0, fmt.Errorf("invalid offset %q", v)
		}
		offset = n
	}
	return domain.ValidateLimitOffset(limit, offset)
}

// splitTags はカンマ・空白区切りのタグ入力を分割する。
// 正規化・検証はapp.Service(ドメイン層)に任せる。
func splitTags(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	return strings.Fields(s)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tag := r.URL.Query().Get("tag")
	favOnly := r.URL.Query().Get("fav") == "1"
	limit, offset, err := parsePaging(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if favOnly && (strings.TrimSpace(query) != "" || tag != "") {
		http.Error(w, "fav=1 cannot be combined with q or tag", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	var items []app.Bookmark
	switch {
	case favOnly:
		items, err = s.svc.ListFavorites(ctx, limit, offset)
	case strings.TrimSpace(query) == "":
		items, err = s.svc.List(ctx, tag, limit, offset)
	default:
		items, err = s.svc.Search(ctx, query, tag, limit, offset)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if items == nil {
		items = []app.Bookmark{}
	}
	prev := offset - limit
	if prev < 0 {
		prev = 0
	}
	s.render(w, "list", listData{
		Query: query, Tag: tag, FavOnly: favOnly, Limit: limit, Offset: offset,
		Bookmarks: items,
		HasPrev: offset > 0, HasNext: len(items) == limit,
		PrevOffset: prev, NextOffset: offset + limit,
	})
}

func (s *Server) handleShow(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, err := s.svc.Get(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "show", showData{Bookmark: b})
}

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, "form", formData{IsNew: true, Action: "/bookmarks"})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	url := r.FormValue("url")
	title := r.FormValue("title")
	comment := r.FormValue("comment")
	tags := r.FormValue("tags")
	b, err := s.svc.Add(r.Context(), url, title, comment, splitTags(tags))
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "form", formData{
			IsNew: true, Action: "/bookmarks",
			URL: url, Title: title, Comment: comment, Tags: tags,
			Error: err.Error(),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d", b.ID), http.StatusSeeOther)
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, err := s.svc.Get(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "form", formData{
		IsNew: false, Action: fmt.Sprintf("/bookmarks/%d", b.ID),
		URL: b.URL, Title: b.Title, Comment: b.Comment,
	})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	url := r.FormValue("url")
	title := r.FormValue("title")
	comment := r.FormValue("comment")
	b, err := s.svc.Update(r.Context(), id, app.UpdateInput{
		URL: &url, Title: &title, Comment: &comment,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "form", formData{
			IsNew: false, Action: fmt.Sprintf("/bookmarks/%d", id),
			URL: url, Title: title, Comment: comment,
			Error: err.Error(),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d", b.ID), http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.svc.Delete(r.Context(), id); err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) showWithError(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	b, err := s.svc.Get(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, "show", showData{Bookmark: b, Error: msg})
}

func (s *Server) handleTagAdd(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if _, err := s.svc.TagAdd(r.Context(), id, r.FormValue("tag")); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		s.showWithError(w, r, id, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d", id), http.StatusSeeOther)
}

func (s *Server) handleTagRemove(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if _, err := s.svc.TagRemove(r.Context(), id, r.FormValue("tag")); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		s.showWithError(w, r, id, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d", id), http.StatusSeeOther)
}

type tagsData struct {
	Tags []app.TagCount
}

func (s *Server) handleTagList(w http.ResponseWriter, r *http.Request) {
	tags, err := s.svc.ListTags(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []app.TagCount{}
	}
	s.render(w, "tags", tagsData{Tags: tags})
}

func (s *Server) handleFavorite(w http.ResponseWriter, r *http.Request, fav bool) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.svc.SetFavorite(r.Context(), id, fav); err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d", id), http.StatusSeeOther)
}

func (s *Server) handleFavoriteSet(w http.ResponseWriter, r *http.Request) {
	s.handleFavorite(w, r, true)
}

func (s *Server) handleFavoriteUnset(w http.ResponseWriter, r *http.Request) {
	s.handleFavorite(w, r, false)
}
