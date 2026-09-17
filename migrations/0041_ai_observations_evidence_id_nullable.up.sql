-- v0.8 Stage 9: Make ai_observations.evidence_id nullable
-- Case-level observations do not have an evidence_id
ALTER TABLE ai_observations
    ALTER COLUMN evidence_id DROP NOT NULL;
