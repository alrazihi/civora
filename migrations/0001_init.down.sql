DROP INDEX IF EXISTS idx_audit_hash;
DROP INDEX IF EXISTS idx_audit_org_timestamp;
DROP INDEX IF EXISTS idx_cases_org_status;
DROP INDEX IF EXISTS idx_users_org_email;

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS cases;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS schema_migrations;
