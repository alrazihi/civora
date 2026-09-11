-- v0.5: Backfill and constrain workflow_state and closed_at on cases
-- Existing rows that have a workflow_instance_id but NULL workflow_state
-- are updated from the linked workflow instance's current_state.
-- Existing CLOSED cases with NULL closed_at are backfilled from updated_at.
-- Finally, CHECK constraints enforce the invariants going forward.

WITH instance_state AS (
    SELECT ci.case_id, ci.current_state
    FROM workflow_instances ci
    JOIN cases c ON c.id = ci.case_id
    WHERE c.workflow_state IS NULL
)
UPDATE cases
SET workflow_state = instance_state.current_state
FROM instance_state
WHERE cases.id = instance_state.case_id;

UPDATE cases
SET closed_at = updated_at
WHERE status = 'CLOSED' AND closed_at IS NULL;

ALTER TABLE cases
    ADD CONSTRAINT chk_workflow_state_when_instance
    CHECK (workflow_instance_id IS NULL OR workflow_state IS NOT NULL);

ALTER TABLE cases
    ADD CONSTRAINT chk_closed_at_when_closed
    CHECK (status != 'CLOSED' OR closed_at IS NOT NULL);
