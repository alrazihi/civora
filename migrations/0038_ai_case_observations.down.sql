DROP INDEX IF EXISTS idx_ai_observations_org_case_status;
DROP INDEX IF EXISTS idx_ai_observations_case;

ALTER TABLE IF EXISTS ai_observations
    DROP CONSTRAINT IF EXISTS valid_ai_observation_type;

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_type CHECK (
        observation_type IN ('SUMMARY', 'ENTITY_EXTRACTION', 'CLASSIFICATION', 'INCONSISTENCY')
    );

ALTER TABLE IF EXISTS ai_observations
    DROP CONSTRAINT IF EXISTS valid_ai_observation_status;

ALTER TABLE IF EXISTS ai_observations
    ADD CONSTRAINT valid_ai_observation_status CHECK (
        status IN ('PENDING_REVIEW', 'ACCEPTED', 'REJECTED')
    );

ALTER TABLE IF EXISTS ai_observations
    DROP COLUMN IF EXISTS source_references;

ALTER TABLE IF EXISTS ai_observations
    DROP COLUMN IF EXISTS statement;

ALTER TABLE IF EXISTS ai_observations
    DROP COLUMN IF EXISTS case_id;
