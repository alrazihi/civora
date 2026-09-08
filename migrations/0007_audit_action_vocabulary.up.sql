-- v0.2 hardening: constrain audit action vocabulary.
-- Without this constraint, the audit_events.action column can hold arbitrary
-- strings, which weakens the audit trail's usefulness for downstream
-- verification and reporting. The CHECK constraint enforces a known
-- vocabulary of audit actions at the database level.
ALTER TABLE audit_events
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