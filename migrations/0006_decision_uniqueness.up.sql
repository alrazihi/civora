-- v0.2 hardening: enforce one decision per service request.
-- Without this constraint, concurrent calls to POST /decisions for the same
-- service request can both succeed, producing duplicate decisions and
-- bypassing the one-decision-per-case lifecycle rule.
CREATE UNIQUE INDEX idx_decisions_service_request_unique
    ON decisions (organization_id, service_request_id);