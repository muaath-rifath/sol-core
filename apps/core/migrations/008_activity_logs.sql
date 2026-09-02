-- +goose Up

CREATE TABLE IF NOT EXISTS room_activity_logs (
    room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    badge_text      TEXT NOT NULL,
    badge_color     TEXT NOT NULL
);

SELECT create_hypertable('room_activity_logs', 'timestamp', if_not_exists => TRUE);
SELECT add_retention_policy('room_activity_logs', INTERVAL '7 days', if_not_exists => TRUE);

-- +goose Down

SELECT remove_retention_policy('room_activity_logs', if_exists => TRUE);
DROP TABLE IF EXISTS room_activity_logs CASCADE;
