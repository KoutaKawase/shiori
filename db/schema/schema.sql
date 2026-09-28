-- sqlc用スキーマ。db/migrations/00001_init.sql と一致させること。
-- FTS5仮想テーブルとトリガーはマイグレーション側で管理するが、
-- sqlcがクエリを解析できるよう定義を含める。

CREATE TABLE bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    favorite INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE bookmark_tags (
    bookmark_id INTEGER NOT NULL REFERENCES bookmarks(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);

CREATE INDEX idx_bookmark_tags_tag_id ON bookmark_tags(tag_id);
CREATE INDEX idx_bookmarks_created_at ON bookmarks(created_at);

CREATE VIRTUAL TABLE bookmarks_fts USING fts5(url, title, comment);
