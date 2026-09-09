-- v0.3 down: drop workflow engine tables
DROP TABLE IF EXISTS workflow_transition_history CASCADE;
DROP TABLE IF EXISTS workflow_instances CASCADE;
DROP TABLE IF EXISTS workflow_transitions CASCADE;
DROP TABLE IF EXISTS workflow_states CASCADE;
DROP TABLE IF EXISTS workflow_definitions CASCADE;

ALTER TABLE cases DROP COLUMN IF EXISTS workflow_instance_id;
