-- v0.2 down: revert public-interest service delivery domain model

DROP TABLE IF EXISTS follow_ups;
DROP TABLE IF EXISTS assistance;
DROP TABLE IF EXISTS decisions;
DROP TABLE IF EXISTS assessments;
DROP TABLE IF EXISTS evidence;
DROP TABLE IF EXISTS eligibilities;

ALTER TABLE cases DROP CONSTRAINT IF EXISTS valid_case_status;
ALTER TABLE cases ADD CONSTRAINT valid_case_status CHECK (
    status IN ('CREATED', 'OPEN', 'IN_REVIEW', 'RESOLVED', 'CLOSED')
);

ALTER TABLE cases DROP COLUMN IF EXISTS person_id;
ALTER TABLE cases DROP COLUMN IF EXISTS service_type;
ALTER TABLE cases DROP COLUMN IF EXISTS priority;

DROP TABLE IF EXISTS people;
