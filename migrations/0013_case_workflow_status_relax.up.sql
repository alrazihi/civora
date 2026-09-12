-- v0.6: Allow case status to mirror configurable workflow state keys
ALTER TABLE cases DROP CONSTRAINT IF EXISTS valid_case_status;
ALTER TABLE cases
    ADD CONSTRAINT chk_case_status_nonempty CHECK (btrim(status) <> '');
