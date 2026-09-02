-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;
ALTER TABLE appliances ADD COLUMN IF NOT EXISTS embedding vector(1024);

-- +goose Down

ALTER TABLE appliances DROP COLUMN IF EXISTS embedding;
