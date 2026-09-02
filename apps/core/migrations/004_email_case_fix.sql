-- +goose Up

-- Functional index to support case-insensitive email lookups efficiently
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users(LOWER(email));
