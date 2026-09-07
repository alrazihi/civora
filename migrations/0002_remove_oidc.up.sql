-- Remove is_oidc_user column and default role assignment logic
ALTER TABLE users DROP COLUMN IF EXISTS is_oidc_user;
