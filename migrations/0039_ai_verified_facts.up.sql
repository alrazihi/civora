-- v0.8 Stage 7: Human Verification of AI Output
-- Creates verified facts with provenance when AI observations are accepted

CREATE TABLE IF NOT EXISTS ai_verified_facts (
    id                UUID PRIMARY KEY,
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id           UUID,
    observation_id    UUID NOT NULL REFERENCES ai_observations(id) ON DELETE CASCADE,
    observation_type  TEXT NOT NULL,
    value             JSONB NOT NULL,
    original_value    JSONB NOT NULL,
    corrected_value   JSONB,
    provenance        JSONB NOT NULL,
    review_action     TEXT NOT NULL,
    reviewer_id       UUID NOT NULL REFERENCES users(id),
    review_notes      TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    source            TEXT NOT NULL,
    model             JSONB,
    input_hash        TEXT NOT NULL,
    output_hash       TEXT NOT NULL,
    CONSTRAINT fk_verified_fact_observation FOREIGN KEY (observation_id) REFERENCES ai_observations(id),
    CONSTRAINT fk_verified_fact_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_verified_facts_observation
    ON ai_verified_facts (observation_id);

CREATE INDEX IF NOT EXISTS idx_ai_verified_facts_org
    ON ai_verified_facts (organization_id);

CREATE INDEX IF NOT EXISTS idx_ai_verified_facts_org_case
    ON ai_verified_facts (organization_id, case_id);

CREATE INDEX IF NOT EXISTS idx_ai_verified_facts_org_case_verified
    ON ai_verified_facts (organization_id, case_id, verified_at);

CREATE INDEX IF NOT EXISTS idx_ai_verified_facts_observation
    ON ai_verified_facts (observation_id);

ALTER TABLE IF EXISTS ai_verified_facts
    ADD CONSTRAINT valid_verified_fact_review_action CHECK (
        review_action IN ('ACCEPTED', 'REJECTED', 'CORRECTED', 'DISMISSED')
    );

ALTER TABLE IF EXISTS ai_verified_facts
    ADD CONSTRAINT valid_verified_fact_source CHECK (
        source IN ('AI_OBSERVATION', 'HUMAN_CORRECTION', 'HUMAN_ACCEPTED')
    );

ALTER TABLE IF EXISTS ai_verified_facts
    ADD CONSTRAINT valid_verified_fact_type CHECK (
        observation_type IN (
            'SUMMARY', 'ENTITY_EXTRACTION', 'CLASSIFICATION', 'INCONSISTENCY',
            'MISSING_INFORMATION', 'RELEVANT_EVIDENCE', 'INCOMPLETE_DOCUMENTATION'
        )
    );
