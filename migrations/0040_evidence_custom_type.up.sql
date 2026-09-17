-- v0.8 Stage 9: Add custom_type to evidence table
ALTER TABLE evidence
    ADD COLUMN IF NOT EXISTS custom_type TEXT;
