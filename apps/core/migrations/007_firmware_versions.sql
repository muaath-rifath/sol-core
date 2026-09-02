-- +goose Up

CREATE TABLE firmware_versions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id    TEXT NOT NULL,
    version        TEXT NOT NULL,
    bootloader_key TEXT NOT NULL,
    partition_key  TEXT NOT NULL,
    app_key        TEXT NOT NULL,
    source_key     TEXT,
    size_bytes     BIGINT,
    uploaded_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_firmware_versions_template ON firmware_versions(template_id, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_firmware_versions_template;
DROP TABLE IF EXISTS firmware_versions;
