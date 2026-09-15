-- v0.6: Human Review & Decision domain extension (down).

ALTER TABLE decisions DROP CONSTRAINT IF EXISTS valid_decision;

ALTER TABLE decisions DROP COLUMN IF EXISTS superseded_by_id;

ALTER TABLE decisions DROP COLUMN IF EXISTS version;

ALTER TABLE decisions DROP COLUMN IF EXISTS form_submission_id;

ALTER TABLE decisions DROP COLUMN IF EXISTS evidence_ids;

ALTER TABLE decisions DROP COLUMN IF EXISTS rule_evaluation_ids;

ALTER TABLE decisions DROP COLUMN IF EXISTS workflow_state;

ALTER TABLE decisions ADD CONSTRAINT valid_decision CHECK (
    decision IN ('APPROVED', 'REJECTED', 'NEEDS_MORE_INFORMATION')
);

DROP INDEX IF EXISTS idx_decisions_superseded_by;

DROP INDEX IF EXISTS idx_decisions_form_submission;
