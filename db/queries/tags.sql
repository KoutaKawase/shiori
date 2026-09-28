-- name: UpsertTag :one
INSERT INTO tags (name) VALUES (?)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING id, name;

-- name: GetTagByName :one
SELECT id, name FROM tags WHERE name = ?;

-- name: AddBookmarkTag :exec
INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: RemoveBookmarkTag :exec
DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?;

-- name: ListTagsForBookmark :many
SELECT t.id, t.name
FROM tags t
JOIN bookmark_tags bt ON bt.tag_id = t.id
WHERE bt.bookmark_id = ?
ORDER BY t.name;

-- name: ListBookmarksByTag :many
SELECT b.id, b.url, b.title, b.comment, b.created_at, b.updated_at, b.favorite
FROM bookmarks b
JOIN bookmark_tags bt ON bt.bookmark_id = b.id
JOIN tags t ON t.id = bt.tag_id
WHERE t.name = ?
ORDER BY b.id DESC
LIMIT ? OFFSET ?;

-- name: CleanupOrphanTags :exec
DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM bookmark_tags);

-- name: ListTagsWithCounts :many
SELECT t.name, COUNT(bt.bookmark_id) AS bookmark_count
FROM tags t
LEFT JOIN bookmark_tags bt ON bt.tag_id = t.id
GROUP BY t.id, t.name
ORDER BY bookmark_count DESC, t.name;
