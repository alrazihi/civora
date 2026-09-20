-- v0.2 hardening: revert the one-decision-per-service-request-version constraint.
DROP INDEX IF EXISTS idx_decisions_service_request_version_unique;