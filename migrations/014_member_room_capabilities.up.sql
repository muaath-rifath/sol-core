CREATE TABLE member_room_capabilities (
    home_id UUID NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    can_manage_devices BOOLEAN NOT NULL DEFAULT false,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID REFERENCES users(id),
    PRIMARY KEY (home_id, user_id, room_id)
);
CREATE INDEX idx_member_room_caps_user_home ON member_room_capabilities(user_id, home_id);
