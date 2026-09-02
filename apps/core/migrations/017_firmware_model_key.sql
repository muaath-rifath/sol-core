-- +goose Up

ALTER TABLE firmware_versions ADD COLUMN IF NOT EXISTS model_key TEXT;

-- +goose Down

ALTER TABLE firmware_versions DROP COLUMN IF EXISTS model_key;
