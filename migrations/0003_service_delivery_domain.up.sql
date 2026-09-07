-- v0.2: Public-Interest Service Delivery Domain Model

-- People (beneficiaries)
CREATE TABLE people (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    external_reference  TEXT,
    first_name          TEXT NOT NULL,
    last_name           TEXT NOT NULL,
    date_of_birth       DATE,
    email               TEXT,
    phone               TEXT,
    address             TEXT,
    city                TEXT,
    preferred_language  TEXT NOT NULL DEFAULT 'en',
    status              TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_people_org ON people (organization_id);
CREATE INDEX idx_people_external_ref ON people (organization_id, external_reference);

-- Service Request lifecycle states
ALTER TABLE cases
    ADD COLUMN IF NOT EXISTS person_id UUID REFERENCES people(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS service_type TEXT NOT NULL DEFAULT 'GENERAL',
    ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'NORMAL';

ALTER TABLE cases DROP CONSTRAINT IF EXISTS valid_case_status;
ALTER TABLE cases ADD CONSTRAINT valid_case_status CHECK (
    status IN ('NEW', 'OPEN', 'IN_REVIEW', 'ASSESSMENT', 'DECISION_PENDING', 'APPROVED', 'REJECTED', 'IN_PROGRESS', 'FOLLOW_UP', 'CLOSED')
);

-- Eligibility assessments
CREATE TABLE eligibilities (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    criteria            JSONB NOT NULL DEFAULT '{}'::jsonb,
    result              TEXT NOT NULL DEFAULT 'REQUIRES_MORE_INFORMATION',
    explanation         TEXT NOT NULL,
    assessed_by         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assessed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_eligibility_result CHECK (
        result IN ('ELIGIBLE', 'NOT_ELIGIBLE', 'REQUIRES_MORE_INFORMATION')
    )
);

CREATE INDEX idx_eligibilities_org ON eligibilities (organization_id);
CREATE INDEX idx_eligibilities_service_request ON eligibilities (service_request_id);

-- Evidence items
CREATE TABLE evidence (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    type                TEXT NOT NULL,
    description         TEXT NOT NULL,
    storage_reference   TEXT NOT NULL,
    uploaded_by         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_evidence_type CHECK (
        type IN ('IDENTITY_DOCUMENT', 'PROOF_OF_RESIDENCE', 'REFERRAL', 'SUPPORTING_DOCUMENT', 'PHOTOGRAPH', 'STAFF_NOTE')
    )
);

CREATE INDEX idx_evidence_org ON evidence (organization_id);
CREATE INDEX idx_evidence_service_request ON evidence (service_request_id);

-- Assessments
CREATE TABLE assessments (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    findings            TEXT NOT NULL,
    needs_identified    TEXT,
    recommendation      TEXT NOT NULL,
    assessor            UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assessed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_assessments_org ON assessments (organization_id);
CREATE INDEX idx_assessments_service_request ON assessments (service_request_id);

-- Human decisions
CREATE TABLE decisions (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    decision            TEXT NOT NULL,
    reason              TEXT NOT NULL,
    decision_maker      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    decided_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_decision CHECK (
        decision IN ('APPROVED', 'REJECTED', 'NEEDS_MORE_INFORMATION')
    )
);

CREATE INDEX idx_decisions_org ON decisions (organization_id);
CREATE INDEX idx_decisions_service_request ON decisions (service_request_id);

-- Assistance / actions
CREATE TABLE assistance (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    type                TEXT NOT NULL,
    description         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'PLANNED',
    responsible_staff   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_assistance_type CHECK (
        type IN ('FINANCIAL', 'FOOD', 'SHELTER', 'MEDICAL', 'EDUCATION', 'TRANSPORT', 'OTHER')
    ),
    CONSTRAINT valid_assistance_status CHECK (
        status IN ('PLANNED', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED')
    )
);

CREATE INDEX idx_assistance_org ON assistance (organization_id);
CREATE INDEX idx_assistance_service_request ON assistance (service_request_id);

-- Follow-ups
CREATE TABLE follow_ups (
    id                  UUID PRIMARY KEY,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_request_id  UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    scheduled_date      DATE NOT NULL,
    completed_date      DATE,
    outcome             TEXT NOT NULL,
    notes               TEXT NOT NULL,
    performed_by        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_follow_ups_org ON follow_ups (organization_id);
CREATE INDEX idx_follow_ups_service_request ON follow_ups (service_request_id);
