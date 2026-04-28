CREATE TABLE IF NOT EXISTS ota_update_attempts (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    home_id UUID NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    firmware_version_id UUID NOT NULL REFERENCES firmware_versions(id) ON DELETE RESTRICT,
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    request_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    progress_pct INT NOT NULL DEFAULT 0,
    logs TEXT NOT NULL DEFAULT '',
    error_text TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ota_update_attempts_room_created
    ON ota_update_attempts(room_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ota_update_attempts_device_created
    ON ota_update_attempts(device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ota_update_attempts_active_device
    ON ota_update_attempts(device_id)
    WHERE status IN ('initiated', 'acknowledged', 'downloading', 'verifying', 'updating', 'cancelling');
