-- v0.8 Stage 9 rollback: Remove custom_type from evidence table
ALTER TABLE evidence
    DROP COLUMN IF EXISTS custom_type;
