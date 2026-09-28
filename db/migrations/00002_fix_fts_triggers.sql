-- +goose Up
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
CREATE TRIGGER bookmarks_ai AFTER INSERT ON bookmarks BEGIN
    INSERT INTO bookmarks_fts(rowid, url, title, comment)
    VALUES (new.id, new.url, new.title, new.comment);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bookmarks_ad AFTER DELETE ON bookmarks BEGIN
    DELETE FROM bookmarks_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bookmarks_au AFTER UPDATE ON bookmarks BEGIN
    DELETE FROM bookmarks_fts WHERE rowid = old.id;
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
