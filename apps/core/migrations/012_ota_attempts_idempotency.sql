-- +goose Up

ALTER TABLE ota_update_attempts
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ota_update_attempts_idempotency_key
    ON ota_update_attempts(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_ota_update_attempts_idempotency_key;

ALTER TABLE ota_update_attempts
    DROP COLUMN IF EXISTS idempotency_key;
