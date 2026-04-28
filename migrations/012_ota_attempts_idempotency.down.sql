DROP INDEX IF EXISTS idx_ota_update_attempts_idempotency_key;

ALTER TABLE ota_update_attempts
    DROP COLUMN IF EXISTS idempotency_key;
