-- v0.6: Human Review & Decision domain extension.
-- Adds ESCALATE decision type, provenance context, and supersession support.
-- Decisions remain immutable once created; supersession creates a new record.

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS workflow_state TEXT NOT NULL DEFAULT '';

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS rule_evaluation_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS evidence_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS form_submission_id UUID;

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS superseded_by_id UUID;

ALTER TABLE decisions DROP CONSTRAINT IF EXISTS valid_decision;

ALTER TABLE decisions ADD CONSTRAINT valid_decision CHECK (
    decision IN ('APPROVED', 'REJECTED', 'NEEDS_MORE_INFORMATION', 'ESCALATE')
);

CREATE INDEX IF NOT EXISTS idx_decisions_superseded_by ON decisions (superseded_by_id);

CREATE INDEX IF NOT EXISTS idx_decisions_form_submission ON decisions (form_submission_id) WHERE form_submission_id IS NOT NULL;
