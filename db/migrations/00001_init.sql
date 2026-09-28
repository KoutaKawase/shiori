-- +goose Up
-- +goose StatementBegin
CREATE TABLE bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE bookmark_tags (
    bookmark_id INTEGER NOT NULL REFERENCES bookmarks(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_bookmark_tags_tag_id ON bookmark_tags(tag_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_bookmarks_created_at ON bookmarks(created_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE VIRTUAL TABLE bookmarks_fts USING fts5(url, title, comment);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bookmarks_ai AFTER INSERT ON bookmarks BEGIN
    INSERT INTO bookmarks_fts(rowid, url, title, comment)
    VALUES (new.id, new.url, new.title, new.comment);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bookmarks_ad AFTER DELETE ON bookmarks BEGIN
    INSERT INTO bookmarks_fts(bookmarks_fts, rowid, url, title, comment)
    VALUES ('delete', old.id, old.url, old.title, old.comment);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bookmarks_au AFTER UPDATE ON bookmarks BEGIN
    INSERT INTO bookmarks_fts(bookmarks_fts, rowid, url, title, comment)
    VALUES ('delete', old.id, old.url, old.title, old.comment);
    INSERT INTO bookmarks_fts(rowid, url, title, comment)
    VALUES (new.id, new.url, new.title, new.comment);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS bookmarks_au;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS bookmarks_ad;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS bookmarks_ai;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS bookmarks_fts;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_bookmarks_created_at;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_bookmark_tags_tag_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS bookmark_tags;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS tags;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS bookmarks;
-- +goose StatementEnd
