-- v0.2.1: Add optimistic locking version to cases

ALTER TABLE cases ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
