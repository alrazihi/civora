-- v0.4: Remove authoritative workflow_state column from cases
ALTER TABLE cases DROP COLUMN IF EXISTS workflow_state;