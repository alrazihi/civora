-- v0.6: Remove decision_type from workflow_transitions.

ALTER TABLE workflow_transitions
    DROP COLUMN decision_type;
