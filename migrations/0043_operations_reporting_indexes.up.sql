-- v0.9 Stage 2: Add indexes for operational metrics reporting
-- These indexes support the new operations module without modifying workflow behavior.

-- Case volume by workflow state for reporting
CREATE INDEX IF NOT EXISTS idx_cases_org_workflow_state_created
    ON cases (organization_id, workflow_state, created_at DESC);

-- Case volume by service type for reporting
CREATE INDEX IF NOT EXISTS idx_cases_org_service_type_created
    ON cases (organization_id, service_type, created_at DESC);

-- Workflow transition history by occurred_at for trend analysis
CREATE INDEX IF NOT EXISTS idx_workflow_transition_history_org_occurred
    ON workflow_transition_history (organization_id, occurred_at DESC);

-- Review queue by workflow_state for filtering
CREATE INDEX IF NOT EXISTS idx_review_queue_org_workflow_state
    ON review_queue (organization_id, workflow_state, status);

-- AI verified facts by case and verified_at for fact provenance
CREATE INDEX IF NOT EXISTS idx_ai_verified_facts_org_case_verified
    ON ai_verified_facts (organization_id, case_id, verified_at DESC);

-- Audit events by action for audit log filtering
CREATE INDEX IF NOT EXISTS idx_audit_org_action_timestamp
    ON audit.audit_events (organization_id, action, timestamp DESC);

-- Rule evaluations by outcome for rule effectiveness reporting
CREATE INDEX IF NOT EXISTS idx_evaluations_org_outcome_evaluated
    ON rules.evaluations (organization_id, outcome, evaluated_at DESC);
