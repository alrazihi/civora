ALTER TABLE IF EXISTS rules.evaluations ADD COLUMN IF NOT EXISTS rule_set_version INT;
UPDATE rules.evaluations SET rule_set_version = (SELECT version FROM rules.rule_sets WHERE id = evaluations.rule_set_id) WHERE rule_set_version IS NULL;
ALTER TABLE IF EXISTS rules.evaluations ALTER COLUMN rule_set_version SET NOT NULL;
