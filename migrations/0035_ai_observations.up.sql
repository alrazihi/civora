-- v0.8 Stage 1: AI observation domain
-- Stores AI-generated observations for evidence review

CREATE TABLE IF NOT EXISTS ai_observations (
    id                UUID PRIMARY KEY,
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    evidence_id       UUID NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,
    observation_type  TEXT NOT NULL,
    observation_source TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'PENDING_REVIEW',
    model             JSONB,
    content           JSONB NOT NULL,
    confidence        REAL,
    input_hash        TEXT NOT NULL,
    output_hash       TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID,
    reviewed_at       TIMESTAMPTZ,
    reviewed_by       UUID,
    review_notes      TEXT
);

CREATE INDEX IF NOT EXISTS idx_ai_observations_org ON ai_observations (organization_id);
CREATE INDEX IF NOT EXISTS idx_ai_observations_evidence ON ai_observations (organization_id, evidence_id);
CREATE INDEX IF NOT EXISTS idx_ai_observations_status ON ai_observations (organization_id, status);

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_status CHECK (
        status IN ('PENDING_REVIEW', 'ACCEPTED', 'REJECTED')
    );

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_source CHECK (
        observation_source IN ('AI_MODEL', 'HUMAN')
    );

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_type CHECK (
        observation_type IN ('SUMMARY', 'ENTITY_EXTRACTION', 'CLASSIFICATION', 'INCONSISTENCY')
    );
