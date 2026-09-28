-- name: CreateBookmark :one
INSERT INTO bookmarks (url, title, comment, created_at, updated_at, favorite)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, url, title, comment, created_at, updated_at, favorite;

-- name: GetBookmark :one
SELECT id, url, title, comment, created_at, updated_at, favorite
FROM bookmarks
WHERE id = ?;

-- name: GetBookmarkByURL :one
SELECT id, url, title, comment, created_at, updated_at, favorite
FROM bookmarks
WHERE url = ?;

-- name: ListBookmarks :many
SELECT id, url, title, comment, created_at, updated_at, favorite
FROM bookmarks
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: ListFavoriteBookmarks :many
SELECT id, url, title, comment, created_at, updated_at, favorite
FROM bookmarks
WHERE favorite = 1
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: UpdateBookmark :one
UPDATE bookmarks
SET url = ?, title = ?, comment = ?, updated_at = ?
WHERE id = ?
RETURNING id, url, title, comment, created_at, updated_at, favorite;

-- name: SetFavoriteBookmark :one
UPDATE bookmarks
SET favorite = ?, updated_at = ?
WHERE id = ?
RETURNING id, url, title, comment, created_at, updated_at, favorite;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks WHERE id = ?;
