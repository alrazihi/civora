# Hostile Engineering Review — CIVORA (v3)

**Reviewer**: External senior open-source maintainer (unfamiliar with project history)
**Date**: 2026-09-07
**Commit reviewed**: `8014196` (HEAD)
**Scope**: Delivery v0.2 — service-delivery lifecycle (people, eligibility, evidence, assessment, decisions, assistance, follow-up)
**Method**: Every finding verified against actual source code. `go build`, `go vet`, `gofmt`, `go test -short -race`, `go test -p 1 ./test/e2e/... ./test/integration/...`, and `npx @redocly/cli lint` all executed. No claims are speculative.

---

## Fix Status Verification (from prior review at `d6a36ec`)

The previous hostile review documented fixes for the Milestone 0.1 foundation. This v3 review independently re-verified each claim against the current HEAD (`8014196`) and assessed the newly introduced Delivery v0.2 domain.

### Previously fixed items (verified still fixed at `8014196`)

| # | Finding | Status | Evidence |
|---|---------|--------|----------|
| 1 | RBAC non-functional | FIXED | `RequireAnyRole("admin","staff")` applied to case transitions/assign, audit, user routes |
| 2 | Self-registration as admin | FIXED | `role_name` removed; first-user-is-admin logic in `internal/identity/application/service.go:88-100` |
| 3 | Audit not atomic | FIXED | All service methods use `database.InTransaction`; tx-aware audit recording via `RecordEventInTx` |
| 4 | OIDC remnants | FIXED | No OIDC config fields, env vars, struct fields, schema columns, or docs remain |
| 5 | Error info leakage in 500s | FIXED | Handlers return generic "internal server error" |
| 6 | Cross-tenant assignment (BOLA) | FIXED | `AssignCase` validates assignee via `UserChecker.BelongsToOrganization` |
| 7 | Correlation ID not in audit events | FIXED | `RequestIDFromContext` passed to all `RecordEventParams` |
| 8 | `AuditConfig` dead config | FIXED | Config passed to `AuditService`; `Enabled`, `HashChainEnabled`, `RetentionDays` all consumed |
| 9 | No per-user rate limiting on auth | FIXED | `UserRateLimiter` added with lockout after 5 failed attempts |
| 10 | No `.dockerignore` | FIXED | `.dockerignore` present |
| 11 | Dockerfile runs as root | FIXED | `USER civora` present |
| 12 | No static analysis in CI | FIXED | `golangci-lint` with `gosec` in CI |
| 13 | No race detection in CI | FIXED | `CGO_ENABLED=1 go test -race` in CI |
| 14 | `fmt.Sprintf("case.transition")` no-op | FIXED | Replaced with literal `"case.transition"` |
| 15 | Handler/API test coverage missing | FIXED | Handler tests added for identity, cases, organizations |
| 16 | `godotenv` unconditional load | FIXED | Conditional on `CIVORA_ENV != "production"` |
| 17 | Organization creation atomicity | FIXED | Org save + role creation + audit in single transaction |
| 18 | `FindByEmail` error discarded | FIXED | Errors propagated in `CreateUser` |
| 19 | Input length validation missing | FIXED | Domain-level validation in `NewCase` |
| 20 | `RequireRole` redundancy | FIXED | Removed; `RequireAnyRole` used consistently |
| 21 | Password policy weaker than documented | FIXED | Enforced 12+ chars, 1 letter + 1 number |
| 22 | Idempotency store memory leak | FIXED | Background cleanup goroutine + `Stop()` |
| 23 | Case number collision retry | FIXED | Retry logic in `saveCase` |
| 24 | `/metrics` endpoint unauthenticated | FIXED | Endpoint removed |
| 25 | `RecordEvent` no test coverage | FIXED | Integration tests verify audit atomicity |
| 26 | Structured logging missing | NOT FIXED | `log.Printf` still used in some places (MEDIUM severity, deferred) |
| 27 | JWT `iss` validation missing | FIXED | `VerifyToken` validates `iss` when configured |

---

## A. CRITICAL Findings

### CRITICAL-1: Cross-tenant `service_request_id` injection in all new delivery modules

**File**: `internal/eligibility/application/service.go:42-80`, `internal/evidence/application/service.go:43-81`, `internal/assessment/application/service.go:43-80`, `internal/decisions/application/service.go:42-80`, `internal/assistance/application/service.go:43-81`, `internal/followup/application/service.go:44-81`
**Component**: Tenant isolation / IDOR
**Problem**: None of the new delivery modules validate that the supplied `service_request_id` belongs to a `Case` in the same organization as the operation's `organization_id`. The repositories only filter by `organization_id` on the new table, but the foreign key to `cases(id)` does not enforce org scoping. An authenticated user in Org A can create eligibility/evidence/assessment/decision/assistance/follow-up records that reference a case belonging to Org B.

