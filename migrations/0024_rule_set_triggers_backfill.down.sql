-- v0.5: revert the triggers backfill.
ALTER TABLE IF EXISTS rules.rule_sets DROP COLUMN IF EXISTS triggers;