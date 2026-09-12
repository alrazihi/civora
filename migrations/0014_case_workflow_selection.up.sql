-- v0.7: Add workflow_key and workflow_id columns to cases for explicit workflow selection
ALTER TABLE cases ADD COLUMN IF NOT EXISTS workflow_key TEXT;
CREATE INDEX idx_cases_workflow_key ON cases (workflow_key);

ALTER TABLE cases ADD COLUMN IF NOT EXISTS workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL;
CREATE INDEX idx_cases_workflow_id ON cases (workflow_id);