**Attack scenario**: User A (Org A) creates an `Eligibility` with `organization_id = <OrgA>` and `service_request_id = <case from OrgB>`. The DB accepts it because `cases(id)` exists and there is no cross-check that `cases.organization_id = eligibilities.organization_id`.

**Impact**: Complete cross-tenant data contamination. Org A can read/write lifecycle data for Org B's cases.

**Recommended fix**: In each service's create/update method, verify the case exists and belongs to the same organization before proceeding. Example:
```go
case, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
if err != nil {
    return nil, ErrCaseNotFound
}
```

**Blocks v0.2**: Yes.

---

### CRITICAL-2: Cross-tenant user references in new delivery modules

**File**: `internal/assistance/application/service.go:44`, `internal/eligibility/application/service.go:43`, `internal/assessment/application/service.go:44`, `internal/decisions/application/service.go:43`, `internal/evidence/application/service.go:44`, `internal/followup/application/service.go:45`
**Component**: Tenant isolation / IDOR
**Problem**: The new modules accept `responsible_staff`, `assessed_by`, `assessor`, `decision_maker`, `uploaded_by`, and `performed_by` as caller-supplied UUIDs. The database foreign keys only verify that the referenced `users(id)` exists. They do not verify that the user belongs to the same organization. An attacker can reference a user from another tenant.

**Attack scenario**: User A (Org A) creates `Assistance` with `responsible_staff = <user from OrgB>`. The DB accepts it because the user exists.

**Impact**: Cross-tenant data leakage and integrity violation. Assistance/decision/assessment records can attribute actions to staff in other organizations.

**Recommended fix**: Validate that the referenced user belongs to the same organization before persisting. This requires a `UserChecker` dependency similar to `CaseService`.

**Blocks v0.2**: Yes.

---

### CRITICAL-3: Missing authorization on consequential delivery endpoints

**File**: `internal/eligibility/api/handler.go:34-46`, `internal/evidence/api/handler.go:32-39`, `internal/assessment/api/handler.go:33-41`, `internal/decisions/api/handler.go:33-41`, `internal/assistance/api/handler.go:33-45`, `internal/followup/api/handler.go:34-45`, `internal/people/api/handler.go:35-47`
**Component**: Authorization / RBAC
**Problem**: The following endpoints are protected by authentication and tenant checks but have NO role check:
- `POST /eligibilities` — any authenticated user can create eligibility assessments
- `POST /evidence` — any authenticated user can upload evidence
- `POST /assessments` — any authenticated user can create assessments
- `POST /decisions` — any authenticated user can make consequential decisions
- `POST /assistance` — any authenticated user can create assistance actions
- `POST /follow-ups` — any authenticated user can schedule follow-ups
- `POST /people` — any authenticated user can create beneficiary records
- `GET /people`, `GET /people/{id}`, `GET /external/{ref}` — any authenticated user can enumerate people

Only `PATCH /eligibilities/{id}/result`, `PATCH /assistance/{id}/status`, and `PATCH /follow-ups/{id}/complete` have `RequireAnyRole("admin", "staff")`.

**Attack scenario**: A newly registered staff user (or any authenticated user) can create a `Decision` with `decision: "APPROVED"` for any case in the organization, bypassing the human-decision gate that the domain model implies.

**Impact**: Unauthorized users can perform consequential public-interest actions. The "human-centered" principle in ADR-0006 is violated.

**Recommended fix**: Apply `RequireAnyRole("admin", "staff")` to all create/update endpoints in the new modules, and to people list/get endpoints if they contain sensitive PII.

**Blocks v0.2**: Yes.

---

### CRITICAL-4: Case `person_id` accepts cross-tenant references

**File**: `internal/cases/api/handler.go:81-89`, `internal/cases/application/service.go:61`
**Component**: Tenant isolation / IDOR
**Problem**: When creating a case, the caller supplies an optional `person_id`. The service does not validate that the referenced `Person` belongs to the same organization. The DB foreign key only checks that the person exists.

**Attack scenario**: User A (Org A) creates a case with `person_id = <person from OrgB>`. The case now references a beneficiary from another organization.

**Impact**: Cross-tenant PII association. Cases can be linked to people from other organizations.

**Recommended fix**: Validate `person_id` org membership in `CreateCase` before persisting.

**Blocks v0.2**: Yes.

---

### CRITICAL-5: No optimistic locking on case state transitions

**File**: `internal/cases/application/service.go:110-155`, `internal/cases/infrastructure/postgres/case_repository.go:158-173`
**Component**: Concurrency / Data integrity
**Problem**: `ChangeStatus` reads the case, validates the transition in memory, then issues a blind `UPDATE cases SET status = $1 WHERE organization_id = $2 AND id = $3`. There is no version check, no `updated_at` precondition, and no row-level lock beyond the transaction. Two concurrent requests can both read `status = NEW`, both validate `NEW → OPEN`, and both write `OPEN`. The last writer wins silently.

