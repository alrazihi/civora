CREATE TABLE organizations (
    id              UUID PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    slug            TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE roles (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    permissions     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE TABLE users (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    name            TEXT NOT NULL,
    role_id         UUID REFERENCES roles(id) ON DELETE SET NULL,
    password_hash   TEXT,
    is_oidc_user    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, email)
);

CREATE TABLE cases (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_number     TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'CREATED',
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assigned_to     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at       TIMESTAMPTZ,
    CONSTRAINT valid_case_status CHECK (
        status IN ('CREATED', 'OPEN', 'IN_REVIEW', 'RESOLVED', 'CLOSED')
    ),
    CONSTRAINT case_number_organization UNIQUE (organization_id, case_number)
);

CREATE TABLE audit_events (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    resource        TEXT NOT NULL,
    resource_id     TEXT,
    outcome         TEXT NOT NULL,
    request_id      TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    previous_hash   TEXT,
    hash            TEXT NOT NULL
);

CREATE INDEX idx_audit_org_timestamp ON audit_events (organization_id, timestamp DESC);
CREATE INDEX idx_audit_hash ON audit_events (hash);
CREATE INDEX idx_cases_org_status ON cases (organization_id, status);
CREATE INDEX idx_users_org_email ON users (organization_id, email);
