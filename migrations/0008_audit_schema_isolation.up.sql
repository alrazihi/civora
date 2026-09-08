-- v0.2 hardening: move audit_events into its own `audit` schema.
-- Previously audit events shared the same public schema as every other
-- CIVORA entity, so a schema migration that corrupted business data could
-- also destroy the audit trail. This migration relocates the table (and
-- its indexes/constraints) into a dedicated schema, giving the audit trail
-- schema-level isolation from business data. Existing rows are preserved.
CREATE SCHEMA IF NOT EXISTS audit;

CREATE TABLE audit.audit_events (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    resource        TEXT NOT NULL,
    resource_id     TEXT,
    outcome         TEXT NOT NULL,
    request_id      TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    previous_hash   TEXT,
    hash            TEXT NOT NULL
);

-- Copy existing rows across before dropping the old table.
INSERT INTO audit.audit_events
    (id, organization_id, actor_id, action, resource,
     resource_id, outcome, request_id, metadata,
     timestamp, previous_hash, hash)
SELECT
    id, organization_id, actor_id, action, resource,
    resource_id, outcome, request_id, metadata,
    timestamp, previous_hash, hash
FROM audit_events;

CREATE INDEX idx_audit_org_timestamp ON audit.audit_events (organization_id, timestamp DESC);
CREATE INDEX idx_audit_hash ON audit.audit_events (hash);
CREATE INDEX idx_audit_resource_id ON audit.audit_events (resource_id);

ALTER TABLE audit.audit_events
    ADD CONSTRAINT valid_audit_action CHECK (
        action IN (
            'organization.created',
            'organization.updated',
            'auth.register',
            'auth.login',
            'auth.login_failed',
            'auth.success',
            'auth.failed',
            'user.created',
            'user.updated',
            'role.created',
            'role.updated',
            'case.created',
            'case.status_changed',
            'case.transition',
            'case.assigned',
            'case.closed',
            'person.created',
            'person.updated',
            'person.status_updated',
            'eligibility.created',
            'eligibility.assessed',
            'eligibility.result_updated',
            'assessment.created',
            'assessment.completed',
            'decision.created',
            'decision.made',
            'assistance.created',
            'assistance.status_changed',
            'follow_up.scheduled',
            'follow_up.completed',
            'evidence.added',
            'evidence.deleted',
            'audit.purged',
            'audit.verified'
        )
    );

-- Detach the old public table. The CHECK constraint and indexes are
-- dropped with it because they are owned by the table.
DROP TABLE IF EXISTS audit_events;