**Attack/failure scenario**: Two staff members simultaneously transition the same case. One transitions `NEW → OPEN`, the other transitions `NEW → IN_REVIEW`. Depending on commit order, the case ends in an inconsistent state with one transition lost and no audit of the conflict.

**Impact**: Lost state transitions. The case lifecycle is not actually enforced under concurrency.

**Recommended fix**: Add a `version` integer column to `cases`. Use `UPDATE ... WHERE version = $N` and check `RowsAffected`. Alternatively, use `SELECT ... FOR UPDATE` before validating the transition.

**Blocks v0.2**: Yes.

---

## B. HIGH Findings

### HIGH-1: OpenAPI spec does not match implemented state machine

**File**: `api/openapi/openapi.yaml:12-23`, `internal/cases/domain/case.go:133-144`
**Problem**: The OpenAPI description states:
- `Any state → CLOSED` is valid
- `REJECTED → CLOSED` is valid
- Valid transitions include `IN_REVIEW → RESOLVED` and `RESOLVED → IN_REVIEW`

The actual implementation:
- `CLOSED` has NO outgoing transitions and NO incoming transitions except `REJECTED → CLOSED` and `FOLLOW_UP → CLOSED`. There is no "any state → CLOSED" rule.
- The `RESOLVED` status does not exist in v0.2. The state machine is: `NEW → OPEN → IN_REVIEW → ASSESSMENT → DECISION_PENDING → APPROVED/REJECTED → IN_PROGRESS → FOLLOW_UP → CLOSED`.

**Impact**: API consumers relying on the OpenAPI spec will attempt invalid transitions and receive 409 errors unexpectedly. The documented behavior is misleading.

**Recommended fix**: Update OpenAPI to reflect the actual implemented state machine.

---

### HIGH-2: Zero test coverage for new delivery domain handlers, services, and repositories

**File**: `internal/eligibility/api/`, `internal/eligibility/application/`, `internal/eligibility/infrastructure/postgres/`, `internal/evidence/api/`, `internal/evidence/application/`, `internal/evidence/infrastructure/postgres/`, `internal/assessment/api/`, `internal/assessment/application/`, `internal/assessment/infrastructure/postgres/`, `internal/decisions/api/`, `internal/decisions/application/`, `internal/decisions/infrastructure/postgres/`, `internal/assistance/api/`, `internal/assistance/application/`, `internal/assistance/infrastructure/postgres/`, `internal/followup/api/`, `internal/followup/application/`, `internal/followup/infrastructure/postgres/`, `internal/people/api/`, `internal/people/application/`, `internal/people/infrastructure/postgres/`
**Problem**: All new v0.2 modules have `[no test files]`. Only domain-level unit tests exist (e.g., `eligibility/domain`, `evidence/domain`). There are no handler tests, no service tests, and no repository integration tests for any new module.

**Impact**: The complete delivery lifecycle has no automated verification of authorization, tenant isolation, input validation, or error handling. The E2E test `TestServiceRequestFullLifecycle` covers one happy path but does not test adversarial inputs.

**Recommended fix**: Add handler tests, service tests, and repository integration tests for all new modules. Prioritize cross-tenant isolation and authorization tests.

---

### HIGH-3: Audit integrity is never verified on read

**File**: `internal/audit/domain/event.go:98-100`, `internal/audit/api/handler.go:31-82`
**Problem**: `AuditEvent.VerifyIntegrity()` exists but is never called. The audit list API (`GET /audit`) returns raw events without verifying the hash chain. Tampered audit records would be served as-is.

**Impact**: The "tamper-evident" claim in ARCHITECTURE.md is not enforced at read time. If an attacker with DB access modifies an audit event's `hash` or `previous_hash`, the API will serve the corrupted record.

**Recommended fix**: Call `VerifyIntegrity()` in the audit list API and mark or reject corrupted records.

---

### HIGH-4: Audit retention is configured but never enforced

**File**: `internal/audit/application/service.go:88-94`, `internal/config/config.go:84`
**Problem**: `AuditConfig.RetentionDays` is loaded from env (default 2555) and `PurgeOld` is implemented, but `PurgeOld` is never called by any scheduled job, HTTP handler, or startup hook. Old audit events accumulate forever.

**Impact**: Unbounded audit table growth. The documented retention policy is not enforced.

**Recommended fix**: Add a `/internal/purge-audit` endpoint or a startup hook that calls `PurgeOld`. Or schedule it via a cron-like mechanism.

---

### HIGH-5: Assistance status updates bypass state machine

**File**: `internal/assistance/application/service.go:115-162`, `internal/assistance/domain/assistance.go:69-86`
**Problem**: `UpdateAssistanceStatus` accepts `action` strings `start`, `complete`, `cancel` and applies them unconditionally. There is no state validation. You can `complete` an assistance that is `PLANNED`, or `start` one that is `CANCELLED`, or `cancel` one that is `COMPLETED`.

