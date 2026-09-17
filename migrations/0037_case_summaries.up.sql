-- v0.8 Stage 5: AI Case Summary domain
-- Stores AI-generated case-level summaries with full provenance

CREATE TABLE IF NOT EXISTS case_summaries (
    id                UUID PRIMARY KEY,
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id           UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    summary_type      TEXT NOT NULL,
    source            TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'PENDING_REVIEW',
    model             JSONB,
    content           JSONB NOT NULL,
    confidence        REAL,
    input_hash        TEXT NOT NULL,
    output_hash       TEXT NOT NULL,
    schema_version    TEXT NOT NULL DEFAULT '1.0',
    token_estimate    INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID,
    reviewed_at       TIMESTAMPTZ,
    reviewed_by       UUID,
    review_notes      TEXT
);

CREATE INDEX IF NOT EXISTS idx_case_summaries_org ON case_summaries (organization_id);
CREATE INDEX IF NOT EXISTS idx_case_summaries_case ON case_summaries (organization_id, case_id);
CREATE INDEX IF NOT EXISTS idx_case_summaries_status ON case_summaries (organization_id, status);
CREATE INDEX IF NOT EXISTS idx_case_summaries_type ON case_summaries (organization_id, summary_type);

ALTER TABLE IF EXISTS case_summaries
    ADD CONSTRAINT valid_case_summary_status CHECK (
        status IN ('PENDING_REVIEW', 'ACCEPTED', 'REJECTED')
    );

ALTER TABLE IF EXISTS case_summaries
    ADD CONSTRAINT valid_case_summary_source CHECK (
        source IN ('AI_MODEL', 'HUMAN')
    );

ALTER TABLE IF EXISTS case_summaries
    ADD CONSTRAINT valid_case_summary_type CHECK (
        summary_type IN (
            'FULL_SUMMARY',
            'SITUATION',
            'RELEVANT_INFORMATION',
            'IMPORTANT_EVIDENCE',
            'MISSING_INFORMATION',
            'POTENTIAL_INCONSISTENCIES',
            'RULE_EVALUATION_RESULTS',
            'WORKFLOW_HISTORY',
            'PREVIOUS_ACTIONS',
            'AI_OBSERVATIONS'
        )
    );