-- v0.4: Add authoritative workflow_state column to cases
-- The denormalized `status` column on cases mirrors the linked workflow
-- instance's current state for query performance, but the authoritative
-- lifecycle state is the workflow state. This column stores that
-- authoritative state directly so consumers can read it without joining
-- the workflow instance, and so the two fields can be validated for
-- consistency.
ALTER TABLE cases ADD COLUMN IF NOT EXISTS workflow_state TEXT;

CREATE INDEX idx_cases_workflow_state ON cases (workflow_state);