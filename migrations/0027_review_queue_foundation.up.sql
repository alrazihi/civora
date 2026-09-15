-- v0.6: Review queue foundation.
-- Tracks human-review work items derived from workflow state and rule evaluations.
-- Uses PostgreSQL SKIP LOCKED for safe concurrent assignment.

CREATE TABLE review_queue (
    id                   UUID PRIMARY KEY,
    organization_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id              UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    workflow_instance_id  UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    status               TEXT NOT NULL DEFAULT 'PENDING',
    assigned_to          UUID REFERENCES users(id) ON DELETE SET NULL,
    priority             TEXT NOT NULL DEFAULT 'NORMAL',
    workflow_state       TEXT NOT NULL,
    rule_evaluation_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    missing_information  JSONB DEFAULT '[]'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at         TIMESTAMPTZ,
    metadata             JSONB DEFAULT '{}'::jsonb,
    CONSTRAINT valid_review_status CHECK (
        status IN ('PENDING', 'ASSIGNED', 'IN_REVIEW', 'COMPLETED', 'ESCALATED', 'WAITING_INFORMATION')
    )
);

CREATE INDEX idx_review_queue_org ON review_queue (organization_id);
CREATE INDEX idx_review_queue_case ON review_queue (case_id);
CREATE INDEX idx_review_queue_assigned_to ON review_queue (assigned_to) WHERE assigned_to IS NOT NULL;
CREATE INDEX idx_review_queue_status ON review_queue (organization_id, status);
