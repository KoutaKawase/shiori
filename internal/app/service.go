package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"shiori/internal/db/generated"
	"shiori/internal/domain"
)

// Bookmark はCLI/将来Webで共有する公開型。JSON構造は安定させる。
type Bookmark struct {
	ID        int64    `json:"id"`
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	Comment   string   `json:"comment"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Favorite  bool     `json:"favorite"`
	Tags      []string `json:"tags"`
}

// UpdateInput は部分更新用。nilは「変更なし」。
type UpdateInput struct {
	URL     *string
	Title   *string
	Comment *string
}

// Service はApplication層。トランザクション境界とFTS+タグ合成を担う。
type Service struct {
	db *sql.DB
	q  *generated.Queries
}

func New(db *sql.DB) *Service {
	return &Service{db: db, q: generated.New(db)}
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func toPublic(b generated.Bookmark, tags []string) Bookmark {
	if tags == nil {
		tags = []string{}
	}
	return Bookmark{
		ID: b.ID, URL: b.Url, Title: b.Title, Comment: b.Comment,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
		Favorite: b.Favorite != 0, Tags: tags,
	}
}

func (s *Service) tagsForBookmark(ctx context.Context, q *generated.Queries, bookmarkID int64) ([]string, error) {
	rows, err := q.ListTagsForBookmark(ctx, bookmarkID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out, nil
}

func (s *Service) withTags(ctx context.Context, items []generated.Bookmark) ([]Bookmark, error) {
	out := make([]Bookmark, 0, len(items))
	for _, b := range items {
		tags, err := s.tagsForBookmark(ctx, s.q, b.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toPublic(b, tags))
	}
	return out, nil
}

// Add はブックマークを作成し、タグを付与する。
func (s *Service) Add(ctx context.Context, rawURL, title, comment string, rawTags []string) (Bookmark, error) {
	u, err := domain.ValidateURL(rawURL)
	if err != nil {
		return Bookmark{}, err
	}
	t, err := domain.ValidateTitle(title)
	if err != nil {
		return Bookmark{}, err
	}
	c, err := domain.ValidateComment(comment)
	if err != nil {
		return Bookmark{}, err
	}
	tags, err := domain.NormalizeTags(rawTags)
	if err != nil {
		return Bookmark{}, err
	}
	now := nowStr()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	b, err := q.CreateBookmark(ctx, generated.CreateBookmarkParams{
		Url: u, Title: t, Comment: c, CreatedAt: now, UpdatedAt: now,
		Favorite: 0,
	})
	if err != nil {
		return Bookmark{}, fmt.Errorf("create bookmark: %w", err)
	}
	for _, name := range tags {
		tag, err := q.UpsertTag(ctx, name)
		if err != nil {
			return Bookmark{}, fmt.Errorf("upsert tag: %w", err)
		}
		if err := q.AddBookmarkTag(ctx, generated.AddBookmarkTagParams{
			BookmarkID: b.ID, TagID: tag.ID,
		}); err != nil {
			return Bookmark{}, fmt.Errorf("add bookmark tag: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit: %w", err)
	}
	return Bookmark{
		ID: b.ID, URL: b.Url, Title: b.Title, Comment: b.Comment,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
		Favorite: false, Tags: tags,
	}, nil
}

// Get は1件取得する。
func (s *Service) Get(ctx context.Context, id int64) (Bookmark, error) {
	b, err := s.q.GetBookmark(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("get bookmark: %w", err)
	}
	tags, err := s.tagsForBookmark(ctx, s.q, b.ID)
	if err != nil {
		return Bookmark{}, err
	}
	return toPublic(b, tags), nil
}

// List はタグフィルタ付きで一覧する。tag=""は全件。
func (s *Service) List(ctx context.Context, tag string, limit, offset int) ([]Bookmark, error) {
	limit, offset, err := domain.ValidateLimitOffset(limit, offset)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		items, err := s.q.ListBookmarks(ctx, generated.ListBookmarksParams{
			Limit: int64(limit), Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("list bookmarks: %w", err)
		}
		return s.withTags(ctx, items)
	}
	name, err := domain.NormalizeTag(tag)
	if err != nil {
		return nil, err
	}
	items, err := s.q.ListBookmarksByTag(ctx, generated.ListBookmarksByTagParams{
		Name: name, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list bookmarks by tag: %w", err)
	}
	return s.withTags(ctx, items)
}

// searchFTSSQL と searchFTSWithTagSQL はsqlc外の明示的実装。
// MATCH式全体は束縛パラメータで渡し、文字列結合しない。
const searchFTSSQL = `SELECT b.id, b.url, b.title, b.comment, b.created_at, b.updated_at, b.favorite
FROM bookmarks b
JOIN bookmarks_fts ON bookmarks_fts.rowid = b.id
WHERE bookmarks_fts MATCH ?
ORDER BY rank
LIMIT ? OFFSET ?`

const searchFTSWithTagSQL = `SELECT b.id, b.url, b.title, b.comment, b.created_at, b.updated_at, b.favorite
FROM bookmarks b
JOIN bookmarks_fts ON bookmarks_fts.rowid = b.id
WHERE bookmarks_fts MATCH ?
AND EXISTS (
  SELECT 1 FROM bookmark_tags bt
  JOIN tags t ON t.id = bt.tag_id
  WHERE bt.bookmark_id = b.id AND t.name = ?
)
ORDER BY rank
LIMIT ? OFFSET ?`

func scanBookmarks(rows *sql.Rows) ([]generated.Bookmark, error) {
	defer rows.Close()
	var out []generated.Bookmark
	for rows.Next() {
		var b generated.Bookmark
		if err := rows.Scan(&b.ID, &b.Url, &b.Title, &b.Comment, &b.CreatedAt, &b.UpdatedAt, &b.Favorite); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmarks: %w", err)
	}
	return out, nil
}

// Search はFTS検索とタグフィルタを合成する。queryは必須、tagは任意。
func (s *Service) Search(ctx context.Context, query, tag string, limit, offset int) ([]Bookmark, error) {
	limit, offset, err := domain.ValidateLimitOffset(limit, offset)
	if err != nil {
		return nil, err
	}
	match, err := ToFTSMatchQuery(query)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if tag == "" {
		rows, err = s.db.QueryContext(ctx, searchFTSSQL, match, limit, offset)
	} else {
		name, nerr := domain.NormalizeTag(tag)
		if nerr != nil {
			return nil, nerr
		}
		rows, err = s.db.QueryContext(ctx, searchFTSWithTagSQL, match, name, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("search bookmarks: %w", err)
	}
	items, err := scanBookmarks(rows)
	if err != nil {
		return nil, err
	}
	return s.withTags(ctx, items)
}

// Update は指定フィールドのみ更新する。
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Bookmark, error) {
	cur, err := s.q.GetBookmark(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("get bookmark: %w", err)
	}
	newURL := cur.Url
	if in.URL != nil {
		u, err := domain.ValidateURL(*in.URL)
		if err != nil {
			return Bookmark{}, err
		}
		newURL = u
	}
	newTitle := cur.Title
	if in.Title != nil {
		t, err := domain.ValidateTitle(*in.Title)
		if err != nil {
			return Bookmark{}, err
		}
		newTitle = t
	}
	newComment := cur.Comment
	if in.Comment != nil {
		c, err := domain.ValidateComment(*in.Comment)
		if err != nil {
			return Bookmark{}, err
		}
		newComment = c
	}
	updated, err := s.q.UpdateBookmark(ctx, generated.UpdateBookmarkParams{
		Url: newURL, Title: newTitle, Comment: newComment,
		UpdatedAt: nowStr(), ID: id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("update bookmark: %w", err)
	}
	tags, err := s.tagsForBookmark(ctx, s.q, id)
	if err != nil {
		return Bookmark{}, err
	}
	return toPublic(updated, tags), nil
}

// Delete はブックマークを削除し、孤児タグを掃除する。
func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.q.GetBookmark(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("bookmark %d not found", id)
		}
		return fmt.Errorf("get bookmark: %w", err)
	}
	if err := s.q.DeleteBookmark(ctx, id); err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	// 孤児タグ掃除の失敗は本体操作を覆さないが、エラーを返す。
	if err := s.q.CleanupOrphanTags(ctx); err != nil {
		return fmt.Errorf("cleanup tags: %w", err)
	}
	return nil
}

// TagAdd はタグを付与する。
func (s *Service) TagAdd(ctx context.Context, id int64, rawTag string) (Bookmark, error) {
	name, err := domain.NormalizeTag(rawTag)
	if err != nil {
		return Bookmark{}, err
	}
	if _, err := s.q.GetBookmark(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("get bookmark: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	tag, err := q.UpsertTag(ctx, name)
	if err != nil {
		return Bookmark{}, fmt.Errorf("upsert tag: %w", err)
	}
	if err := q.AddBookmarkTag(ctx, generated.AddBookmarkTagParams{
		BookmarkID: id, TagID: tag.ID,
	}); err != nil {
		return Bookmark{}, fmt.Errorf("add bookmark tag: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit: %w", err)
	}
	return s.Get(ctx, id)
}

// TagRemove はタグを外す。
func (s *Service) TagRemove(ctx context.Context, id int64, rawTag string) (Bookmark, error) {
	name, err := domain.NormalizeTag(rawTag)
	if err != nil {
		return Bookmark{}, err
	}
	if _, err := s.q.GetBookmark(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("get bookmark: %w", err)
	}
	tag, err := s.q.GetTagByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 付いていないタグを外す操作は冪等に成功扱いする。
			return s.Get(ctx, id)
		}
		return Bookmark{}, fmt.Errorf("get tag: %w", err)
	}
	if err := s.q.RemoveBookmarkTag(ctx, generated.RemoveBookmarkTagParams{
		BookmarkID: id, TagID: tag.ID,
	}); err != nil {
		return Bookmark{}, fmt.Errorf("remove bookmark tag: %w", err)
	}
	if err := s.q.CleanupOrphanTags(ctx); err != nil {
		return Bookmark{}, fmt.Errorf("cleanup tags: %w", err)
	}
	return s.Get(ctx, id)
}

// TagCount はタグとそれが付いたブックマーク件数。
type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// ListTags は全タグを件数付きで返す。件数降順・名前昇順。
func (s *Service) ListTags(ctx context.Context) ([]TagCount, error) {
	rows, err := s.q.ListTagsWithCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]TagCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, TagCount{Name: r.Name, Count: r.BookmarkCount})
	}
	return out, nil
}

// ListFavorites はお気に入りのみを一覧する。
func (s *Service) ListFavorites(ctx context.Context, limit, offset int) ([]Bookmark, error) {
	limit, offset, err := domain.ValidateLimitOffset(limit, offset)
	if err != nil {
		return nil, err
	}
	items, err := s.q.ListFavoriteBookmarks(ctx, generated.ListFavoriteBookmarksParams{
		Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	return s.withTags(ctx, items)
}

// SetFavorite はお気に入りを設定・解除する。
func (s *Service) SetFavorite(ctx context.Context, id int64, fav bool) (Bookmark, error) {
	val := int64(0)
	if fav {
		val = 1
	}
	updated, err := s.q.SetFavoriteBookmark(ctx, generated.SetFavoriteBookmarkParams{
		Favorite: val, UpdatedAt: nowStr(), ID: id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d not found", id)
		}
		return Bookmark{}, fmt.Errorf("set favorite: %w", err)
	}
	// UPDATE...RETURNINGは対象行なしでErrNoRowsを返す。
	tags, err := s.tagsForBookmark(ctx, s.q, id)
	if err != nil {
		return Bookmark{}, err
	}
	return toPublic(updated, tags), nil
}
