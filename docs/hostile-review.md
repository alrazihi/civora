# Hostile Engineering Review — CIVORA (v2)

**Reviewer**: External senior open-source maintainer (unfamiliar with project history)
**Date**: 2026-09-07
**Commit reviewed**: `8539b80` (HEAD), full history from `3e41e5c`
**Method**: Every finding verified against actual source code. `go build`, `go vet`, `gofmt`, and `go test -short` all run successfully. No claims are speculative.

---

## Fix Status Verification

The previous hostile review (commit `eb354fc`) claimed many findings were "Fixed" in commit `8539b80`. This v2 review independently verifies each claim against the actual current source code.

### Claims verified as ACTUALLY FIXED:

| # | Finding | Status | Evidence |
|---|---------|--------|----------|
| 1 | Error info leakage in 500s | Fixed | `internal/identity/api/handler.go:189-191`, `internal/cases/api/handler.go:267-269`, `internal/organizations/api/handler.go:95-97` all return generic "internal server error" |
| 2 | Cross-tenant assignment (BOLA) | Fixed | `internal/cases/application/service.go:132-140` validates assignee via `UserChecker.BelongsToOrganization` |
| 3 | Two JWT implementations unified | Fixed | Single `JWTService` in `internal/middleware/auth.go` handles both generation and verification |
| 4 | RequireSameTenant bypass | Fixed | `internal/middleware/auth.go:126-129` returns 400 for empty orgId |
| 5 | Password strength enforcement | Fixed | `internal/identity/application/service.go:193-211` enforces 8+ chars, 1 letter + 1 number |
| 6 | Email validation improved | Fixed | `internal/identity/application/service.go:25,186-191` uses regex |
| 7 | CORS configurable | Fixed | `internal/middleware/middleware.go:10-17` reads `CIVORA_SERVER_CORS_ORIGINS` |
| 8 | HSTS conditional on TLS | Fixed | `internal/middleware/headers.go:15-17` checks `r.TLS != nil` |
| 9 | Body size limit | Fixed | `internal/middleware/headers.go:22-34` + `internal/server/server.go:35` |
| 10 | Missing DB indexes | Fixed | `migrations/0001_init.up.sql:67-71` adds all recommended indexes |
| 11 | ReadHeaderTimeout | Fixed | `internal/server/server.go:51` sets `ReadHeaderTimeout: 10 * time.Second` |
| 12 | Health endpoint DB-aware | Fixed | `internal/server/server.go:78-95` pings DB on `/ready` |
| 13 | Empty module scaffolds removed | Fixed | Only 4 modules remain: audit, cases, identity, organizations |
| 14 | DB name consistency | Fixed | `init-test-db.sql` creates `civora_test`, mounted in docker-compose |
| 15 | PII removed from audit metadata | Fixed | No `email` in `user.created` audit event params |
| 16 | Default roles created | Fixed | `internal/identity/domain/user.go:76-101` + `internal/organizations/application/service.go:67-71` |
| 17 | OIDC provider implementation deleted | Fixed | No OIDC files, no OIDC dependencies in go.mod |

### Claims NOT actually fixed (revisited):

| # | Claimed Fixed | Status | Evidence |
|---|--------------|--------|----------|
| 1 | RBAC | **NOT FIXED** | `RequireRole` is defined but NEVER applied to any route. See CRITICAL-1. |
| 4 | Audit hash chain | **NOT FIXED** | Domain write and audit write are in separate transactions. See CRITICAL-3. |
| 26 | RecordEvent test coverage | **NOT FIXED** | `audit_repository_test.go` still uses `Save` + `GetLastHash`, not `RecordEvent` (lines 14-115). |
| 36 | Structured logging | **NOT FIXED** | `log.Printf("audit event recording failed: %v", err)` still in service.go:68-69, 106, 112, 148, 159-160, 175-176. |

---

## A. CRITICAL Findings

### CRITICAL-1: RBAC is non-functional — `RequireRole` is never applied to any route

**File**: `internal/middleware/auth.go:142,159` (definitions); `internal/cases/api/handler.go:25-34`, `internal/identity/api/handler.go:22-33`, `internal/organizations/api/handler.go:22-31`, `internal/audit/api/handler.go:22-28` (route registrations)
**Component**: Authorization
**Problem**: The `RequireRole` and `RequireAnyRole` middleware functions exist, are tested (`internal/middleware/auth_test.go:166-224`), and the JWT `role` claim is populated during authentication. However, **neither is ever registered on any route** in any `RegisterRoutes` method. Every authenticated endpoint uses only `authMiddleware` (verify token) + `RequireSameTenant` (org match). The `role` claim is extracted and stored in context (`auth.go:111`) but **never checked**.

