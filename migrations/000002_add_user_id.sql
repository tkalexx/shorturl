-- +goose Up
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls (user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_urls_user_id;
ALTER TABLE urls DROP COLUMN IF EXISTS user_id;
