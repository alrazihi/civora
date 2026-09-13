CREATE TABLE forms (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'DRAFT',
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_form_status CHECK (
        status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')
    ),
    UNIQUE (organization_id, key)
);

CREATE INDEX idx_forms_org ON forms (organization_id);
CREATE INDEX idx_forms_org_key ON forms (organization_id, key);

CREATE TABLE form_versions (
    id              UUID PRIMARY KEY,
    form_id         UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'DRAFT',
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
     published_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (form_id, version),
    CONSTRAINT valid_version_status CHECK (
        status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED')
    )
);

CREATE INDEX idx_form_versions_form ON form_versions (form_id);
CREATE INDEX idx_form_versions_org ON form_versions (organization_id);

CREATE TABLE form_fields (
    id              UUID PRIMARY KEY,
    form_id         UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    form_version_id UUID NOT NULL REFERENCES form_versions(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    label           TEXT NOT NULL,
    type            TEXT NOT NULL,
    required        BOOLEAN NOT NULL DEFAULT false,
    description     TEXT,
    placeholder     TEXT,
    default_value   TEXT,
    validation      JSONB NOT NULL DEFAULT '{}'::jsonb,
    options_json    JSONB NOT NULL DEFAULT '[]'::jsonb,
    "order"         INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_field_type CHECK (
        type IN (
            'TEXT', 'TEXTAREA', 'NUMBER', 'DECIMAL',
            'DATE', 'DATETIME', 'BOOLEAN',
            'SELECT', 'MULTISELECT', 'RADIO', 'CHECKBOX',
            'EMAIL', 'PHONE'
        )
    ),
    UNIQUE (form_version_id, key)
);

CREATE INDEX idx_form_fields_form ON form_fields (form_id);
CREATE INDEX idx_form_fields_version ON form_fields (form_version_id);
CREATE INDEX idx_form_fields_org ON form_fields (organization_id);
