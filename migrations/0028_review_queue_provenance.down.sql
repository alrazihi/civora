-- v0.6: Review queue provenance columns (down).

ALTER TABLE review_queue
    DROP COLUMN IF EXISTS evidence_ids,
    DROP COLUMN IF EXISTS form_submission_id;
