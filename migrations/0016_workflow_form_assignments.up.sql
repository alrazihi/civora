CREATE TABLE workflow_state_form_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workflow_definition_id UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    workflow_state_key VARCHAR(100) NOT NULL,
    form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    form_version_id UUID NOT NULL REFERENCES form_versions(id) ON DELETE CASCADE,
    required BOOLEAN NOT NULL DEFAULT true,
    display_order INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),

    CONSTRAINT unique_workflow_state_form_version
        UNIQUE (tenant_id, workflow_definition_id, workflow_state_key, form_version_id),
    CONSTRAINT unique_workflow_state_display_order
        UNIQUE (tenant_id, workflow_definition_id, workflow_state_key, display_order),
    CONSTRAINT valid_display_order CHECK (display_order >= 0)
);

CREATE INDEX idx_workflow_state_form_assignments_tenant_workflow
    ON workflow_state_form_assignments (tenant_id, workflow_definition_id);

CREATE INDEX idx_workflow_state_form_assignments_state
    ON workflow_state_form_assignments (tenant_id, workflow_definition_id, workflow_state_key);
