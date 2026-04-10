-- Prevent duplicate home names per owner at the DB level (idempotency safety net)
CREATE UNIQUE INDEX idx_homes_owner_name ON homes(owner_id, lower(name));