Verification: grep for `RequireRole` in non-test, non-definition files returns zero results. The only callers of `RequireRole` and `RequireAnyRole` are in `auth_test.go`.

**Why it matters**: The threat model (T-01, T-02), ARCHITECTURE.md ("Authorization checked at the service and data layers"), ADR-0001 ("All endpoints require authentication and authorization"), and `docs/architecture/api-spec.md:32-34` ("the `RequireRole` middleware checks the JWT's `role` claim") all claim role-based authorization. Every one of these claims is **false**. Any authenticated user — including one who self-registered without specifying a role (getting JWT role `"user"`) — can create cases, transition case statuses, assign cases, list all users, and read the full audit log. This is a total authorization bypass.

The prior review claimed this was "Fixed" by creating default roles. Role creation is only half the story. Roles that are created but never checked are security theater.

**Recommended fix**: Apply `RequireRole("admin")` or `RequireRole("staff")` to specific routes that require it. Without this, every authenticated user is a superuser.
**Severity**: CRITICAL

### CRITICAL-2: Unauthenticated registration can self-assign the "admin" role

**File**: `internal/identity/api/handler.go:35-72`; `internal/identity/application/service.go:78-83`
**Component**: Authentication / Authorization
**Problem**: The `Register` endpoint accepts `role_name` from the client request body and passes it to `CreateUser`. `CreateUser` looks up the role by name within the organization and assigns it to the new user if it exists. Since `DefaultRoleCreator` creates an `"admin"` role on organization creation, any attacker who knows an organization ID can register a user with `role_name: "admin"`. The OpenAPI spec documents `role_name` as accepting `"admin"` or `"staff"` with no access-control caveat.
**Why it matters**: The threat model (T-02: "Privilege escalation") explicitly lists this concern. The registration endpoint has no authentication — it is the only onboarding path — yet it trusts the client to specify the user's role.
**Recommended fix**: Remove `role_name` from public registration. Implement first-user-is-admin logic: the first user registered for an organization gets `"admin"`, all subsequent users get `"staff"`.
**Severity**: CRITICAL

### CRITICAL-3: Audit events are not written atomically with domain writes

**File**: `internal/cases/application/service.go:55-70` (domain write, then audit); `internal/identity/application/service.go:93-110`; `internal/organizations/application/service.go:63-83`; `internal/audit/application/service.go:22-50`; `internal/audit/infrastructure/postgres/audit_repository.go:22-80`
**Component**: Audit integrity / Transaction boundaries
**Problem**: Every service method follows the pattern: (1) save the domain entity to the DB in one transaction (`s.repo.Save(ctx, c)` commits), (2) then call `auditor.RecordEvent()` in a **separate** transaction. If step 2 fails (DB error, connection pool exhaustion), the error is logged via `log.Printf` and the method returns `nil` (success). The domain write in step 1 is already committed. Result: the data change exists but the audit trail is permanently incomplete.

The prior review marked this as "Documented" — it is not fixed. The `RecordEvent` repository method does begin its own transaction with `SELECT ... FOR UPDATE` to maintain the hash chain (audit_repository.go:23-42), but this is a *separate* transaction from the domain write, so there is no atomicity guarantee.

**Why it matters**: Audit integrity is CIVORA's central security guarantee. The threat model (T-09: "Audit tampering") rates this **Critical**. If a domain write succeeds but its audit event fails, an attacker could perform actions that leave no trace. The `VerifyIntegrity()` method checks hash correctness but cannot detect missing events.

**Recommended fix**: Implement a transactional outbox pattern — write the domain entity and the audit event in the **same** database transaction using `database.InTransactional` (`internal/database/db.go:58`). If the audit write fails, roll back the domain write.
**Severity**: CRITICAL

### CRITICAL-4: OIDC config and schema remnants remain after "removal"