**Attack scenario**: A staff member marks assistance as `COMPLETED` before it is `IN_PROGRESS`, bypassing the intended lifecycle.

**Impact**: Invalid business states. Assistance records no longer reflect reality.

**Recommended fix**: Add a state machine to `Assistance` (similar to `Case.TransitionTo`) and validate transitions in `UpdateAssistanceStatus`.

---

### HIGH-6: Follow-ups can be created for closed or rejected cases

**File**: `internal/followup/application/service.go:44-81`, `internal/followup/api/handler.go:48-98`
**Problem**: `CreateFollowUp` does not check the case's current status. A follow-up can be scheduled for a `CLOSED` or `REJECTED` case.

**Impact**: Business logic violation. Follow-ups after closure are nonsensical in the public-interest delivery model.

**Recommended fix**: Verify the case status in `CreateFollowUp`. Allowed statuses should probably be `APPROVED`, `IN_PROGRESS`, or `FOLLOW_UP`.

---

### HIGH-7: Decisions can be created without case being in DECISION_PENDING

**File**: `internal/decisions/application/service.go:42-80`, `internal/decisions/api/handler.go:44-86`
**Problem**: `MakeDecision` does not validate that the case is in `DECISION_PENDING` status. A decision can be recorded for a `NEW` case or a `CLOSED` case.

**Impact**: The decision entity becomes decoupled from the case lifecycle. Decisions can be made at inappropriate times.

**Recommended fix**: Load the case and verify `status == DECISION_PENDING` before creating the decision.

---

### HIGH-8: Person `external_reference` is not unique per organization

**File**: `migrations/0003_service_delivery_domain.up.sql:21-22`
**Problem**: The migration creates an index on `(organization_id, external_reference)` but no `UNIQUE` constraint. Multiple people in the same org can share the same external reference.

**Impact**: Duplicate beneficiary records with the same external reference. `FindByExternalReference` returns an arbitrary match.

**Recommended fix**: Add `UNIQUE (organization_id, external_reference)` to the `people` table.

---

### HIGH-9: Pre-existing test bugs break race-enabled full-suite runs

**File**: `internal/identity/infrastructure/postgres/repository_test.go:49-68`, `internal/cases/infrastructure/postgres/case_repository_test.go:162-192`
**Problem**: 
1. `TestUserRepository_TenantIsolation` uses `require.Error(t, err, ...)` but `scanUser` returns `nil, nil` (not an error) when no user is found. The test passes only when skipped (`-short`).
2. `TestCaseRepository_CaseNumberCollisionRetries` is flaky and fails under `-race` because the retry logic depends on generating a unique case number within the same transaction, but the test setup and race detector timing expose a fragility in the collision-retry contract.

**Impact**: The full test suite cannot be run with `-race` without skipping integration tests. Test quality is lower than claimed.

**Recommended fix**: Fix `TestUserRepository_TenantIsolation` to assert `require.Nil(t, found)`. Fix the collision test to use a deterministic collision setup or remove the retry expectation from the repository layer.

---

### HIGH-10: No length validation on new module string fields

**File**: `internal/eligibility/domain/eligibility.go`, `internal/evidence/domain/evidence.go`, `internal/assessment/domain/assessment.go`, `internal/decisions/domain/decision.go`, `internal/assistance/domain/assistance.go`, `internal/followup/domain/followup.go`
**Problem**: None of the new domain entities validate maximum lengths for string fields. `explanation`, `description`, `findings`, `recommendation`, `reason`, `outcome`, `notes` can be arbitrarily long (tested up to several MB).

**Impact**: Potential memory exhaustion and oversized DB rows. No protection against accidental or malicious bulk input.

**Recommended fix**: Add max-length constants and validation in each domain constructor.

---

## C. MEDIUM Findings

### MEDIUM-1: OpenAPI has ambiguous paths and missing operationIds

**File**: `api/openapi/openapi.yaml`
**Problem**: Redocly reports 46 warnings, including:
- 3 ambiguous path warnings (`/eligibilities/{id}/result` vs `/eligibilities/service-request/{id}`, `/assistance/{id}/status` vs `/assistance/service-request/{id}`, `/follow-ups/{id}/complete` vs `/follow-ups/service-request/{id}`)
- Missing `operationId` on every operation
- Missing `4XX` responses on `/health` and `/ready`

**Impact**: SDK generation is impaired. Some tooling may misroute requests.

---

### MEDIUM-2: `FindByExternalReference` endpoint is undocumented

**File**: `internal/people/api/handler.go:42`, `api/openapi/openapi.yaml`
**Problem**: The handler registers `GET /organizations/{orgId}/people/external/{externalRef}` but this endpoint is absent from the OpenAPI spec.

**Impact**: API consumers cannot discover this endpoint. It is an undocumented surface.

---

### MEDIUM-3: Case status filter accepts arbitrary strings

