-- v0.2.1: Rollback people external_reference uniqueness

ALTER TABLE people DROP CONSTRAINT IF EXISTS unique_people_org_external_ref;
