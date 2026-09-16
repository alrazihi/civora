-- v0.6: Decision provenance — link decisions to review queue entries
-- and workflow transitions to decisions.

ALTER TABLE decisions ADD COLUMN IF NOT EXISTS review_queue_entry_id UUID;

ALTER TABLE workflow_transition_history ADD COLUMN IF NOT EXISTS decision_id UUID;

CREATE INDEX IF NOT EXISTS idx_decisions_review_queue_entry ON decisions (review_queue_entry_id) WHERE review_queue_entry_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_workflow_history_decision ON workflow_transition_history (decision_id) WHERE decision_id IS NOT NULL;
