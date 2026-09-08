-- v0.2 hardening: revert audit_events schema isolation.
-- The CHECK constraint and indexes are dropped with the table because they
-- are owned by it.
DROP SCHEMA IF EXISTS audit;