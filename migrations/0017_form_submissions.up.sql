CREATE TABLE form_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    form_version_id UUID NOT NULL REFERENCES form_versions(id) ON DELETE CASCADE,
    submitted_by UUID NOT NULL REFERENCES users(id),
    status VARCHAR(50) NOT NULL DEFAULT 'submitted',
    data JSONB NOT NULL,
    submitted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_case_form_version UNIQUE (tenant_id, case_id, form_version_id),
    CONSTRAINT valid_status CHECK (status IN ('submitted', 'rejected', 'corrected', 'approved'))
);

CREATE INDEX idx_form_submissions_tenant_case ON form_submissions (tenant_id, case_id);
CREATE INDEX idx_form_submissions_form_version ON form_submissions (form_version_id);
CREATE INDEX idx_form_submissions_case_form ON form_submissions (tenant_id, case_id, form_id);
CREATE INDEX idx_form_submissions_status ON form_submissions (status);
