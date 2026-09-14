-- v0.5.1: Rule Templates for cross-organization rule reuse.
-- Rule templates are reusable rule snippets that can be instantiated into
-- rule sets. They exist at two scopes: ORG (tenant-scoped) or GLOBAL (platform-wide).

CREATE TABLE IF NOT EXISTS rules.rule_templates (
    id              UUID PRIMARY KEY,
    scope           TEXT NOT NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    rule_json       JSONB NOT NULL,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT valid_rule_template_scope CHECK (
        scope IN ('ORG', 'GLOBAL')
    ),
    CONSTRAINT valid_rule_template_org CHECK (
        (scope = 'ORG' AND organization_id IS NOT NULL) OR
        (scope = 'GLOBAL' AND organization_id IS NULL)
    )
);

CREATE INDEX idx_rule_templates_org ON rules.rule_templates (organization_id) WHERE scope = 'ORG';
CREATE INDEX idx_rule_templates_org_key ON rules.rule_templates (organization_id, key) WHERE scope = 'ORG';
CREATE INDEX idx_rule_templates_global ON rules.rule_templates (key) WHERE scope = 'GLOBAL';
CREATE INDEX idx_rule_templates_category ON rules.rule_templates (category) WHERE category IS NOT NULL;

-- Ensure unique keys per scope+org
CREATE UNIQUE INDEX rule_templates_uniq_org_key
    ON rules.rule_templates (organization_id, key)
    WHERE scope = 'ORG';

CREATE UNIQUE INDEX rule_templates_uniq_global_key
    ON rules.rule_templates (key)
    WHERE scope = 'GLOBAL';

COMMENT ON TABLE rules.rule_templates IS
    'Reusable rule snippets. ORG scope = tenant-owned. GLOBAL scope = platform templates.';