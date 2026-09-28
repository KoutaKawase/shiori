-- +goose Up
-- +goose StatementBegin
ALTER TABLE bookmarks ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bookmarks DROP COLUMN favorite;
-- +goose StatementEnd
