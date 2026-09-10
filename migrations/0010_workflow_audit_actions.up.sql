-- v0.3 workflow audit actions: add workflow-specific actions to the audit vocabulary.
ALTER TABLE audit.audit_events
    DROP CONSTRAINT IF EXISTS valid_audit_action;

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
            'workflow.definition_created',
            'workflow.definition_activated',
            'workflow.definition_archived',
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
            'audit.purged',
            'audit.verified'
        )
    );
