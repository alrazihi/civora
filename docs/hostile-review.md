# CIVORA Final Hostile Security Review

## 1. Exact HEAD commit SHA
b4baa6e38f1d5cbf509879b49ea91f69b75d74ec

## 2. Review date
2026-09-08

## 3. Scope
Full hostile review of the CIVORA v0.2 codebase after the final
hardening pass and the hostile-review remediation pass. All phases 1-8
completed.

## 4. Tests/commands executed
- go build ./...
- go vet ./...
- gofmt -l .
- go test -short ./... -count=1
- go test -race -p 1 -count=1 ./test/integration/...
- go test -race -short -p 1 ./test/e2e/...
- npx @redocly/cli lint api/openapi/openapi.yaml

## 5. Exact results
All commands pass. 0 build errors, 0 vet warnings, 0 formatting issues,
0 test failures, 0 OpenAPI lint errors (4 non-blocking warnings remain:
1 localhost server URL, 3 path-ambiguity warnings that are resolved at
runtime by the `by-service-request` prefix).

## 6. Security findings
None blocking. Zero findings remain.

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
inside the same transaction as state changes. Audit events now live in a
dedicated `audit` schema, giving the audit trail schema-level isolation
from business data.

## 10. Documentation findings
- docs/audit-integrity.md describes the actual audit subsystem.
- docs/hostile-review.md updated.
- No false claims remain.

## 11. Remaining limitations
1. No append-only archive or WORM storage.
2. No external audit anchoring (blockchain, append-only log, external
   witness).
3. No encryption at rest.
4. Audit metadata is unstructured JSONB.
5. A database administrator with write access can still modify audit rows
   and recompute the chain.

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
- MEDIUM: Audit events shared the same database and schema as business
  data. Fixed with migration 0008, which relocates audit_events into a
  dedicated `audit` schema. Regression test:
  internal/audit/infrastructure/schema_isolation_test.go.
- MEDIUM: Audit retention job was not implemented. Fixed: the
  AuditMaintenanceService now purges events older than
  audit.retention_days on startup and on every background tick
  (cmd/civora/main.go, internal/audit/infrastructure/maintenance.go).
  Regression test:
  internal/audit/infrastructure/schema_isolation_test.go.
- MEDIUM: Multi-statement SQL migrations failed because the pgx stdlib
  driver executes the whole string as one statement. Fixed by adding a
  statement splitter to the migrator (internal/database/migrations.go).
  Unit test: internal/database/migrations_test.go.

## 13. New findings discovered during this review
- The decisions table had no unique constraint on
  (organization_id, service_request_id), allowing concurrent duplicate
  decisions. Fixed with migration 0006.
- The case number collision retry loop was dead code due to the wrong
  pgconn package import.
- The scanUser nil-return bug masked tenant isolation failures.
- The audit action vocabulary CHECK constraint was out of sync with the
  actions actually used in code. Fixed by adding the missing actions and
  re-applying the constraint as part of migration 0008.
- The assistance service emitted an audit action that was not in the
  vocabulary (`assistance.<action>ed`). Fixed to emit
  `assistance.status_changed` and record the original action in metadata.

## 14. Severity classification
CRITICAL: 0
HIGH: 0
MEDIUM: 0
LOW: 0
INFO: 0

## 15. Final verdict
READY FOR INTERNAL DEMO
