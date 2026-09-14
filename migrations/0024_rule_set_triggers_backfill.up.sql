-- v0.5: backfill the `triggers` column on rules.rule_sets.
-- Migration 0019 originally shipped without the triggers column; the
-- repository INSERTs/SELECTs it, so any database that ran 0019 before the
-- column existed is broken. This migration adds the column if missing and
-- backfills existing rows with an empty array so all reads/writes succeed.
ALTER TABLE IF EXISTS rules.rule_sets ADD COLUMN IF NOT EXISTS triggers JSONB;
ALTER TABLE IF EXISTS rules.rule_sets ALTER COLUMN triggers SET NOT NULL;
ALTER TABLE IF EXISTS rules.rule_sets ALTER COLUMN triggers SET DEFAULT '[]'::jsonb;
UPDATE rules.rule_sets SET triggers = '[]'::jsonb WHERE triggers IS NULL;