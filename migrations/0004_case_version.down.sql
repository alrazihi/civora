-- v0.2.1: Rollback case version column

ALTER TABLE cases DROP COLUMN IF EXISTS version;
