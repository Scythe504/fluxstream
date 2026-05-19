-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS videos (
    id TEXT PRIMARY KEY,
    magnet_link TEXT NOT NULL,
    file_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted BOOLEAN NOT NULL DEFAULT FALSE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS videos;
-- +goose StatementEnd
