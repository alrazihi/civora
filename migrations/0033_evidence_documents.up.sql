-- v0.7 Stage 3: Evidence document storage
-- Adds evidence_documents table for file upload tracking with internal storage keys,
-- checksums, content types, and file metadata. Storage keys are server-generated
-- and never exposed to clients.

CREATE TABLE IF NOT EXISTS evidence_documents (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    evidence_id         UUID NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,
    file_name           TEXT NOT NULL,
    content_type        TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL,
    checksum            TEXT NOT NULL,
    storage_key         TEXT NOT NULL,
    storage_provider    TEXT NOT NULL DEFAULT 'LOCAL',
    uploaded_by         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    uploaded_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_storage_provider CHECK (
        storage_provider IN ('LOCAL', 'S3', 'GS', 'AZURE')
    ),
    CONSTRAINT valid_content_type CHECK (
        content_type ~ '^application/(json|octet-stream|pdf|zip)$'
        OR content_type ~ '^image/(png|jpeg|gif|webp)$'
        OR content_type ~ '^text/(plain|csv|html)$'
        OR content_type ~ '^application/vnd\.openxmlformats-officedocument\.spreadsheetml\.sheet$'
    )
);

CREATE INDEX IF NOT EXISTS idx_evidence_docs_org ON evidence_documents (organization_id);
CREATE INDEX IF NOT EXISTS idx_evidence_docs_evidence ON evidence_documents (evidence_id);
CREATE INDEX IF NOT EXISTS idx_evidence_docs_checksum ON evidence_documents (checksum);
CREATE UNIQUE INDEX IF NOT EXISTS idx_evidence_docs_storage_key ON evidence_documents (storage_key);

-- Extend audit action vocabulary with document upload/download events
ALTER TABLE IF EXISTS audit.audit_events
    DROP CONSTRAINT IF EXISTS valid_audit_action;

ALTER TABLE IF EXISTS audit.audit_events
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
            'workflow.definition_created',
            'workflow.definition_updated',
            'workflow.definition_activated',
            'workflow.definition_archived',
            'workflow.definition_deleted',
            'workflow.instance_created',
            'workflow.transition',
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
            'evidence.verified',
            'evidence.rejected',
            'evidence.needs_review',
            'evidence.reopened',
            'evidence.metadata_updated',
            'evidence.document_uploaded',
            'evidence.document_downloaded',
            'form.created',
            'form.updated',
            'form.version_created',
            'form.published',
            'form.archived',
            'form.submitted',
            'field.created',
            'field.updated',
            'field.deleted',
            'workflow_form_assignment.created',
            'workflow_form_assignment.updated',
            'workflow_form_assignment.deleted',
            'ruleset.created',
            'ruleset.updated',
            'ruleset.version_created',
            'ruleset.published',
            'ruleset.archived',
            'ruleset.deleted',
            'evaluation.manual',
            'evaluation.automatic',
            'evaluation.error',
            'fact.assembled',
            'ruletemplate.created',
            'ruletemplate.deleted',
            'rule_assignment.created',
            'rule_assignment.updated',
            'rule_assignment.deleted',
            'audit.purged',
            'audit.verified',
            'review.claimed',
            'review.started',
            'review.completed',
            'review.escalated',
            'review.requested_information'
        )
    );