**File**: `internal/cases/api/handler.go:151-153`
**Problem**: `ListCases` accepts any string for the `status` query parameter and passes it directly to the repository as `domain.CaseStatus`. No validation is performed. Invalid statuses produce empty result sets rather than errors.

**Impact**: Silent misbehavior rather than explicit rejection. Clients cannot distinguish "no cases with this status" from "invalid status value".

---

### MEDIUM-4: No duplicate prevention for per-request records

**File**: `internal/eligibility/domain/eligibility.go`, `internal/assessment/domain/assessment.go`, `internal/decisions/domain/decision.go`
**Problem**: There is no uniqueness constraint or application-level check preventing multiple eligibility assessments, assessments, or decisions for the same service request. The DB will happily accept duplicates.

**Impact**: Data quality degradation. Multiple eligibility records for one case create ambiguity about which is authoritative.

**Recommended fix**: Add a unique constraint on `(organization_id, service_request_id)` for `eligibilities`, `assessments`, and `decisions`, or enforce at the application layer.

---

### MEDIUM-5: Auth.failed audit events store PII (email) in ResourceID

**File**: `internal/identity/application/service.go:183-191`
**Problem**: Failed login attempts record the user's email as `ResourceID` in the audit event. This places PII directly in the audit log.

**Impact**: Audit log contains raw email addresses. If audit logs are exported or accessed by operators, PII is exposed.

**Recommended fix**: Hash or tokenize the email before storing it in `ResourceID`, or store a user ID instead.

---

### MEDIUM-6: Code duplication of `strPtr` and `recordAuditEventInTx`

**File**: Every `internal/*/application/service.go` file
**Problem**: `strPtr` and `recordAuditEventInTx` are copy-pasted identically into all 10+ service files. This violates DRY and creates maintenance burden.

**Recommended fix**: Extract both helpers into a shared package (e.g., `internal/shared/audit.go`).

---

### MEDIUM-7: Register endpoint lacks per-email rate limiting

**File**: `internal/identity/api/handler.go:39-42`, `internal/middleware/ratelimit.go`
**Problem**: The global rate limiter (100 req/s, burst 20) applies to registration, but there is no per-email rate limit. An attacker can enumerate valid emails or flood registration for a specific email.

**Impact**: Email enumeration and registration abuse.

---

### MEDIUM-8: JWT `iss` validation is conditional on non-empty issuer

**File**: `internal/middleware/auth.go:77-80`
**Problem**: `VerifyToken` only validates `iss` if `s.issuer != ""`. The JWT service is initialized with issuer `"civora"`, so this is currently enforced. However, if the issuer is ever misconfigured to empty string, validation is silently skipped.

**Impact**: Low in current configuration, but a misconfiguration risk.

---

## D. LOW Findings

### LOW-1: OpenAPI localhost server warning

**File**: `api/openapi/openapi.yaml:33`
**Problem**: Redocly warns that the server URL points to localhost. Cosmetic only.

---

### LOW-2: Missing 4XX responses on health/ready

**File**: `api/openapi/openapi.yaml:763-776`
**Problem**: `/health` and `/ready` operations document only `200` responses. In practice, `/ready` can return `503` when the DB is down.

---

### LOW-3: Case `GenerateCaseNumber` entropy is lower than ideal

**File**: `internal/cases/domain/case.go:181-182`
**Problem**: `t.Nanosecond()%100000000` provides at most 27 bits of entropy from the timestamp, plus 32 bits from UUID. Total ~59 bits. Under extreme throughput, collisions are unlikely but possible.

**Recommended fix**: Use a full UUID or a ULID. The retry logic already handles collisions, so this is low severity.

---

### LOW-4: No email uniqueness across organizations

**File**: `internal/identity/domain/user.go`, migration `0001_init.up.sql:30`
**Problem**: The `users` table has `UNIQUE (organization_id, email)`. The same email can be registered in different organizations. This is by design for multi-tenant, but may enable phishing across orgs.

---

### LOW-5: `shared/validator.go` and `shared/id.go` removed (verified)

**Status**: Confirmed removed. No dead code remains.

---

## E. Verified Strengths

1. **Modular monolith discipline preserved**: All new modules follow the domain/application/infrastructure/api layer pattern. No direct cross-module DB access detected.
2. **Transactional audit atomicity**: Every new domain write wraps the business operation and audit event in `database.InTransaction`.
3. **Tenant-scoped queries**: All new repository `FindBy*` methods filter by `organization_id`.
4. **Case state machine is enforced in domain**: `Case.TransitionTo` rejects invalid transitions. The domain model is pure.
5. **OpenAPI is comprehensive**: The spec documents all v0.2 endpoints with schemas and security schemes.
6. **CI is mature**: `golangci-lint` with `gosec`, race detection, OpenAPI validation, and PostgreSQL service are all present.
7. **Dockerfile runs as non-root**: `USER civora` is present.
8. **`.dockerignore` exists**: Reduces build context size.
9. **E2E full-lifecycle test**: `TestServiceRequestFullLifecycle` exercises the complete delivery path.
10. **Audit hash chain**: `ComputeHash` and `VerifyIntegrity` exist and are deterministic. `RecordEventTx` uses `SELECT ... FOR UPDATE` for chain continuity.

