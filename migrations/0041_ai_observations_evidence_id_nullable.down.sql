-- v0.8 Stage 9 rollback: Make ai_observations.evidence_id non-nullable
ALTER TABLE ai_observations
    ALTER COLUMN evidence_id SET NOT NULL;
