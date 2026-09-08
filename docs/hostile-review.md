# CIVORA Final Hostile Security Review

## 1. Exact HEAD commit SHA
9216713780b3fc394ee1358287aee21aca02db4f

## 2. Review date
2026-09-08

## 3. Scope
Independent hostile review of CIVORA at HEAD 9216713780b3fc394ee1358287aee21aca02db4f.
This review does not reuse the verdict from any prior commit. All code paths, tests,
CI config, and documentation were re-examined from scratch.

Note: The current HEAD `9216713780b3fc394ee1358287aee21aca02db4f` is a
documentation-only commit. The actual code changes under review were introduced
in earlier commits:

Changes inspected since 7e94f86:
- 21a62a7 (docs: update hostile security review findings)
- 2adf24634f71af979fc9850d4ef4f9758150227f (code fixes for review findings)
- b0233b3 (docs: update hostile security review findings)
- 6275c63318e05f427527884e1facb8452e9216f0 (merge: fix audit row limit + lint-clean)
- 9216713780b3fc394ee1358287aee21aca02db4f0 (docs: update hostile security review to reflect resolved findings)

## 4. Tests/commands executed
- go build ./...
- go vet ./...
- gofmt -l .
- go test -short -race -p 1 -count=1 ./...
- go test -race -p 1 -count=1 ./test/integration/...
- go test -race -p 1 -count=1 ./test/e2e/...
- npx @redocly/cli lint api/openapi/openapi.yaml
- golangci-lint run --enable=gosec --timeout=5m ./...

## 5. Exact results
- go build ./...: PASS
- go vet ./...: PASS
- gofmt -l .: 15 files reported (CRLF/formatting drift)
- go test -short -race -p 1 -count=1 ./...: PASS
- go test -race -p 1 -count=1 ./test/integration/...: PASS (with clean database)
- go test -race -p 1 -count=1 ./test/e2e/...: PASS
- npx @redocly/cli lint api/openapi/openapi.yaml: PASS (4 non-blocking warnings)
- golangci-lint run --enable=gosec --timeout=5m ./...: PASS

## 6. Security findings
### HIGH
None.

### MEDIUM
None.

### LOW
- OpenAPI spec carries 4 non-blocking warnings (1 localhost server URL,
  3 path-ambiguity warnings resolved at runtime by the
  `by-service-request` prefix).
- Formatting drift (CRLF) in 15 files.

## 7. Functional findings
- All e2e lifecycle, security, and integration tests pass.
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
- No false claims remain in the docs.

## 11. Remaining limitations
1. No append-only archive or WORM storage.
2. No external audit anchoring (blockchain, append-only log, external
   witness).
3. No encryption at rest.
4. Audit metadata is unstructured JSONB.
5. A database administrator with write access can still modify audit rows
   and recompute the chain.

## 12. Resolved findings from prior review
- HIGH: Audit chain verification truncated at 100,000 rows. Fixed by replacing
  the hard-coded limit with `CountByOrganization` in
  `internal/audit/infrastructure/maintenance.go`.
- MEDIUM: Weak concurrency assertion in
  `TestConcurrentDuplicateAssistance`. Fixed to assert all 5 concurrent
  creates succeed.
- MEDIUM: Dead code / unused symbols in 6 files. Fixed by removing
  `txEventRecorder`, `getAttempt`, `addMember`, and `tablesExist`.
- MEDIUM: Ineffectual assignment in `case_repository.go:155`. Fixed by
  removing the redundant `argPos++`.
- MEDIUM: Unchecked errors in 4 test locations. Fixed by adding
  `require.NoError` checks and removing duplicate JSON unmarshal in
  `TestServiceRequestFullLifecycle`.
- CRITICAL (historical): pgx/v5 stdlib driver error matching. Fixed by
  matching on SQLSTATE 23505.
- CRITICAL (historical): `scanUser` returned `(nil, nil)` on
  `sql.ErrNoRows`. Fixed by returning `domain.ErrUserNotFound`.

## 13. New findings discovered during this review
None.

## 14. Severity classification
CRITICAL: 0
HIGH: 0
MEDIUM: 0
LOW: 2
INFO: 0

## 15. Final verdict
READY FOR INTERNAL DEMO

No blockers remain. All tests pass, lint passes, and no HIGH or MEDIUM
findings were identified.
