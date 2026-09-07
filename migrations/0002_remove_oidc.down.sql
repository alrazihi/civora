-- Re-add is_oidc_user column (for rollback only)
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_oidc_user BOOLEAN NOT NULL DEFAULT FALSE;
