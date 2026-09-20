-- v0.9.5 Stage 1: Rollback auth session table
DROP INDEX IF EXISTS idx_auth_sessions_refresh_token_hash;
DROP INDEX IF EXISTS idx_auth_sessions_expires_at;
DROP INDEX IF EXISTS idx_auth_sessions_token_family;
DROP INDEX IF EXISTS idx_auth_sessions_user_id;
DROP TABLE IF EXISTS auth_sessions;
