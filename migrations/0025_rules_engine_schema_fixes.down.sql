-- Revert rules engine schema fixes.
ALTER TABLE IF EXISTS rules.evaluations DROP COLUMN IF EXISTS matched_rule_id;

ALTER TABLE IF EXISTS rules.evaluations ALTER COLUMN case_id SET NOT NULL;
