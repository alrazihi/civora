-- v0.2 hardening: enforce one decision per service request version.
-- Without this constraint, concurrent calls to POST /decisions for the same
-- service request can both succeed, producing duplicate decisions and
-- bypassing the one-decision-per-case lifecycle rule.
-- Superseding decisions are supported via versioning; the unique key
-- includes version to allow multiple versions per service request.
ALTER TABLE decisions ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX idx_decisions_service_request_version_unique
    ON decisions (organization_id, service_request_id, version);
