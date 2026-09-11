-- v0.5: Remove workflow_state and closed_at constraints from cases
ALTER TABLE cases DROP CONSTRAINT IF EXISTS chk_workflow_state_when_instance;
ALTER TABLE cases DROP CONSTRAINT IF EXISTS chk_closed_at_when_closed;