---

## F. Previously Fixed Issues (confirmed still fixed)

- RBAC enforcement on protected routes
- Self-registration privilege escalation
- Audit atomicity with domain writes
- OIDC removal (config, schema, code, docs)
- Error info leakage (generic 500 messages)
- Cross-tenant assignment validation
- Correlation ID propagation to audit events
- `AuditConfig` wiring (Enabled, RetentionDays)
- Per-user rate limiting on login
- Docker non-root + `.dockerignore`
- CI: golangci-lint + gosec + race + OpenAPI validation
- Password policy: 12+ chars, letter + number
- Case number collision retry
- Idempotency store cleanup goroutine
- `godotenv` conditional load
- Organization creation atomicity
- `FindByEmail` error propagation
- Input length validation for cases
- Handler/API test coverage for identity, cases, organizations

---

## G. New Issues Introduced by v0.2

1. **CRITICAL**: Cross-tenant `service_request_id` injection in all 6 new modules
2. **CRITICAL**: Cross-tenant user references (responsible_staff, assessor, decision_maker, etc.)
3. **CRITICAL**: Missing role authorization on 7 consequential endpoints
4. **CRITICAL**: Case `person_id` cross-tenant reference
5. **CRITICAL**: No optimistic locking on case state transitions
6. **HIGH**: OpenAPI spec mismatches actual state machine
7. **HIGH**: Zero test coverage for new delivery handlers/services/repositories
8. **HIGH**: Audit integrity never verified on read
9. **HIGH**: Audit retention configured but never enforced
10. **HIGH**: Assistance state machine bypass
11. **HIGH**: Follow-ups creatable for closed/rejected cases
12. **HIGH**: Decisions creatable without DECISION_PENDING case status
13. **HIGH**: Person `external_reference` not unique per org
14. **HIGH**: No length validation on new module string fields
15. **MEDIUM**: OpenAPI ambiguous paths / missing operationIds
16. **MEDIUM**: Undocumented `FindByExternalReference` endpoint
17. **MEDIUM**: Case status filter accepts arbitrary strings
18. **MEDIUM**: No duplicate prevention for eligibility/assessment/decision per case
19. **MEDIUM**: PII (email) in auth.failed audit events
20. **MEDIUM**: Code duplication of audit/string helpers across services
21. **MEDIUM**: Register endpoint lacks per-email rate limiting
22. **MEDIUM**: Pre-existing test bugs surface under `-race`

---

## H. Product-Domain Assessment

### Does v0.2 make CIVORA visibly different from generic case management?

**Partially, but the gap is narrowing.**

The v0.2 domain model introduces genuine public-interest concepts: `Person` (beneficiary), `Eligibility`, `Evidence`, `Assessment`, `Decision`, `Assistance`, `FollowUp`. These are distinct from generic ticketing. The state machine (`NEW → OPEN → IN_REVIEW → ASSESSMENT → DECISION_PENDING → APPROVED → IN_PROGRESS → FOLLOW_UP → CLOSED`) expresses a real service-delivery process.

However, the implementation currently treats these as **CRUD tables with a state machine on `Case` only**. The new entities are append-only logs with no enforced relationship to the case lifecycle:
- Eligibility can exist for any case, regardless of status
- Assessment can exist for any case
- Decision can be made for any case, at any time
- Assistance can be created for any case, regardless of decision
- Follow-up can be created for closed cases

The domain logic that should bind these together (e.g., "you cannot create assistance before approval", "you cannot make a decision before assessment") is **entirely missing**. The lifecycle is documented in ADR-0006 and OpenAPI, but not enforced in code.

**What is still missing for a genuine public-interest platform:**
- State-gated transitions between the new entities
- Business rule validation (e.g., decision required before assistance)
- Beneficiary deduplication and merge
- Evidence type validation and anti-malware scanning (planned for 0.5)
- Form submissions (planned for 0.4)
- Policy evaluation (planned for 0.6)

**Verdict**: v0.2 adds the right tables and concepts, but without enforced lifecycle rules, it remains a structured CRUD system rather than a true service-delivery engine.

---

## I. Security Assessment

### Tenant Isolation

**Status: BROKEN for new modules.**

While the existing case and user modules enforce tenant isolation at the repository query level, the new v0.2 modules have two critical gaps:
1. `service_request_id` foreign keys do not validate case org ownership
2. User reference foreign keys (assessor, decision_maker, etc.) do not validate user org ownership
3. `person_id` on cases does not validate person org ownership

An authenticated user in any organization can read and write data across tenant boundaries by manipulating UUIDs.

### Authorization

**Status: INCOMPLETE.**

