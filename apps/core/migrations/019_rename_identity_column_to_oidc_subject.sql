-- +goose Up
-- +goose StatementBegin

-- Use a provider-neutral name for the stable OIDC `sub` claim. The guards
-- keep this migration valid for both existing and newly initialized databases.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'keycloak_id'
    ) THEN
        ALTER TABLE users RENAME COLUMN keycloak_id TO oidc_subject;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_indexes
        WHERE schemaname = 'public'
          AND indexname = 'idx_users_keycloak_id'
    ) THEN
        ALTER INDEX idx_users_keycloak_id RENAME TO idx_users_oidc_subject;
    END IF;
END $$;

-- +goose StatementEnd
