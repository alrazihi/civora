-- v0.6: Add decision_type to workflow_transitions for human decision integration.

ALTER TABLE workflow_transitions
    ADD COLUMN decision_type TEXT;
