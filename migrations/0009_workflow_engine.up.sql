-- v0.3: Configurable Workflow Definition and Execution Engine
-- Workflow Definitions (tenant-scoped, versioned, status-controlled)
CREATE TABLE workflow_definitions (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key                 TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    version             INTEGER NOT NULL,
    status              TEXT NOT NULL,
    initial_state       TEXT NOT NULL,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, key, version)
);

CREATE INDEX idx_workflow_definitions_org ON workflow_definitions (organization_id);
CREATE INDEX idx_workflow_definitions_org_key ON workflow_definitions (organization_id, key);

ALTER TABLE workflow_definitions ADD CONSTRAINT valid_workflow_status CHECK (
    status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')
);

-- Workflow States (belong to a specific definition version)
CREATE TABLE workflow_states (
    id                      UUID PRIMARY KEY,
    workflow_definition_id  UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    organization_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key                     TEXT NOT NULL,
    name                    TEXT NOT NULL,
    description             TEXT,
    category                TEXT,
    terminal                BOOLEAN NOT NULL DEFAULT false,
    display_order           INTEGER NOT NULL DEFAULT 0,
    responsible_role        TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_states_def ON workflow_states (workflow_definition_id);

-- Workflow Transitions (explicit edges between states)
CREATE TABLE workflow_transitions (
    id                      UUID PRIMARY KEY,
    workflow_definition_id  UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    organization_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key                     TEXT NOT NULL,
    name                    TEXT NOT NULL,
    from_state              TEXT NOT NULL,
    to_state                TEXT NOT NULL,
    description             TEXT,
    conditions              JSONB NOT NULL DEFAULT '[]',
    allowed_roles           JSONB NOT NULL DEFAULT '[]',
    active                  BOOLEAN NOT NULL DEFAULT true,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_transitions_def ON workflow_transitions (workflow_definition_id);
CREATE INDEX idx_workflow_transitions_from ON workflow_transitions (workflow_definition_id, from_state);

-- Workflow Instances (execution of a definition for a specific case)
CREATE TABLE workflow_instances (
    id                          UUID PRIMARY KEY,
    organization_id             UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workflow_definition_id      UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE RESTRICT,
    workflow_definition_version INTEGER NOT NULL,
    case_id                     UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    current_state               TEXT NOT NULL,
    started_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at                TIMESTAMPTZ,
    metadata                    JSONB NOT NULL DEFAULT '{}',
    version                     INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_workflow_instances_org ON workflow_instances (organization_id);
CREATE INDEX idx_workflow_instances_case ON workflow_instances (case_id);
CREATE UNIQUE INDEX idx_workflow_instances_case_unique ON workflow_instances (case_id);

-- Workflow Transition History (persistent audit of every transition)
CREATE TABLE workflow_transition_history (
    id                      UUID PRIMARY KEY,
    organization_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workflow_instance_id    UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    case_id                 UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    from_state              TEXT NOT NULL,
    to_state                TEXT NOT NULL,
    transition_key          TEXT NOT NULL,
    actor_id                UUID REFERENCES users(id) ON DELETE SET NULL,
    occurred_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason                  TEXT,
    metadata                JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_workflow_history_instance ON workflow_transition_history (workflow_instance_id);
CREATE INDEX idx_workflow_history_case ON workflow_transition_history (case_id);

-- Link cases to their workflow instance (nullable for backward compat)
ALTER TABLE cases ADD COLUMN IF NOT EXISTS workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL;
CREATE INDEX idx_cases_workflow_instance ON cases (workflow_instance_id);
