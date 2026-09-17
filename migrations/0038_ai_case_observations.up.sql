-- v0.8 Stage 6: AI case-level observations
-- Adds case-centric observation support with detection categories

ALTER TABLE IF EXISTS ai_observations
    ADD COLUMN IF NOT EXISTS case_id UUID;

ALTER TABLE IF EXISTS ai_observations
    ADD COLUMN IF NOT EXISTS statement TEXT;

ALTER TABLE IF EXISTS ai_observations
    ADD COLUMN IF NOT EXISTS source_references JSONB;

ALTER TABLE IF EXISTS ai_observations
    DROP CONSTRAINT IF EXISTS valid_ai_observation_status;

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_status CHECK (
        status IN ('OPEN', 'PENDING_REVIEW', 'ACCEPTED', 'REJECTED', 'CORRECTED', 'DISMISSED')
    );

ALTER TABLE IF EXISTS ai_observations
    DROP CONSTRAINT IF EXISTS valid_ai_observation_type;

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_type CHECK (
        observation_type IN (
            'SUMMARY',
            'ENTITY_EXTRACTION',
            'CLASSIFICATION',
            'INCONSISTENCY',
            'MISSING_INFORMATION',
            'RELEVANT_EVIDENCE',
            'INCOMPLETE_DOCUMENTATION'
        )
    );

CREATE INDEX IF NOT EXISTS idx_ai_observations_case
    ON ai_observations (organization_id, case_id);

CREATE INDEX IF NOT EXISTS idx_ai_observations_org_case_status
    ON ai_observations (organization_id, case_id, status);
