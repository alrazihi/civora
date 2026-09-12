-- v0.7: Downgrade - remove workflow_key and workflow_id columns from cases
DROP INDEX IF EXISTS idx_cases_workflow_key;
DROP INDEX IF EXISTS idx_cases_workflow_id;
ALTER TABLE cases DROP COLUMN IF EXISTS workflow_key;
ALTER TABLE cases DROP COLUMN IF EXISTS workflow_id;
