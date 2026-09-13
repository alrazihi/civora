-- v0.5: Deterministic Rules Engine tables.
-- A dedicated `rules` schema provides namespace isolation (matches the audit
-- schema pattern from migration 0008), so rule-engine tables cannot be
-- accidentally confused with business-domain tables.

CREATE SCHEMA IF NOT EXISTS rules;

-- Rule sets: versioned, tenant-scoped collections of rules. Each rule set
-- version is immutable once published — edits produce a new version with a new
-- id. A rule set may be global (case_id IS NULL) or case-bound.
CREATE TABLE rules.rule_sets (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id         UUID REFERENCES cases(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    version         INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'DRAFT',
    default_outcome TEXT NOT NULL DEFAULT 'MANUAL_REVIEW',
    rules           JSONB NOT NULL,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_rule_set_status CHECK (
        status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED')
    ),
    CONSTRAINT valid_default_outcome CHECK (
        default_outcome IN ('ELIGIBLE', 'INELIGIBLE', 'REQUIRES_REVIEW', 'INFORMATION_REQUIRED', 'FLAG', 'SCORE')
    )
);

CREATE INDEX idx_rule_sets_org ON rules.rule_sets (organization_id);
CREATE INDEX idx_rule_sets_org_key ON rules.rule_sets (organization_id, key);
CREATE INDEX idx_rule_sets_case ON rules.rule_sets (organization_id, case_id) WHERE case_id IS NOT NULL;

-- Only one PUBLISHED version per (organization, key, case_id) is permitted.
-- GLOBAL rule sets have case_id = NULL; the partial index still scopes them to
-- the organization. This enforces immutability of active versions.
CREATE UNIQUE INDEX rule_sets_uniq_published
    ON rules.rule_sets (organization_id, key, case_id)
    WHERE status = 'PUBLISHED';

-- Rule set version uniqueness: a second immutable version of the same logical
-- rule set (same key) must use a higher version number.
CREATE UNIQUE INDEX rule_sets_uniq_version
    ON rules.rule_sets (organization_id, key, version);

-- Rule set modification guard: published versions can never be updated again.
-- The repository enforces this in code; this index documents the invariant.
COMMENT ON TABLE rules.rule_sets IS
    'Published versions are immutable. Edits create new versions with new ids.';

-- Evaluations: a single run of a rule set against a case''s frozen facts.
CREATE TABLE rules.evaluations (
    id              UUID PRIMARY KEY,
    rule_set_id     UUID NOT NULL REFERENCES rules.rule_sets(id) ON DELETE RESTRICT,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id         UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    status          TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    reason          TEXT,
    trace           JSONB NOT NULL DEFAULT '{}'::jsonb,
    trigger         TEXT NOT NULL,
    performed_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    facts_snapshot  JSONB NOT NULL,

    CONSTRAINT valid_evaluation_status CHECK (
        status IN ('ELIGIBLE', 'INELIGIBLE', 'REQUIRES_REVIEW', 'INFORMATION_REQUIRED', 'FLAG', 'SCORE', 'ERROR')
    ),
    CONSTRAINT valid_evaluation_trigger CHECK (
        trigger IN ('MANUAL', 'AUTOMATIC')
    ),
    CONSTRAINT valid_evaluation_outcome CHECK (
        outcome IN ('ELIGIBLE', 'INELIGIBLE', 'REQUIRES_REVIEW', 'INFORMATION_REQUIRED', 'FLAG', 'SCORE', 'ERROR')
    )
);

CREATE INDEX idx_evaluations_org ON rules.evaluations (organization_id);
CREATE INDEX idx_evaluations_case ON rules.evaluations (organization_id, case_id);
CREATE INDEX idx_evaluations_rule_set ON rules.evaluations (rule_set_id);
CREATE INDEX idx_evaluations_evaluated_at ON rules.evaluations (organization_id, evaluated_at DESC);
