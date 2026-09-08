-- v0.2 hardening: revert the audit action vocabulary constraint.
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS valid_audit_action;