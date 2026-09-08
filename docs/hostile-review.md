# CIVORA Final Hostile Security Review

## 1. Exact HEAD commit SHA
c8e28ff065c4cef8263beb44da536c63723e7920

## 2. Review date
2026-09-08

## 3. Scope
Full hostile review of the CIVORA v0.2 codebase after the final
hardening pass. All phases 1-8 completed.

## 4. Tests/commands executed
- go build ./...
- go vet ./...
- gofmt -l .
- go test -race -short -p 1 ./...
- go test -race -p 1 -count=1 ./internal/... ./test/e2e/... ./test/integration/...
- npx @redocly/cli lint api/openapi/openapi.yaml
- go test -race -p 1 -count=1 ./test/integration/... (concurrency tests)

## 5. Exact results
All commands pass. 0 build errors, 0 vet warnings, 0 formatting issues,
0 test failures, 0 OpenAPI lint warnings.

## 6. Security findings
None blocking. Two medium findings remain:

### MED-01: Audit events share the same database as business data
The audit_events table is in the same PostgreSQL instance and public schema
as all other entities. There is no separate audit store, no append-only
archive, and no external cryptographic anchoring. Mitigation: hash chain
per organization with SHA-256, integrity_valid flag in the audit API.
Status: documented in docs/audit-integrity.md. Does not block release.

### MED-02: Audit retention job is not implemented
The audit.retention_days configuration value is accepted but no automated
purge runs. Mitigation: documented as a known limitation.
Status: does not block release.

## 7. Functional findings
None blocking.

## 8. Privacy findings
No PII is logged. Audit metadata records identifiers (UUIDs), not PII.
Error messages are generic. The Person domain stores DOB, email, phone, and
address as nullable fields; these are returned only to authenticated and
authorized users. No encryption at rest is implemented (documented honestly).

## 9. Architecture findings
Module boundaries are clean. Domain, application, infrastructure, and API
layers are separated. Transaction ownership is correct: audit writes happen
inside the same transaction as state changes.

## 10. Documentation findings
- docs/audit-integrity.md added to describe the actual audit subsystem.
- docs/hostile-review.md updated.
- No false claims remain.

## 11. Remaining limitations
1. Audit events are not stored separately from business data.
2. No automated audit retention/deletion.
3. No background audit verification job.
4. No encryption at rest.
5. No external audit anchoring.
6. Audit metadata is unstructured JSONB.

## 12. Resolved findings
- CRITICAL: pgx/v5 stdlib driver returns *pgconn.PgError from
  github.com/jackc/pgx/v5/pgconn, but case_repository.go imported
  github.com/jackc/pgconn. errors.As always failed, so case number collision
  retries never worked. Fixed by matching on SQLSTATE 23505.
- CRITICAL: scanUser returned (nil, nil) on sql.ErrNoRows, breaking tenant
  isolation checks and causing nil-pointer panics. Fixed by returning
  domain.ErrUserNotFound.
- HIGH: Unchecked errors fixed across 40+ call sites.
- HIGH: OpenAPI ambiguity warnings fixed by reordering paths and renaming
  the literal service-request segment to by-service-request.
- HIGH: Audit hash chain integrity now returns an error on metadata
  serialization failure instead of silently dropping it.
- HIGH: Decisions table now has a unique constraint on
  (organization_id, service_request_id) via migration 0006, preventing
  concurrent duplicate decisions.

## 13. New findings discovered during this review
- The decisions table had no unique constraint on
  (organization_id, service_request_id), allowing concurrent duplicate
  decisions. Fixed with migration 0006.
- The case number collision retry loop was dead code due to the wrong
  pgconn package import.
- The scanUser nil-return bug masked tenant isolation failures.

## 14. Severity classification
CRITICAL: 0
HIGH: 0
MEDIUM: 2 (audit storage, audit retention)
LOW: 0
INFO: 0

## 15. Final verdict
READY FOR INTERNAL DEMO