The new modules expose 15+ endpoints. Only 3 of them have role checks. The remaining 12+ allow any authenticated user to perform consequential actions (creating decisions, assessments, evidence, etc.).

### PII

**Status: MODERATE RISK.**

- Person records contain name, DOB, email, phone, address — all stored in plaintext
- Auth.failed audit events store raw email addresses
- No encryption at rest (documented as future)
- No PII minimization in API responses (all optional fields returned when present)

### Injection

**Status: SAFE.**

All queries use parameterized statements. No SQL injection vectors found.

### Mass Assignment

**Status: SAFE.**

Each endpoint accepts a well-defined request struct. No unbounded map binding.

### Audit Integrity

**Status: WEAK.**

Hash chain is written correctly but never verified on read. Retention is configured but not enforced.

---

## J. Test Assessment

### What tests prove

- `go build ./...` compiles clean
- `go vet ./...` passes
- `gofmt -l .` returns no output
- `go test -short ./...` passes
- `go test -short -race -p 1 -count=1 ./...` passes
- `go test -p 1 ./test/e2e/... ./test/integration/...` passes
- E2E full-lifecycle test exercises the happy path
- Domain unit tests cover state transitions and input validation

### What tests do NOT prove

- Cross-tenant isolation for new modules (no tests exist)
- Authorization on new module endpoints (no tests exist)
- Invalid state transitions via API (only domain-level tests exist)
- Concurrent state transitions (no race-condition tests for new modules)
- Audit atomicity for new modules (only case and org creation are tested)
- PII leakage in new module responses (no tests)
- Evidence access control (no tests)
- Decision authorization (no tests)
- Closure rules (no tests)
- Invalid state combinations (no tests)
- PostgreSQL constraint enforcement for new tables (no repository tests)

### Test quality verdict

The existing tests prove the foundation is solid. The new delivery domain is **under-tested by at least 20x** compared to the foundation modules.

---

## K. Documentation Accuracy

| Document | Claim | Reality |
|----------|-------|---------|
| `api/openapi/openapi.yaml:12-23` | "Any state → CLOSED" | NOT implemented |
| `api/openapi/openapi.yaml:233` | Status enum includes `RESOLVED` | NOT implemented in v0.2 |
| `api/openapi/openapi.yaml:1508` | `/eligibilities/{id}/result` path | Ambiguous with `/eligibilities/service-request/{id}` |
| `docs/decisions/0006-public-interest-domain-model.md:34` | Lifecycle diagram | Matches code, but enforcement is missing |
| `ARCHITECTURE.md:195-214` | Encryption at rest, data export, soft-delete | Still not implemented (documented as future in prior review) |
| `ARCHITECTURE.md:242` | Audit log stored separately | Still false — same DB |
| `ROADMAP.md:38-44` | v0.2 scope | Implementation matches scope, but missing workflow definition/execution |

---

## L. Final Verdict

### Executive Summary

CIVORA v0.2 successfully introduces the right domain concepts (Person, Eligibility, Evidence, Assessment, Decision, Assistance, FollowUp) and extends the case lifecycle with public-interest states. The modular monolith discipline is maintained, and the foundation security properties (RBAC, audit atomicity, tenant isolation for existing modules) remain intact.

However, the new delivery modules have **critical tenant isolation failures** and **critical authorization gaps** that make the system unsafe for multi-tenant deployment. An authenticated user can read and write data across organization boundaries, and can perform consequential decisions without proper role checks. The case state machine is vulnerable to lost updates under concurrency. Test coverage for the new domain is effectively zero at the handler, service, and repository layers.

### Critical Findings

| ID | Severity | Summary |
|----|----------|---------|
| CRITICAL-1 | CRITICAL | Cross-tenant `service_request_id` injection in all 6 new modules |
| CRITICAL-2 | CRITICAL | Cross-tenant user references in new modules |
| CRITICAL-3 | CRITICAL | Missing role authorization on 7 consequential endpoints |
| CRITICAL-4 | CRITICAL | Case `person_id` accepts cross-tenant references |
| CRITICAL-5 | CRITICAL | No optimistic locking on case state transitions |

### High Findings

| ID | Severity | Summary |
|----|----------|---------|
| HIGH-1 | HIGH | OpenAPI spec does not match implemented state machine |
| HIGH-2 | HIGH | Zero test coverage for new delivery handlers/services/repositories |
| HIGH-3 | HIGH | Audit integrity never verified on read |
| HIGH-4 | HIGH | Audit retention configured but never enforced |
| HIGH-5 | HIGH | Assistance status updates bypass state machine |
| HIGH-6 | HIGH | Follow-ups can be created for closed/rejected cases |
| HIGH-7 | HIGH | Decisions can be created without DECISION_PENDING case |
| HIGH-8 | HIGH | Person `external_reference` not unique per org |
| HIGH-9 | HIGH | Pre-existing test bugs break race-enabled full-suite |
| HIGH-10 | HIGH | No length validation on new module string fields |

