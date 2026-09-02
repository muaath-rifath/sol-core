-- +goose Up

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS vector;

-- Rooms
CREATE TABLE IF NOT EXISTS rooms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    floor       INT,
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Devices
CREATE TABLE IF NOT EXISTS devices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    room_id     UUID REFERENCES rooms(id) ON DELETE SET NULL,
    state       JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB DEFAULT '{}',
    firmware_id TEXT,
    online      BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_devices_room_id ON devices(room_id);
CREATE INDEX idx_devices_type ON devices(type);

-- Device telemetry (TimescaleDB hypertable)
CREATE TABLE IF NOT EXISTS device_telemetry (
    device_id   UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ NOT NULL,
    data        JSONB NOT NULL
);

SELECT create_hypertable('device_telemetry', 'timestamp', if_not_exists => TRUE);

-- Retention: keep raw telemetry for 30 days
SELECT add_retention_policy('device_telemetry', INTERVAL '30 days', if_not_exists => TRUE);

-- Automation rules
CREATE TABLE IF NOT EXISTS automation_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    description     TEXT DEFAULT '',
    enabled         BOOLEAN NOT NULL DEFAULT true,
    trigger_config  JSONB NOT NULL,
    conditions      JSONB DEFAULT '[]',
    actions         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_automation_rules_enabled ON automation_rules(enabled);

-- Vector embeddings for semantic search (pgvector)
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS embedding vector(1536);
ALTER TABLE devices          ADD COLUMN IF NOT EXISTS embedding vector(1536);

CREATE INDEX IF NOT EXISTS idx_automation_rules_embedding ON automation_rules USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_devices_embedding          ON devices          USING hnsw (embedding vector_cosine_ops);

-- +goose Down

DROP TABLE IF EXISTS automation_rules;
DROP TABLE IF EXISTS device_telemetry;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS rooms;
