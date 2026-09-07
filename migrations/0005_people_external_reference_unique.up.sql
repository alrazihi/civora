-- v0.2.1: Ensure external_reference uniqueness per organization

ALTER TABLE people ADD CONSTRAINT unique_people_org_external_ref UNIQUE (organization_id, external_reference);
