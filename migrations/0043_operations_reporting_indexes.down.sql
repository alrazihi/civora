-- v0.9 Stage 2: Remove reporting indexes
DROP INDEX IF EXISTS idx_cases_org_workflow_state_created;
DROP INDEX IF EXISTS idx_cases_org_service_type_created;
DROP INDEX IF EXISTS idx_workflow_transition_history_org_occurred;
DROP INDEX IF EXISTS idx_review_queue_org_workflow_state;
DROP INDEX IF EXISTS idx_ai_verified_facts_org_case_verified;
DROP INDEX IF EXISTS idx_audit_org_action_timestamp;
DROP INDEX IF EXISTS idx_evaluations_org_outcome_evaluated;
