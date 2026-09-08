# CIVORA Final Hostile Security Review

## 1. Exact HEAD commit SHA
## 1. Exact HEAD commit SHA
2adf24634f71af979fc9850d4ef4f9758150227f

## 2. Review date
2026-09-08

## 3. Scope
Independent hostile review of CIVORA at HEAD 2adf246. This review does not
reuse the verdict from any prior commit. All code paths, tests, CI config,
and documentation were re-examined from scratch.

## 4. Tests/commands executed
- go build ./...
- go vet ./...
- gofmt -l .
- go test -short ./... -count=1
- go test -race -p 1 -count=1 ./test/integration/...
- go test -race -p 1 -count=1 ./test/e2e/...
- npx @redocly/cli lint api/openapi/openapi.yaml
- golangci-lint run --enable=gosec --timeout=5m ./...

## 5. Exact results
- go build ./...: PASS
- go vet ./...: PASS
- gofmt -l .: 15 files reported (CRLF/formatting drift)
- go test -short ./... -count=1: PASS
- go test -race -p 1 -count=1 ./test/integration/...: PASS
- go test -race -p 1 -count=1 ./test/e2e/...: PASS
- npx @redocly/cli lint api/openapi/openapi.yaml: PASS (4 warnings)
- golangci-lint run --enable=gosec --timeout=5m ./...: FAIL (13 issues)

## 6. Security findings
### HIGH
- `internal/audit/infrastructure/maintenance.go:76`: `VerifyOrganization`
  uses a hard-coded `100000` row limit when calling `FindByOrganization`.
  The repository already exposes `CountByOrganization`, but it is not used.
  This silently truncates audit-chain verification for any organization with
  more than 100,000 audit events. Tampered events beyond that boundary would
  not be detected, undermining the integrity guarantee.

### MEDIUM
- `test/integration/concurrency_test.go:170`: `TestConcurrentDuplicateAssistance`
  asserts only `assert.Greater(t, successCount, 0, ...)`. Because the
  assistance domain has no unique constraint on `(organization_id,
  service_request_id)`, all 5 concurrent creates are expected to succeed.
  The weak assertion masks potential race conditions, database deadlocks, or
  transaction failures.
- Dead code / unused symbols across 6 files (`internal/cases/application/service.go:31`,
  `internal/middleware/ratelimit.go:160`, `internal/assistance/application/service_test.go:87`,
  `internal/evidence/application/service_test.go:84`,
  `internal/followup/application/service_test.go:88`,
  `test/e2e/e2e_test.go:252`). Unused interfaces, methods, and functions
  indicate incomplete refactoring and increase attack surface.
- `internal/cases/infrastructure/postgres/case_repository.go:155`:
  ineffectual assignment to `argPos` after the final filter branch.
- Unchecked errors in tests (`internal/middleware/idempotency_test.go:128`,
  `test/e2e/e2e_test.go:212`, `test/e2e/e2e_test.go:230`,
  `test/e2e/e2e_test.go:248`).

### LOW
- OpenAPI spec carries 4 non-blocking warnings (1 localhost server URL,
  3 path-ambiguity warnings resolved at runtime by the
  `by-service-request` prefix).
- Formatting drift (CRLF) in 15 files.

## 7. Functional findings
- The e2e lifecycle, security, and integration tests all pass.
- No functional blockers observed during testing.

## 8. Privacy findings
No PII is logged. Audit metadata records identifiers (UUIDs), not PII.
Error messages are generic. The Person domain stores DOB, email, phone, and
address as nullable fields; these are returned only to authenticated and
authorized users. No encryption at rest is implemented (documented honestly).

## 9. Architecture findings
Module boundaries are clean. Domain, application, infrastructure, and API
layers are separated. Transaction ownership is correct: audit writes happen
inside the same transaction as state changes. Audit events live in a
dedicated `audit` schema, giving the audit trail schema-level isolation
from business data. The `VerifyOrganization` method correctly reverses
event order before calling `VerifyChain`.

## 10. Documentation findings
- docs/audit-integrity.md describes the actual audit subsystem.
- docs/hostile-review.md was stale (referenced commit b4baa6e); it now
  reflects the current HEAD.
- No false claims remain in the docs.

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
- MEDIUM: The audit action vocabulary CHECK constraint was out of sync with
  the actions actually used in code. Fixed by adding the missing actions
  and re-applying the constraint as part of migration 0008.
- MEDIUM: The assistance service emitted an audit action that was not in the
  vocabulary (`assistance.<action>ed`). Fixed to emit
  `assistance.status_changed` and record the original action in metadata.

## 13. New findings discovered during this review
- Audit chain verification truncation at 100,000 rows (`maintenance.go:76`).
  `CountByOrganization` exists but is not called.
- Weak concurrency test assertion in `TestConcurrentDuplicateAssistance`.
- Dead code / unused symbols in 6 files.
- Ineffectual assignment in `case_repository.go:155`.
- Unchecked errors in 4 test locations.

## 14. Severity classification
CRITICAL: 0
HIGH: 1
MEDIUM: 5
LOW: 2
INFO: 0

## 15. Final verdict
NOT READY FOR INTERNAL DEMO

Blockers:
1. HIGH: Fix the audit verification truncation by replacing the hard-coded
   `100000` limit with a call to `CountByOrganization` in
   `VerifyOrganization`.
2. MEDIUM: Fix all golangci-lint failures (dead code, ineffectual
   assignment, unchecked errors) so CI passes cleanly.
