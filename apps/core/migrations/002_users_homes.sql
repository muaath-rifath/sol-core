-- +goose Up

-- Users (synced from the OIDC issuer on first token verification)
CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_subject TEXT NOT NULL UNIQUE,
    email        TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_oidc_subject ON users(oidc_subject);
CREATE INDEX idx_users_email ON users(email);

-- Homes
CREATE TABLE homes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_homes_owner_id ON homes(owner_id);

-- Home members (owner is also a member with role='owner')
CREATE TABLE home_members (
    home_id    UUID NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member')),
    invited_by UUID REFERENCES users(id),
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (home_id, user_id)
);
CREATE INDEX idx_home_members_user_id ON home_members(user_id);

-- Home invitations (token-based, email-addressed)
CREATE TABLE home_invitations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_id       UUID NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    inviter_id    UUID NOT NULL REFERENCES users(id),
    invitee_email TEXT NOT NULL,
    token         TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','declined','expired')),
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_home_invitations_token ON home_invitations(token);
CREATE INDEX idx_home_invitations_email ON home_invitations(invitee_email);

-- Scope rooms and automations to homes (nullable for backward compat)
ALTER TABLE rooms ADD COLUMN home_id UUID REFERENCES homes(id) ON DELETE SET NULL;
ALTER TABLE automation_rules ADD COLUMN home_id UUID REFERENCES homes(id) ON DELETE SET NULL;

-- +goose Down

ALTER TABLE automation_rules DROP COLUMN IF EXISTS home_id;
ALTER TABLE rooms DROP COLUMN IF EXISTS home_id;
DROP TABLE IF EXISTS home_invitations;
DROP TABLE IF EXISTS home_members;
DROP TABLE IF EXISTS homes;
DROP TABLE IF EXISTS users;