**File**: `internal/config/config.go:42-44,77-79`; `internal/identity/domain/user.go:20`; `migrations/0001_init.up.sql:27`; `.env.example:25-28`; `docs/architecture/configuration.md:37-40`; `SECURITY.md:54` (references threat model)
**Component**: Authentication / Dead code / Documentation accuracy
**Problem**: The prior review (#2) claimed OIDC was "Fixed: Removed (interface, provider implementation, all imports)." Verification shows this is **partially true**: the `OIDCProvider` interface and `internal/identity/infrastructure/auth/oidc/provider.go` were deleted, and no OIDC libraries are in `go.mod`/`go.sum` (confirmed: grep returns zero results). However, the removal is **incomplete**:
- `config.go` still has `OIDCIssuer`, `OIDCClientID`, `OIDCRedirectURL` fields loaded from env (lines 77-79) but never read (confirmed: zero references outside config.go)
- `.env.example` still documents `CIVORA_AUTH_OIDC_ISSUER`, `CIVORA_AUTH_OIDC_CLIENT_ID`, `CIVORA_AUTH_OIDC_REDIRECT_URL` (lines 25-28)
- `docs/architecture/configuration.md` still documents these as active config variables (lines 37-40)
- The `users` table still has `is_oidc_user BOOLEAN NOT NULL DEFAULT FALSE` (migration line 27)
- The `User` struct still has `IsOIDCUser bool` (user.go:20), set on every insert/query but never used in logic
**Why it matters**: Operators who set OIDC env vars will find them silently ignored. The config and schema remnants create a false impression of OIDC capability.
**Recommended fix**: Remove all OIDC config fields, env vars, schema columns, struct fields, and documentation references.
**Severity**: CRITICAL

### CRITICAL-5: No encryption at rest for personal data — directly contradicts documentation

**File**: `ARCHITECTURE.md:195-214`; all repository files (no encryption code exists)
**Component**: Security / Data protection / Documentation accuracy
**Problem**: ARCHITECTURE.md claims (lines 195-214):
- "Personal data is identified by a `data_classification` label" — no such column or mechanism exists
- "Personal data is stored encrypted at rest where configured"
- "Encryption at rest is supported via database column-level encryption for personal data"
- "Encryption keys are managed by the operator's key management system (KMS) where available"

All PII (emails, names, case titles, descriptions) is stored in **plaintext** in PostgreSQL. Only passwords are hashed.
**Recommended fix**: Remove all encryption-at-rest claims from documentation until implemented.
**Severity**: CRITICAL

### CRITICAL-6: Documented data export and soft-delete features do not exist

**File**: `ARCHITECTURE.md:206,224-228`; `api/openapi/openapi.yaml`
**Component**: Documentation accuracy / Data management / Privacy
**Problem**: ARCHITECTURE.md claims:
- "Data export is available in JSON format via the API" (line 226) — no export endpoints exist
- "Bulk export endpoints are available for administrators" (line 227) — no bulk export endpoints exist
- "Deleted data is soft-deleted by default" (line 206) — no soft-delete mechanism. No `deleted_at` column, no `is_deleted` flag, no deletion API. All FKs use `ON DELETE CASCADE` (hard delete).
**Recommended fix**: Remove all claims about export, soft-delete, and data portability from documentation. Mark as future work in ROADMAP.md.
**Severity**: CRITICAL

### CRITICAL-7: Stale CONTRIBUTING.md with pervasive placeholder text

**File**: `CONTRIBUTING.md:56-57,76-77,123-124,147-148`
**Component**: Documentation accuracy
**Problem**: CONTRIBUTING.md contains four separate paragraphs of placeholder text claiming the build system, test commands, and ADR template "will be created" — all of which already exist in `AGENTS.md` and `docs/decisions/0000-template.md`.
**Severity**: CRITICAL

---

## B. HIGH Findings

### HIGH-1: No event bus exists — ARCHITECTURE.md and ADR-0001 describe one

**File**: `ARCHITECTURE.md:37,53,76-83`; `docs/decisions/0001-initial-architecture.md:64-67`
**Problem**: ARCHITECTURE.md line 37 lists "Events (in-process)" as shared infrastructure. ADR-0001 lines 64-67 describe "in-process event bus or pub/sub pattern for cross-module notifications." **No event bus exists.** All cross-module communication is synchronous direct function calls.
**Severity**: HIGH

### HIGH-2: Organization creation and role creation are not atomic

**File**: `internal/organizations/application/service.go:63-71`
**Problem**: `CreateOrganization` saves the org (line 63), then creates default roles in a separate transaction (lines 67-70). If role creation fails, the error is logged and the org is returned successfully — **without any roles**. The org exists but is broken.
**Severity**: HIGH

### HIGH-3: `CreateUser` silently discards errors from `FindByEmail` and role lookup

**File**: `internal/identity/application/service.go:72,77-83`
**Problem**: `existing, _ := s.userRepo.FindByEmail(...)` (line 72) — error discarded. If the email-uniqueness query fails, the method proceeds to create a duplicate user, causing a DB error. Role lookup failures are silently ignored: if `role_name` doesn't exist, `roleID` is silently nil (lines 78-83).
**Severity**: HIGH

### HIGH-4: Correlation ID (`X-Request-ID`) never propagated to audit events

**File**: `internal/middleware/logging.go:28,35-45` (generation); all application service audit calls
**Problem**: The `RequestID` middleware generates a UUID and stores it in context. The `Logging` middleware uses it. But **no handler or service ever calls `GetRequestID(r)`** to pass it to `AuditService.RecordEvent`. Every audit event has `RequestID = nil`. `docs/architecture/api-spec.md:41-43` claims correlation IDs are "included in all audit events."
**Severity**: HIGH

### HIGH-5: `AuditConfig` fields are dead configuration

**File**: `internal/config/config.go:47-51,81-84`; `internal/audit/application/service.go`
**Problem**: `Enabled`, `HashChainEnabled`, `RetentionDays` are loaded from env but **never passed to `AuditService`** and never checked. `CIVORA_AUDIT_ENABLED=false` is silently ignored. `CIVORA_AUDIT_RETENTION_DAYS` is silently ignored — old events are never purged. Confirmed: grep for `Enabled`, `HashChainEnabled`, `RetentionDays` outside config.go returns zero results.
**Severity**: HIGH

### HIGH-6: `ListUsers` API has no pagination — OpenAPI documents it

**File**: `internal/identity/api/handler.go:111-131`; `internal/identity/application/service.go:221-223`; `api/openapi/openapi.yaml:508-520`
**Problem**: The handler and service return all users with no `limit`/`offset`. The OpenAPI spec documents `page` and `per_page` query parameters that are not implemented.
**Severity**: HIGH

### HIGH-7: No `.dockerignore` file

**File**: Repository root (missing)
**Problem**: No `.dockerignore` exists. `Dockerfile` does `COPY . .` sending entire repo (including `.git/`, `docs/`, test binaries) as build context.
**Severity**: HIGH

### HIGH-8: Dockerfile runs as root

**File**: `Dockerfile:12-13`
**Problem**: No `USER` directive. Container runs as root.
**Severity**: HIGH

### HIGH-9: No static analysis beyond `gofmt` and `go vet` in CI

**File**: `.github/workflows/ci.yml:14-29`
**Problem**: Only `gofmt` and `go vet`. No gosec, golangci-lint, or govulncheck.
**Severity**: HIGH

### HIGH-10: No per-user rate limiting on authentication endpoints

**File**: `internal/middleware/ratelimit.go:22-29`; `internal/identity/api/handler.go:74-90`
**Problem**: Rate limiting is per-IP only (100 req/s, burst 20). No per-user rate limiting on login/register. Brute-force attacks against specific users are possible. Threat model T-04 says "Rate limiting on authentication endpoints."
**Severity**: HIGH

### HIGH-11: No data export or soft-delete — ARCHITECTURE.md claims both exist

**File**: `ARCHITECTURE.md:206,224-228`
**Problem**: (See CRITICAL-6) Documented export and soft-delete features do not exist.
**Severity**: HIGH

### HIGH-12: Domain-model gap — no "Person" (beneficiary) entity

**File**: `internal/cases/domain/case.go:28`; `ARCHITECTURE.md:97-100`
**Problem**: ARCHITECTURE.md distinguishes `Person` (beneficiary) from `User` (authenticated staff). `Case.CreatedByID` is a `User`. There is no `Person` entity.
**Severity**: HIGH

---

## C. MEDIUM Findings

### MEDIUM-1: `fmt.Sprintf("case.transition")` — no-op format string

**File**: `internal/cases/application/service.go:103`
**Problem**: `fmt.Sprintf("case.transition")` is identical to the literal `"case.transition"`. Never fixed despite being flagged in the prior review (#19).
**Severity**: MEDIUM

### MEDIUM-2: `shared/validator.go` and `shared/id.go` are entirely dead code

**File**: `internal/shared/validator.go:1-64`; `internal/shared/id.go:1-16`
**Problem**: All functions in both files are never called from any source file (confirmed by grep). `shared.ValidateEmail` uses weak `strings.Contains` validation (the vulnerability the prior review #8 flagged). `database.SplitDSN` (`internal/database/db.go:78-88`) is also dead code.
**Severity**: MEDIUM

### MEDIUM-3: Password policy weaker than documented

**File**: `internal/identity/application/service.go:27,193-211`; `docs/threat-model.md:98-104`
**Problem**: Implementation: 8 chars, 1 letter + 1 number. Threat model: "minimum 12 characters." Prior review recommended 12. Currently 8.
**Severity**: MEDIUM

### MEDIUM-4: No race detection in CI

**File**: `.github/workflows/ci.yml:64,67`
**Problem**: `CGO_ENABLED=0 go test` cannot use `-race` flag (requires CGO). Concurrent code (RateLimiter, IdempotencyStore) is untested for data races.
**Severity**: MEDIUM

### MEDIUM-5: Idempotency store memory leak

**File**: `internal/middleware/idempotency.go:48-57`
**Problem**: `Cleanup()` method exists but is never called. No background goroutine. Map grows unboundedly.
**Severity**: MEDIUM

### MEDIUM-6: JWT does not validate `iss` (issuer) claim

**File**: `internal/middleware/auth.go:57-86`
**Problem**: `VerifyToken` validates signature and `exp` but not `iss`. Tokens from other services with the same secret would be accepted.
**Severity**: MEDIUM

### MEDIUM-7: Case number generation has collision risk

**File**: `internal/cases/domain/case.go:109-111`
**Problem**: UUID truncated to 8 hex chars (32 bits of entropy). `case_number_organization` UNIQUE constraint means collision = hard DB error (500), not retry.
**Severity**: MEDIUM

### MEDIUM-8: `Recovery` middleware constructs JSON with `fmt.Sprintf`

**File**: `internal/middleware/recovery.go:14`
**Problem**: Manual JSON construction via `fmt.Sprintf` — no escaping of special characters. Could produce invalid JSON on panics with special chars in the message.
**Severity**: MEDIUM

### MEDIUM-9: No test coverage for handler/API layers

**File**: All `internal/*/api/handler.go` files — zero `*_test.go` files
**Problem**: No unit tests for any HTTP handler. Identity application coverage is 27.8%. The `shared` package, `database` package, `config` package, and `server` package all have 0% coverage.
**Severity**: MEDIUM

### MEDIUM-10: Audit repository test uses old `Save`/`GetLastHash` API, not `RecordEvent`

**File**: `internal/audit/infrastructure/postgres/audit_repository_test.go`
**Problem**: Tests (lines 14-115) use `repo.Save` and `repo.GetLastHash` — the old API. The production code path uses `repo.RecordEvent` (audit_repository.go:22-80). The `RecordEvent` method has **zero test coverage**.
**Severity**: MEDIUM

### MEDIUM-11: Organization creation does not record an audit event

**File**: `internal/organizations/application/service.go:73-83`
**Problem**: `CreateOrganization` records an audit event (`organization.created`), but if `RecordEvent` fails, the error is swallowed (`log.Printf`). The organization was already committed to DB in a prior step (line 63). This is the same transaction-boundary issue as CRITICAL-3.
**Severity**: MEDIUM

### MEDIUM-12: No token revocation / logout endpoint

**File**: `internal/identity/api/handler.go` (no logout route); `api/openapi/openapi.yaml`
**Problem**: No logout endpoint exists. JWTs are valid until expiry (24h). Threat model T-04 says "Session tokens are random, short-lived, and **revocable**."
**Severity**: MEDIUM

---

## D. LOW Findings

### LOW-1: Unused OpenAPI schemas (`SuccessResponse`, `PaginationMeta`)

**File**: `api/openapi/openapi.yaml:82-104`
**Severity**: LOW

### LOW-2: No `operationId` in OpenAPI spec

**File**: `api/openapi/openapi.yaml`
**Severity**: LOW

### LOW-3: `RequireAnyRole` is redundant with `RequireRole`

**File**: `internal/middleware/auth.go:159-178`
**Problem**: Identical logic to `RequireRole`. Never applied to any route.
**Severity**: LOW

### LOW-4: `docs/architecture/README.md` is stale

**File**: `docs/architecture/README.md:9`
**Problem**: Shows empty table "(will be populated)" but `api-spec.md` and `configuration.md` exist.
**Severity**: LOW

### LOW-5: `CONTRIBUTING.md` line 148 references ADR template "to be created"

**File**: `CONTRIBUTING.md:148`
**Severity**: LOW

### LOW-6: ARCHITECTURE.md claims SQLite is supported — it is not

**File**: `ARCHITECTURE.md:193,259,293`; `docs/decisions/0001-initial-architecture.md:71-72`
**Problem**: No SQLite driver import, PostgreSQL-specific DSN format and SQL types.
**Severity**: LOW

### LOW-7: ARCHITECTURE.md claims backup strategy is documented

**File**: `ARCHITECTURE.md:216-222`
**Problem**: No backup documentation exists anywhere.
**Severity**: LOW

### LOW-8: ARCHITECTURE.md claims audit log is stored separately from operational data

**File**: `ARCHITECTURE.md:198,242`
**Problem**: Audit events are in the same PostgreSQL database and same tables.
**Severity**: LOW

### LOW-9: `auth_test.go` test `TestRequireSameTenant_BlocksWhenPathOrgIDEmpty` may be a false positive

**File**: `internal/middleware/auth_test.go:149-164`
**Problem**: Registers route `/test/{orgId}`, sends request to `/test/`. Chi's `{orgId}` matches `[^/]+` (requires 1+ chars). An empty segment may not match, causing a 404 instead of the expected 400. The test assertion expects 400. If chi returns 404, the test would fail; if it passes, it's unclear whether the middleware was actually exercised.
**Severity**: LOW

### LOW-10: `godotenv` loaded unconditionally in production config

**File**: `internal/config/config.go:54`
**Problem**: `godotenv.Load(".env")` is called on every startup. In production, a accidentally deployed `.env` file would silently override environment variables.
**Severity**: LOW

---

## E. Architecture Concerns

### E-1: Audit domain imported directly by all application services

**File**: `internal/cases/application/service.go:11`; `internal/identity/application/service.go:11`; `internal/organizations/application/service.go:10`
**Problem**: Three modules import `internal/audit/domain` for `EventRecorder` and `RecordEventParams`. This creates tight coupling — audit is a cross-cutting concern that all modules depend on directly. ADR-0001 states modules should communicate through "well-defined interfaces" and "events," but this is direct synchronous calls.

### E-2: Config has no validation beyond JWT secret

**File**: `internal/config/config.go:88-94`
**Problem**: Only `CIVORA_AUTH_JWT_SECRET` is validated in production. DB credentials default to `civora`/`civora`/`localhost` with no validation.

### E-3: Three disconnected role concepts

**File**: `internal/identity/domain/role.go` (Role entity with Permissions); `internal/middleware/auth.go` (JWT role claim); `internal/identity/domain/role.go:29` (HasPermission)
**Problem**: DB Role entity with permissions, JWT string claim, and `HasPermission` method exist but are never reconciled. `RequireRole` checks the JWT string name, not the `HasPermission` method. No permission-based authorization exists.

### E-4: Idempotency middleware applied globally

**File**: `internal/server/server.go:34`
**Problem**: `IdempotencyKey` applied to all routes including auth and org creation. In-memory store is not shared across instances.

---

## F. Security Concerns

### F-1: No CSRF risk (positive finding)

**File**: N/A
**Problem**: Not a problem — the API uses Bearer tokens in `Authorization` header, not cookies. CSRF is not applicable. The prior review correctly noted this.

### F-2: SQL injection not possible

**File**: All repository files
**Problem**: Not a problem — all queries use parameterized `$N` placeholders. No string concatenation in SQL. Verified.

---

## G. Product/Mission Concerns

### G-1: CIVORA is indistinguishable from a generic case management system

**File**: Entire codebase; `ARCHITECTURE.md:101-132`; `docs/vision.md`
**Problem**: After reviewing all code, there is nothing specific to "Emergency Assistance Request" or public-interest service delivery. The case lifecycle (CREATED → OPEN → IN_REVIEW → RESOLVED → CLOSED) could describe any ticketing system. Features described in the vision — workflow definitions, form submissions, evidence chains, policy evaluation, AI assistance, beneficiary management, follow-ups — are absent. There are no case types, no multilingual forms, no beneficiary entity, no evidence/document model.

The system is a generic CRUD API with JWT auth and audit logging. The "first vertical slice (Emergency Assistance Request)" described in the OpenAPI spec is just "create a case, change its status, assign it."

**Severity**: HIGH

### G-2: No localization / internationalization support exists

**File**: `docs/vision.md:207-222`; `ARCHITECTURE.md`
**Problem**: Vision claims "All user-facing strings are externalized," "36 languages," "locales loaded dynamically." No i18n infrastructure exists. Case numbers use English prefix `CAS-`.
**Severity**: MEDIUM

---

## H. What Is Genuinely Strong

1. **Modular monolith structure** — Clean domain/application/infrastructure/api layers. Acyclic dependency graph. 7 direct dependencies in go.mod.
2. **Domain model purity** — `Case` entity encapsulates state transitions. 92.3% coverage in `case_test.go`.
3. **Audit hash chain concept** — `ComputeHash()` is deterministic. `VerifyIntegrity()` detects tampering. Repository uses `SELECT ... FOR UPDATE` for chain continuity.
4. **OpenAPI specification** — Comprehensive, standardized envelopes, pagination, security schemes.
5. **Tenant isolation at data layer** — Every query includes `organization_id`. FK constraints enforce org scoping. Cross-tenant tests pass.
6. **Structured JSON logging** — Consistent log format with request ID, latency, status.
7. **Custom migration runner** — `go:embed` for SQL files, transactional execution, `schema_migrations` table.
8. **Threat model** — 13 threat categories with concrete mitigations.
9. **ADR process** — 5 ADRs with alternatives, trade-offs, reversibility.
10. **Security headers + body size limit + ReadHeaderTimeout + DB-aware readiness** — All implemented and tested.
11. **Cross-tenant assignment validation** — `AssignCase` validates assignee via `UserChecker`.
12. **Build compiles, go vet clean, gofmt clean, all short tests pass.**

---

## I. What Should Be Removed

1. **`internal/shared/validator.go`** — Entirely dead code. Weak `ValidateEmail` uses `strings.Contains`.
2. **`internal/shared/id.go`** — `NewID`, `ParseID`, `IsValidUUID` never called.
3. **`database.SplitDSN`** (`internal/database/db.go:78-88`) — Never called.
4. **`RequireAnyRole`** (`internal/middleware/auth.go:159-178`) — Redundant with `RequireRole`. Never applied.
5. **Unused OpenAPI schemas** — `SuccessResponse`, `PaginationMeta` (openapi.yaml:82-104).
6. **OIDC config fields** — `OIDCIssuer`, `OIDCClientID`, `OIDCRedirectURL` in config.go. Never read.
7. **`IsOIDCUser` field** — `user.go:20` and `users.is_oidc_user` column. Never used in logic.
8. **`Save` and `GetLastHash` on `AuditRepository`** — Only used in tests. Production uses `RecordEvent`.
9. **`docs/architecture/README.md` placeholder row** — Line 9 claims empty directory.
10. **`CONTRIBUTING.md` stale placeholder text** — Lines 56, 76, 123, 147.
11. **`fmt.Sprintf("case.transition")`** — Use literal string (service.go:103).

---

## J. What Should Be Redesigned

1. **Authorization model** — Implement and apply `RequireRole` to routes. Distinguish admin vs staff vs user capabilities. Remove `role_name` from self-registration.
2. **Audit transaction boundaries** — Transactional outbox: domain write + audit event in same transaction.
3. **Organization creation** — Wrap org save + role creation in single transaction.
4. **Centralized error handling** — Replace per-handler `writeDomainError`/`writeOrgError`/`writeCaseError` with shared error-to-HTTP mapper. Standardize on `errors.Is`.
5. **CORS configuration** — Already done. Acceptable.
6. **Rate limiting** — Add per-user limits. Separate limits for auth endpoints.
7. **Case model** — Add `Person` (beneficiary) entity separate from `User` (staff).
8. **Request body size limiting** — Already done. Acceptable.
9. **Health/readiness checks** — Already done. Acceptable.
10. **Email validation** — Already done with regex. Remove dead `shared.ValidateEmail`.
11. **Correlation ID propagation** — Pass `X-Request-ID` from middleware through handlers to `RecordEventParams.RequestID`.
12. **Audit enable/retention** — Pass `AuditConfig` to `AuditService`. Enforce `Enabled` and `RetentionDays`.

---

## K. What MUST Be Fixed Before Milestone 0.1

These are **blocking** issues:

| # | Issue | Category |
|---|-------|----------|
| 1 | **RBAC non-functional** — `RequireRole` never applied to routes. All authenticated users are superusers. | CRITICAL |
| 2 | **Self-registration as admin** — `role_name` accepted from unauthenticated client | CRITICAL |
| 3 | **Audit not atomic with domain writes** — separate transactions, errors swallowed | CRITICAL |
| 4 | **OIDC config/schema remnants** — dead config fields, unused DB column, stale docs | CRITICAL |
| 5 | **No encryption at rest** — docs claim it, code doesn't implement it | CRITICAL |
| 6 | **No data export or soft-delete** — docs claim both, neither exists | CRITICAL |
| 7 | **Stale CONTRIBUTING.md** — placeholder text where commands exist | CRITICAL |
| 8 | **No `.dockerignore`** | HIGH |
| 9 | **Dockerfile runs as root** | HIGH |
| 10 | **Unauthenticated `/metrics`** | HIGH |
| 11 | **No static analysis in CI** (gosec/golangci-lint) | HIGH |
| 12 | **Correlation ID not in audit events** | HIGH |
| 13 | **`AuditConfig` dead config** — Enabled, HashChainEnabled, RetentionDays unused | HIGH |
| 14 | **No per-user rate limiting on auth endpoints** | HIGH |
| 15 | **Dead code** (`shared/validator.go`, `shared/id.go`, `SplitDSN`, `RequireAnyRole`, unused OpenAPI schemas) | HIGH |
| 16 | **`log.Printf` for audit errors** — not structured logging | MEDIUM |
| 17 | **`godotenv` unconditional load** | LOW |
| 18 | **No race detection in CI** | MEDIUM |
| 19 | **Event bus claims in ARCHITECTURE.md** | HIGH |
| 20 | **`docs/architecture/README.md` stale** | LOW |
| 21 | **SQLite claims in docs** | LOW |
| 22 | **`RecordEvent` has no test coverage** | MEDIUM |
| 23 | **`fmt.Sprintf("case.transition")` no-op** | MEDIUM |

---

## L. What Can Wait Until Later

1. Full i18n/l10n — Not needed until frontend (Milestone 0.3+).
2. OIDC/OAuth2 provider — Can wait. But remove config remnants now.
3. Structured audit integrity verification on read — `VerifyIntegrity()` exists; call it when serving audit data.
4. Prometheus metrics exporter — Secure the `/metrics` stub or remove it now. Replace with real metrics later.
5. OpenTelemetry tracing — Not needed for 0.1.
6. Data portability/export API — Milestone 0.6. Remove claims now.
7. Case reopening after closure — Milestone 0.2.
8. Form submissions — Milestone 0.4.
9. Evidence/documents — Milestone 0.5.
10. AI assistance — Milestone 0.7.
11. Token revocation / logout endpoint — Document as limitation; implement in later milestone.
12. `operationId` in OpenAPI — Add when generating SDKs.
13. Password breach checking (HIBP API) — Later.
14. Person entity — Milestone 0.2/0.3.
15. Decision entity, Comment entity — Milestone 0.3+.
16. Backup strategy documentation — Operator responsibility; remove claim.
17. JWT key rotation — Later.

---

## Test Results Summary

| Test Suite | Status | Coverage |
|---|---|---|
| `internal/cases/domain` | Pass | 92.3% |
| `internal/cases/application` | Pass | 64.9% |
| `internal/cases/infrastructure/postgres` | Pass (DB) | — |
| `internal/audit/domain` | Pass | 97.0% |
| `internal/audit/infrastructure/postgres` | Pass (DB) | — |
| `internal/identity/application` | Pass | 27.8% |
| `internal/identity/infrastructure/postgres` | Pass (DB) | — |
| `internal/organizations/application` | Pass | 48.3% |
| `internal/organizations/infrastructure/postgres` | Pass (DB) | — |
| `internal/middleware` | Pass | 60.3% |
| `test/e2e` | Pass (DB) | — |
| `test/integration` | Pass (DB) | — |
| `internal/shared` | No tests | 0.0% |
| `internal/database` | No tests | 0.0% |
| `internal/config` | No tests | 0.0% |
| `internal/server` | No tests | 0.0% |
| `internal/*/api` (handlers) | No tests | 0.0% |

**Total**: ~50 tests, all passing (when PostgreSQL is available and `-p 1` is used). Handler layer, shared utilities, config, database, and server packages are entirely untested.

---

## Proposed Corrected Milestone 0.1 Scope

The current implementation **exceeds** the founder's Milestone 0.1 requirements in some areas (security headers, body size limit, rate limiting, structured logging, DB-aware health checks, migration system) and **falls critically short** in others:

**Before 0.1 can be declared complete, the following MUST be done:**

1. **Implement and apply RBAC** — Register `RequireRole` on all routes that require authorization. Not just create roles — actually enforce them.
2. **Fix registration privilege escalation** — Remove `role_name` from public registration. Implement first-user-is-admin.
3. **Fix audit transaction boundaries** — Use transactional outbox pattern. Domain writes and audit events in the same transaction.
4. **Remove OIDC remnants** — Delete config fields, `.env.example` lines, `IsOIDCUser` field, `is_oidc_user` column. Update docs.
5. **Correct security documentation** — Remove claims about encryption at rest, data export, soft-delete, event bus, SQLite, backup strategy, separate audit storage. Either implement or document as future.
6. **Propagate correlation IDs to audit events** — Pass `X-Request-ID` from middleware context to `RecordEventParams.RequestID`.
7. **Wire up `AuditConfig`** — Pass config to `AuditService`. Enforce `Enabled` and `RetentionDays`.
8. **Remove all dead code** — `shared/validator.go`, `shared/id.go`, `database.SplitDSN`, `RequireAnyRole`, unused OpenAPI schemas, `SuccessResponse`/`PaginationMeta`.
9. **Fix `CONTRIBUTING.md`** — Remove all placeholder text. Reference actual AGENTS.md commands.
10. **Create `.dockerignore`** and add non-root user to Dockerfile.
11. **Secure or remove `/metrics`** endpoint.
12. **Add `golangci-lint` with `gosec`** to CI.
13. **Fix `fmt.Sprintf("case.transition")`** to literal string.
14. **Add test coverage for handlers** and the `RecordEvent` repository method.
15. **Add `-race` detection** to CI (requires CGO_ENABLED=1).
16. **Fix organization creation atomicity** — org save + role creation in same transaction.
17. **Stop discarding `FindByEmail` errors** in `CreateUser`.
18. **Add input length validation** for case title/description.

**After 0.1 is corrected, the following can be deferred:**

- Full i18n/l10n (until frontend)
- OIDC provider implementation (when requested)
- Prometheus metrics (replace stub later)
- OpenTelemetry tracing
- Data export API (Milestone 0.6)
- Case reopening after closure (Milestone 0.2)
- Form submissions (Milestone 0.4)
- Evidence/documents (Milestone 0.5)
- AI assistance (Milestone 0.7)
- Token revocation/logout (document as limitation)
- Person entity, Decision entity, Comment entity (Milestone 0.3+)
- Password breach checking
- JWT key rotation

---

Last reviewed: 2026-09-07