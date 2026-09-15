-- v0.6: Review queue provenance columns.

ALTER TABLE review_queue
    ADD COLUMN evidence_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN form_submission_id UUID;