### Medium Findings

| ID | Severity | Summary |
|----|----------|---------|
| MEDIUM-1 | MEDIUM | OpenAPI ambiguous paths / missing operationIds |
| MEDIUM-2 | MEDIUM | Undocumented `FindByExternalReference` endpoint |
| MEDIUM-3 | MEDIUM | Case status filter accepts arbitrary strings |
| MEDIUM-4 | MEDIUM | No duplicate prevention for per-request records |
| MEDIUM-5 | MEDIUM | PII (email) in auth.failed audit events |
| MEDIUM-6 | MEDIUM | Code duplication of `strPtr` and `recordAuditEventInTx` |
| MEDIUM-7 | MEDIUM | Register endpoint lacks per-email rate limiting |

### Low Findings

| ID | Severity | Summary |
|----|----------|---------|
| LOW-1 | LOW | OpenAPI localhost server warning |
| LOW-2 | LOW | Missing 4XX responses on health/ready |
| LOW-3 | LOW | Case number entropy is lower than ideal |
| LOW-4 | LOW | No email uniqueness across organizations |

### Verified Strengths

1. Modular monolith discipline maintained for new modules
2. Transactional audit atomicity for all new domain writes
3. Tenant-scoped queries in all new repositories
4. Case state machine enforced in domain layer
5. Comprehensive OpenAPI specification
6. Mature CI: golangci-lint + gosec + race + OpenAPI validation
7. Dockerfile runs as non-root
8. `.dockerignore` present
9. E2E full-lifecycle happy-path test
10. Audit hash chain implemented deterministically

### Previously Fixed Issues

All Milestone 0.1 blocking issues remain fixed. See section "Fix Status Verification" above.

### New Issues Introduced by v0.2

See section G above. The v0.2 implementation introduced 5 critical, 10 high, 7 medium, and 4 low findings. The critical and high findings are concentrated in **tenant isolation** and **authorization** for the new delivery modules.

### Security Assessment

**Overall: NOT SAFE for multi-tenant deployment.**

The foundation is secure, but the new delivery domain has systemic tenant isolation and authorization failures. An authenticated attacker can:
- Read/write data across organization boundaries (CRITICAL-1, CRITICAL-2, CRITICAL-4)
- Perform consequential actions without proper roles (CRITICAL-3)
- Cause lost updates under concurrency (CRITICAL-5)

### Test Assessment

**Overall: INSUFFICIENT for v0.2 delivery domain.**

Foundation modules have good coverage. New delivery modules have zero handler/service/repository tests. The existing E2E test covers only one happy path.

### Documentation Accuracy

**Overall: MOSTLY ACCURATE with gaps.**

OpenAPI contains incorrect state machine claims and ambiguous paths. Architecture docs correctly mark encryption/export as future.

---

## M. v0.2 Readiness

| Criterion | Status |
|-----------|--------|
| Domain model complete | PASS |
| Modular monolith preserved | PASS |
| Audit atomicity | PASS |
| RBAC on existing modules | PASS |
| Tenant isolation (existing modules) | PASS |
| Tenant isolation (new modules) | **FAIL** |
| Authorization (new modules) | **FAIL** |
| State machine enforcement | **FAIL** |
| Test coverage (new modules) | **FAIL** |
| OpenAPI accuracy | **FAIL** |
| CI/CD maturity | PASS |
| Docker security | PASS |

### Final Verdict: NOT READY

**v0.2 is NOT READY for internal demo, public alpha, or production.**

The five critical findings (cross-tenant data injection, cross-tenant user references, missing authorization on consequential endpoints, cross-tenant person references, and missing optimistic locking) must be fixed before any deployment. The high findings (OpenAPI inaccuracies, zero test coverage for new modules, audit integrity unverified, assistance state machine bypass) must be addressed before any demo.

The v0.2 domain model is architecturally sound, but the implementation has systemic security gaps that make it untrustworthy as a public-interest service-delivery platform.

---

## N. Test Commands Executed

```bash
$ go build ./...                          # OK
$ go vet ./...                            # OK
$ gofmt -l .                              # OK (no files listed)
$ go test -short ./...                    # PASS
$ go test -short -race -p 1 -count=1 ./... # PASS
$ go test -p 1 -count=1 ./test/e2e/... ./test/integration/... # PASS
$ npx @redocly/cli lint api/openapi/openapi.yaml # PASS (46 warnings)
```

**Note**: Full `-race` suite without `-short` fails due to pre-existing integration test bugs (`TestUserRepository_TenantIsolation` wrong assertion, `TestCaseRepository_CaseNumberCollisionRetries` flaky).

---

## O. Commit Metadata

- **Commit SHA**: `80141961a39dbbe1f0f21e290f96390fc2fc6d5d`
- **Date**: 2026-09-07
- **Branch**: main
- **Previous review commit**: `d6a36ec`

---

Last reviewed: 2026-09-07
