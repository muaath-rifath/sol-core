CREATE TABLE member_permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_id     UUID NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope_type  TEXT NOT NULL CHECK (scope_type IN ('room','device','appliance')),
    scope_id    UUID NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by  UUID REFERENCES users(id),
    UNIQUE (home_id, user_id, scope_type, scope_id)
);

CREATE INDEX idx_member_permissions_user_home ON member_permissions(user_id, home_id);
CREATE INDEX idx_member_permissions_scope ON member_permissions(scope_type, scope_id);
