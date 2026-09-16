-- v0.7 Stage 2: Evidence verification schema
-- Adds verification status, metadata, source, and person linkage to evidence.
-- Creates immutable verification history table.

-- Step 1: Add verification and metadata columns to evidence table
ALTER TABLE evidence
    ADD COLUMN IF NOT EXISTS person_id UUID REFERENCES people(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS evidence_source TEXT NOT NULL DEFAULT 'UPLOAD',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'UNVERIFIED',
    ADD COLUMN IF NOT EXISTS verified_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS verification_reason TEXT,
    ADD COLUMN IF NOT EXISTS verification_method TEXT;

-- Drop and recreate the evidence_source CHECK constraint
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_evidence_source;
ALTER TABLE evidence ADD CONSTRAINT valid_evidence_source CHECK (
    evidence_source IN ('UPLOAD', 'EXTERNAL_REFERENCE', 'EXTERNAL_API', 'MANUAL')
);

-- Drop and recreate the verification_status CHECK constraint
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_verification_status;
ALTER TABLE evidence ADD CONSTRAINT valid_verification_status CHECK (
    verification_status IN ('UNVERIFIED', 'NEEDS_REVIEW', 'VERIFIED', 'REJECTED')
);

-- Drop and recreate the evidence_type CHECK constraint to allow 'OTHER' for custom types
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_evidence_type;
ALTER TABLE evidence ADD CONSTRAINT valid_evidence_type CHECK (
    type IN ('IDENTITY_DOCUMENT', 'PROOF_OF_RESIDENCE', 'REFERRAL',
             'SUPPORTING_DOCUMENT', 'PHOTOGRAPH', 'STAFF_NOTE', 'OTHER')
);

-- Step 3: Indexes for verification and source filtering
CREATE INDEX IF NOT EXISTS idx_evidence_org_verification ON evidence (organization_id, verification_status);
CREATE INDEX IF NOT EXISTS idx_evidence_org_type ON evidence (organization_id, type);
CREATE INDEX IF NOT EXISTS idx_evidence_person ON evidence (person_id);

-- Step 4: Immutable verification history table
CREATE TABLE IF NOT EXISTS evidence_verification_history (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    evidence_id         UUID NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,
    verification_status TEXT NOT NULL,
    verifier_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    verified_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason              TEXT,
    method              TEXT,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_history_status CHECK (
        verification_status IN ('UNVERIFIED', 'NEEDS_REVIEW', 'VERIFIED', 'REJECTED')
    )
);

CREATE INDEX IF NOT EXISTS idx_evidence_verification_evidence_id ON evidence_verification_history (evidence_id);
CREATE INDEX IF NOT EXISTS idx_evidence_verification_org ON evidence_verification_history (organization_id);
CREATE INDEX IF NOT EXISTS idx_evidence_verification_at ON evidence_verification_history (verified_at);

-- Step 5: Extend audit action vocabulary with evidence verification events
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
