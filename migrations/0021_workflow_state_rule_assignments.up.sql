CREATE TABLE IF NOT EXISTS rules.workflow_state_rule_assignments (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workflow_definition_id UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    workflow_state_key TEXT NOT NULL,
    rule_set_id     UUID NOT NULL REFERENCES rules.rule_sets(id) ON DELETE CASCADE,
    required        BOOLEAN NOT NULL DEFAULT true,
    active          BOOLEAN NOT NULL DEFAULT true,
    display_order   INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_org_id CHECK (organization_id IS NOT NULL),
    CONSTRAINT valid_workflow_state CHECK (workflow_state_key IS NOT NULL AND length(workflow_state_key) > 0),
    CONSTRAINT valid_display_order CHECK (display_order >= 0)
);

CREATE UNIQUE INDEX idx_wsra_unique
    ON rules.workflow_state_rule_assignments (organization_id, workflow_definition_id, workflow_state_key, rule_set_id);

CREATE UNIQUE INDEX idx_wsra_display_order
    ON rules.workflow_state_rule_assignments (organization_id, workflow_definition_id, workflow_state_key, display_order);

CREATE INDEX idx_wsra_workflow_state
    ON rules.workflow_state_rule_assignments (organization_id, workflow_definition_id, workflow_state_key);

CREATE INDEX idx_wsra_rule_set
    ON rules.workflow_state_rule_assignments (organization_id, rule_set_id);
