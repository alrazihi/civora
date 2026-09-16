-- v0.7 Stage 2: Rollback evidence verification schema

-- Drop verification history table
DROP TABLE IF EXISTS evidence_verification_history;

-- Drop indexes
DROP INDEX IF EXISTS idx_evidence_org_verification;
DROP INDEX IF EXISTS idx_evidence_org_type;
DROP INDEX IF EXISTS idx_evidence_person;

-- Restore evidence_type CHECK constraint (remove OTHER)
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_evidence_type;
ALTER TABLE evidence ADD CONSTRAINT valid_evidence_type CHECK (
    type IN ('IDENTITY_DOCUMENT', 'PROOF_OF_RESIDENCE', 'REFERRAL',
             'SUPPORTING_DOCUMENT', 'PHOTOGRAPH', 'STAFF_NOTE')
);

-- Restore verification_status CHECK constraint (keep for safety during rollback)
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_verification_status;
ALTER TABLE evidence DROP CONSTRAINT IF EXISTS valid_metadata_update_reason;

-- Restore audit vocabulary to migration 0031 state
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

-- Drop new evidence columns
ALTER TABLE evidence DROP COLUMN IF EXISTS verification_method;
ALTER TABLE evidence DROP COLUMN IF EXISTS verification_reason;
ALTER TABLE evidence DROP COLUMN IF EXISTS verified_at;
ALTER TABLE evidence DROP COLUMN IF EXISTS verified_by;
ALTER TABLE evidence DROP COLUMN IF EXISTS verification_status;
ALTER TABLE evidence DROP COLUMN IF EXISTS metadata;
ALTER TABLE evidence DROP COLUMN IF EXISTS evidence_source;
ALTER TABLE evidence DROP COLUMN IF EXISTS person_id;
