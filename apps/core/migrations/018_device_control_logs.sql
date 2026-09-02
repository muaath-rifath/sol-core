-- +goose Up

CREATE TABLE IF NOT EXISTS device_control_logs (
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    home_id         UUID NOT NULL,
    room_id         UUID NOT NULL,
    device_id       UUID NOT NULL,
    appliance_id    UUID,
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('user', 'esp32', 'automation')),
    actor_id        TEXT NOT NULL,
    action          TEXT NOT NULL,
    command_params  JSONB,
    state_before    JSONB,
    success         BOOLEAN NOT NULL,
    error_message   TEXT
);

SELECT create_hypertable('device_control_logs', 'timestamp', if_not_exists => TRUE);
SELECT add_retention_policy('device_control_logs', INTERVAL '1 year', if_not_exists => TRUE);
CREATE INDEX ON device_control_logs (home_id, timestamp DESC);
CREATE INDEX ON device_control_logs (room_id, timestamp DESC);
CREATE INDEX ON device_control_logs (actor_id, timestamp DESC);

-- +goose Down

DROP TABLE IF EXISTS device_control_logs;
