-- v0.6: Decision provenance links (down).

DROP INDEX IF EXISTS idx_decisions_review_queue_entry;

DROP INDEX IF EXISTS idx_workflow_history_decision;

ALTER TABLE decisions DROP COLUMN IF EXISTS review_queue_entry_id;

ALTER TABLE workflow_transition_history DROP COLUMN IF EXISTS decision_id;
