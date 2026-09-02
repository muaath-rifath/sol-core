-- +goose Up

-- Prevent duplicate pending invites at the DB level (safety net behind service-layer guard)
CREATE UNIQUE INDEX idx_home_invitations_pending_email
    ON home_invitations(home_id, invitee_email)
    WHERE status = 'pending';
