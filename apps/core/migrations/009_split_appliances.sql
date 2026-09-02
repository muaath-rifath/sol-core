-- +goose Up

CREATE TABLE IF NOT EXISTS appliances (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id   UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    room_id     UUID REFERENCES rooms(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    channel     INT,
    gpio_pin    INT,
    active_low  BOOLEAN DEFAULT false,
    state       JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_appliances_device_id ON appliances(device_id);
CREATE INDEX idx_appliances_room_id ON appliances(room_id);

-- +goose Down

DROP INDEX IF EXISTS idx_appliances_room_id;
DROP INDEX IF EXISTS idx_appliances_device_id;
DROP TABLE IF EXISTS appliances;
