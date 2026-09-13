-- v0.5: Revert Rules Engine tables.
DROP TABLE IF EXISTS rules.evaluations CASCADE;
DROP TABLE IF EXISTS rules.rule_sets CASCADE;
DROP SCHEMA IF EXISTS rules CASCADE;
