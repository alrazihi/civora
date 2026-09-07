# Hostile Engineering Review — CIVORA (Pre-Milestone 0.1)

**Reviewer**: External senior open-source maintainer (unfamiliar with project history)
**Date**: 2026-09-07
**Commit reviewed**: `6ae018b` (latest), with full history from `3e41e5c`

---

## Fix Status

The following **blocking** findings were addressed before completion of this review:

| # | Finding | Status | Details |
|---|---|---|---|
| 1 | RBAC non-functional | **Fixed** | Default roles (`admin`, `staff`) created on org creation via `DefaultRoleCreator` |
| 2 | OIDC dead code | **Fixed** | `OIDCProvider` interface removed; `provider.go` deleted; import removed from `go.mod` |
| 3 | Cross-tenant BOLA | **Fixed** | `AssignCase` validates assignee belongs to org via `UserChecker` |
| 4 | Audit hash chain | **Documented** | Transactional outbox recommended for future; current pattern logs errors |
| 5 | Error info leakage | **Fixed** | All 5 handler `default:` cases now return generic "internal server error" |
| 6 | RequireSameTenant bypass | **Fixed** | Empty `orgId` param now returns 400, not pass-through |
| 7 | Two JWT implementations | **Fixed** | Unified into single `JWTService` in middleware; `JWTTokenService` removed |
| 8 | Password strength | **Fixed** | Minimum 8 chars, requires ≥1 letter and ≥1 number |
| 9 | Email validation | **Fixed** | Regex-based validation replacing `strings.Contains` |
| 10 | CORS all origins | **Fixed** | Origins now configurable via `CIVORA_SERVER_CORS_ORIGINS`, defaults to `http://localhost:3000` |
| 11 | HSTS on HTTP | **Fixed** | Only set when `r.TLS != nil` |
| 12 | No body size limit | **Fixed** | `BodySizeLimit()` middleware limits to 10 MB via `http.MaxBytesReader` |
| 13 | PII in audit metadata | **Fixed** | `email` removed from `user.created` audit event metadata |
| 14 | Missing DB indexes | **Fixed** | Added `idx_cases_assigned_to`, `idx_users_role_id`, `idx_audit_resource_id` |
| 15 | No ReadHeaderTimeout | **Fixed** | Added `ReadHeaderTimeout: 10s` to `http.Server` |
| 16 | Health doesn't check DB | **Fixed** | `/ready` now pings database, returns 503 if unavailable |
| 17 | Empty module scaffolds | **Removed** | Deleted `ai`, `tasks`, `workflow`, `forms`, `documents`, `notifications`, `integrations`, `policy` |
| 18 | Docker-compose DB sync | **Fixed** | Added `init-test-db.sql` to create `civora_test` user and database |

**Tests**: All unit tests pass (`go test -short ./...`). Integration and e2e tests require PostgreSQL (see DB env vars in `AGENTS.md`).

---

## A. CRITICAL Findings

### 1. RBAC is completely non-functional
**File**: `internal/identity/application/service.go:149-155`
**Component**: Authentication / Authorization
**Problem**: No default roles (admin, staff) are created when an organization is created (`OrganizationService.CreateOrganization` at `internal/organizations/application/service.go:39`). When `Authenticate` runs, `user.RoleID` is typically nil because `CreateUser` silently sets `RoleID = nil` if the named role doesn't exist (`internal/identity/application/service.go:71-77`). The fallback sets `role = "user"` — but no `"user"` role exists in the database. The JWT `role` claim is always `"user"`. `RequireRole("admin")` will **never** match. Any future permission check on roles is dead code.
**Why it matters**: Claims of RBAC in documentation are false. Security is theater.
**Recommended fix**: Create default roles (admin, staff) in `CreateOrganization` with a transactional outbox, or define role names as constants in the domain and validate against them in the JWT.

### 2. OIDC provider has no implementation
**File**: `internal/identity/infrastructure/auth/oidc/provider.go`
**Component**: Authentication
**Problem**: `redirectURI()` returns `""` (hardcoded empty string). `VerifyToken` is defined as an interface method but has no concrete implementation anywhere. The provider is never instantiated in `main.go`. The `OIDCProvider` interface in `identity/domain/repository.go:35` is dead code.
**Why it matters**: The threat model, ADR-0004, and ARCHITECTURE.md all reference OIDC support. It does not exist.
**Recommended fix**: Either remove the OIDC scaffolding entirely (avoid false claims), or implement a working provider with testable discovery and token verification.

### 3. Cross-tenant assignment vulnerability (BOLA)
**File**: `internal/cases/api/handler.go:194-206, 208-213`
**Component**: Case management
**Problem**: `AssignCase` accepts a `UserID` from the request body (`req.UserID`) and passes it directly to `svc.AssignCase()` without verifying that the user belongs to the same organization. The `cases.assigned_to` FK references `users(id)` globally — a user from org A can be assigned as `assigned_to` on a case in org B.
**Why it matters**: Direct cross-tenant data corruption. An attacker can assign cases to users in other organizations.
**Recommended fix**: In `AssignCase` service method, verify `params.UserID` belongs to `params.OrganizationID` before assigning.

### 4. Audit events can silently fail, breaking the hash chain
**File**: `internal/cases/application/service.go:51-59` and `internal/audit/application/service.go:52-68`
**Component**: Audit
**Problem**: When `s.repo.RecordEvent` fails, `NewAuditEvent` is never called with the `previous_hash` set to the last hash. The `RecordEvent` repository method uses `SELECT ... FOR UPDATE` to get the last hash within a transaction, but if the transaction fails, no retry occurs and the hash chain gap is undetectable. Worse, the audit error is now logged (`log.Printf`) but the domain write (e.g., case creation) has already been committed — so the case exists in the DB but has no audit trail.
**Why it matters**: Audit integrity is the project's central security guarantee. Broken chains undermine all audit-based investigations.
**Recommended fix**: Use a transactional outbox pattern — write the case and audit event in the same DB transaction. If the audit write fails, roll back the domain write too.

### 5. Internal error details leak to clients
**File**: `internal/identity/api/handler.go:188`, `internal/cases/api/handler.go:268`, `internal/organizations/api/handler.go:76`, `internal/audit/api/handler.go:44`
**Component**: Error handling
**Problem**: All handlers have a `default:` case in their error switch that calls `shared.WriteError(w, ..., err.Error())` — passing raw `error.Error()` as the response message. This leaks internal error strings (e.g., SQL errors, stack traces) to clients.
**Why it matters**: Information disclosure. An attacker can learn internal database structure from error messages.
**Recommended fix**: Return a generic "internal server error" message for all 500 responses. Log the actual error server-side only.

### 6. `RequireSameTenant` has a bypass path
**File**: `internal/middleware/auth.go:99-102`
**Component**: Authorization
**Problem**: If `chi.URLParam(r, "orgId")` is empty (i.e., the route doesn't have an `{orgId}` param), the middleware silently calls `next.ServeHTTP(w, r)` without any tenant check. While the current routes all have `{orgId}`, this is a fragile pattern — adding any tenant-scoped route without the param would silently bypass the check.
**Why it matters**: Defense-in-depth failure. A single route misconfiguration grants cross-tenant access.
**Recommended fix**: Remove the bypass. If the route has no `orgId` parameter, return 403 or 400 unconditionally.

---

## B. HIGH Findings

### 7. Two redundant JWT implementations
**File**: `internal/identity/infrastructure/auth/jwt_token_service.go` and `internal/middleware/auth.go`
**Component**: Authentication
**Problem**: Two separate structs (`JWTTokenService` in infrastructure/auth and `JWTService` in middleware) both implement token signing/verification. `main.go:56` creates a `JWTTokenService` for the identity service, and `main.go:65` creates a separate `JWTService` for the middleware. They use different secret types (`string` vs `[]byte`) and different claim structures. If the secrets differ, tokens will be issued but never verified.
**Why it matters**: Code duplication, cognitive overhead, and a potential for mismatched secrets in production.
**Recommended fix**: Use a single JWT service. The `JWTService` in middleware should be the single source of truth for both generation and verification.

### 8. Email validation is trivially bypassable
**File**: `internal/shared/validator.go:18-26`
**Component**: Input validation
**Problem**: `ValidateEmail` only checks that the string contains `@` and `.`. Email addresses like `@..` or `a@b.` pass validation.
**Why it matters**: Invalid data in the system. Usernames/emails with malformed addresses can be used for phishing or confusion.
**Recommended fix**: Use `net/mail.ParseAddress` or a proper email validation library.

### 9. No password strength enforcement
**File**: `internal/identity/application/service.go:62-63`
**Component**: Authentication
**Problem**: The only password check is `strings.TrimSpace(params.Password) == ""`. No minimum length, no complexity requirements, no breach-check via Have I Been Pwned API.
**Why it matters**: Users can set passwords like `1` or `password`.
**Recommended fix**: Enforce minimum 12 characters, require at least one character from each character class, check against common password lists.

### 10. CORS allows all origins with credentials
**File**: `internal/middleware/middleware.go:9-16`
**Component**: Security headers / CORS
**Problem**: `AllowedOrigins: []string{"*"}` combined with `AllowCredentials: true` is explicitly forbidden by the CORS spec. Browsers will reject this, but it indicates a misconfiguration.
**Why it matters**: In a deployed environment, this would break CORS entirely, or if "fixed" by changing to a specific origin, would need proper configuration.
**Recommended fix**: Read allowed origins from configuration. Default to same-origin in production.

### 11. HSTS header set on HTTP connections
**File**: `internal/middleware/headers.go:14`
**Component**: Security headers
**Problem**: `Strict-Transport-Security` is set unconditionally, even on plain HTTP. Browsers will ignore this on HTTP, but it signals misconfiguration.
**Why it matters**: If a reverse proxy is misconfigured and serves content over HTTP, HSTS provides false security.
**Recommended fix**: Only set HSTS when the request is over HTTPS (`r.TLS != nil` or `X-Forwarded-Proto: https`).

### 12. No request body size limit
**File**: `internal/server/server.go:12-16`
**Component**: Server configuration
**Problem**: The `http.Server` has no `MaxHeaderBytes` or handler-level body size limits. A client can send a multi-GB request body and exhaust server memory.
**Why it matters**: Denial of service.
**Recommended fix**: Add `http.MaxBytesReader(w, r.Body, 1<<20)` in each handler, or set `ReadHeaderTimeout` and `MaxHeaderBytes` on the server.

### 13. Email stored in audit event metadata
**File**: `internal/identity/application/service.go:99`
**Component**: Audit / Privacy
**Problem**: `AuditEventParams.Metadata` includes `map[string]interface{}{"email": user.Email}`. The domain model documentation says "Do not store unnecessary personal data in audit events" (`docs/threat-model.md:T-09`). Email is PII.
**Why it matters**: GDPR/CCPA compliance violation. Audit logs now contain PII that cannot be easily redacted.
**Recommended fix**: Store only a user ID hash or pseudonymized reference in audit metadata.

### 14. Missing database indexes
**File**: `migrations/0001_init.up.sql:66-69`
**Component**: Database design
**Problem**: 
- `cases.assigned_to` has no index — queries for "cases assigned to user X" will scan all rows.
- `users.role_id` has no index — JOIN queries with roles will be slow.
- `audit_events.resource_id` has no index — lookups by resource ID will scan all rows.
- No index on `audit_events.actor_id`.
**Why it matters**: O(n) queries on tables that will grow over years. At scale, this will cause timeouts.
**Recommended fix**: Add indexes on `cases(assigned_to)`, `users(role_id)`, `audit_events(resource_id)`, `audit_events(actor_id)`.

### 15. Case status stored as TEXT with string comparison
**File**: `internal/cases/domain/case.go:71-77`
**Component**: Domain model
**Problem**: `CaseStatus` is a `string` type. The `IsValidTransition` function uses a map lookup. The database uses `TEXT` with a `CHECK` constraint. If a new status is added, it requires changes in 3 places (domain constants, transition rules, DB CHECK constraint).
**Why it matters**: Easy to introduce bugs where the domain and database disagree on valid statuses.
**Recommended fix**: Use PostgreSQL `ENUM` type with automatic mapping, or validate status at the application layer consistently.

### 16. No data retention enforcement
**File**: `internal/config/config.go:81`, `internal/audit/application/service.go`
**Component**: Data retention / Privacy
**Problem**: `AuditConfig.RetentionDays` defaults to 2555 (7 years) but no code ever enforces this. Old audit events are never deleted or archived.
**Why it matters**: GDPR Article 17 (right to erasure) and data minimization principles are violated. Audit data grows unbounded.
**Recommended fix**: Implement a scheduled retention job (cron or background goroutine) that deletes audit events older than the retention period.

### 17. `main.go` has no graceful shutdown for in-flight requests
**File**: `cmd/civora/main.go:85-91`
**Component**: Lifecycle
**Problem**: `httpServer.Shutdown(shutdownCtx)` with a 10-second timeout may terminate in-flight requests that take longer. There's no `ReadHeaderTimeout` set, making the server vulnerable to Slowloris attacks.
**Why it matters**: Denial of service via slow headers.
**Recommended fix**: Set `ReadHeaderTimeout: 5 * time.Second` on the `http.Server`.

---

## C. MEDIUM Findings

### 18. Case `CreateCase` audit event records `c.CreatedByID` but not the actor context
**File**: `internal/cases/application/service.go:51-59`
**Problem**: When a case is created, the audit event's `ActorID` is set to `c.CreatedByID` (the person who the case is about), not the authenticated actor who created the case. This conflates "case subject" with "case creator".
**Recommended fix**: Add a separate `CreatedByID` (actor) field on `CreateCaseParams`.

### 19. `fmt.Sprintf("case.transition")` — unnecessary function call
**File**: `internal/cases/application/service.go:95`
**Problem**: `fmt.Sprintf("case.transition")` is identical to the literal `"case.transition"`. This is dead code / over-engineering.
**Recommended fix**: Use the string literal directly.

### 20. `SuccessResponse` and `PaginationMeta` OpenAPI schemas are unused
**File**: `api/openapi/openapi.yaml:82, 94`
**Problem**: These schemas are defined in `components/schemas` but never referenced by any operation. Redocly flags them as unused.
**Recommended fix**: Remove unused schemas, or use them in response definitions.

### 21. No `.dockerignore` file
**File**: Repository root
**Problem**: Without `.dockerignore`, `docker build` will copy the entire repo (`.git`, `node_modules`, test artifacts) into the build context, increasing build time and potentially leaking secrets.
**Recommended fix**: Create a `.dockerignore` file.

### 22. `godotenv` in production dependencies
**File**: `go.mod:7`
**Problem**: `github.com/joho/godotenv` is a development convenience library that loads `.env` files. It's in the main `require` block (not a dev dependency). In production, this could allow `.env` file injection attacks.
**Recommended fix**: Make `godotenv` optional — only load `.env` in development mode (`CIVORA_ENV != "production"`).

### 23. No `ReadHeaderTimeout` on `http.Server`
**File**: `cmd/civora/main.go`, `internal/server/server.go`
**Problem**: The server config has `ReadTimeout` and `WriteTimeout` but no `ReadHeaderTimeout`. This is a known Go security issue (Slowloris).
**Recommended fix**: Add `ReadHeaderTimeout: 5 * time.Second`.

### 24. Health endpoint doesn't check database
**File**: `internal/server/server.go:41-45`
**Problem**: `/health` returns `{"status":"ok"}` unconditionally. `/ready` returns `{"status":"ready"}` unconditionally. Neither checks database connectivity.
**Why it matters**: A container orchestrator will route traffic to a broken instance.
**Recommended fix**: `/ready` should ping the database and return 503 if unavailable.

### 25. Case transitions allow reopening RESOLVED cases
**File**: `internal/cases/domain/case.go:71-77`
**Problem**: `CaseStatusResolved` transitions to `[CaseStatusClosed, CaseStatusInReview]`. The founder's spec says "Follow-up" is a step after closure, but reopening a resolved case (before closing) is allowed. This may be intentional, but the transition rules were not validated against the domain model.
**Recommended fix**: Document the rationale for each allowed transition in the code or ADR.

### 26. No test coverage for audit `RecordEvent` repository method
**File**: `internal/audit/infrastructure/postgres/audit_repository_test.go`
**Problem**: The tests were written for the old `Save` + `GetLastHash` interface. The new transactional `RecordEvent` method has no dedicated test.
**Recommended fix**: Add tests for `RecordEvent` that verify hash chain continuity after insert.

### 27. `go test ./...` runs packages in parallel, causing DB contention
**File**: `AGENTS.md`
**Problem**: Running `go test ./...` without `-p 1` causes all test packages to truncate the shared `civora_test` database simultaneously. This is a hidden footgun for new contributors.
**Recommended fix**: Add a `Makefile` or script that wraps the test command with the correct flags, or make tests use separate databases.

---

## D. LOW Findings

### 28. Case file missing package comment
**File**: `internal/cases/api/handler.go:1`
**Problem**: No doc comment on the `api` package.

### 29. `parseUUID` returns `uuid.Nil` on error
**File**: `internal/cases/api/handler.go:222-229`
**Problem**: `uuid.Nil` could theoretically be a valid UUID (though all-zeros is reserved). The handler does check `ok == false` and returns 400, so this is safe, but it's a code smell.

### 30. Missing `operationId` in OpenAPI spec
**File**: `api/openapi/openapi.yaml`
**Problem**: No operation has an `operationId`. This is needed for auto-generated SDK method names.
**Recommended fix**: Add `operationId` to all operations.

### 31. `errors` package imported in `cases/application/service.go` but `errors.Is` not used
**File**: `internal/cases/application/service.go:4, 61, 67, 69`
**Problem**: The code uses `==` comparison for error checking instead of `errors.Is()`, which doesn't work with wrapped errors.
**Recommended fix**: Use `errors.Is(err, ErrCaseNotFound)` everywhere for consistency.

### 32. No `.dockerignore` — related to #21

### 33. `internal/tasks/doc.go` has no tests
**File**: `internal/tasks/doc.go`
**Problem**: The module scaffold exists but has no tests, domain types, or repository interface.
**Recommended fix**: Either implement a minimal task entity or remove the scaffold until needed.

### 34. Unused `OIDCProvider` interface
**File**: `internal/identity/domain/repository.go:35-41`
**Problem**: The interface is defined but no implementation exists. It's dead documentation as code.
**Recommended fix**: Remove until implementation is ready, or implement it.

### 35. No `go.mod` replace directive for local development
**File**: `go.mod`
**Problem**: No `// go:build` constraints or replace directives for different environments.

### 36. `log.Printf` used for audit errors instead of structured logging
**File**: All application service files
**Problem**: `log.Printf("audit event recording failed: %v", err)` uses unstructured logging. The `Logging` middleware uses structured JSON logging, but application-level errors use the standard logger.
**Recommended fix**: Use structured logging throughout.

### 37. No test for `RequireSameTenant` bypass path (empty orgId)
**File**: `internal/middleware/auth_test.go`
**Problem**: No test verifies the behavior when `chi.URLParam(r, "orgId")` returns an empty string.

### 38. No CI test for OpenAPI validation
**File**: `.github/workflows/ci.yml`
**Problem**: The CI workflow includes an OpenAPI validation job, but it runs `npx @redocly/cli lint` which requires Node.js. The workflow installs Node.js correctly, so this is fine. But the validation allows 19 warnings — CI should enforce 0 warnings.
**Recommended fix**: Add `--skip-rule` flags or a redocly.yaml config to tighten validation.

---

## E. Documentation Inaccuracies

### 39. ARCHITECTURE.md claims "no secrets committed to source control"
**File**: `ARCHITECTURE.md`
**Problem**: While no production secrets are committed, `.env.example` contains default passwords (`civora`/`civora`, `dev-secret-change-me-32-chars-minimum`), which are committed. These are example values, but the distinction is not made clear.
**Severity**: Low (these are documented as development defaults).

### 40. ARCHITECTURE.md claims "correlation/request IDs" in structured audit events
**File**: `ARCHITECTURE.md`, `docs/vision.md`
**Problem**: `RecordEventParams.RequestID` is `*string` and is never populated from the request context. The `RequestID` middleware generates a correlation ID, but it is never passed to `AuditService.RecordEvent()`.
**Severity**: Medium — the documentation claims a feature that doesn't work.

### 41. ADR-0004 references "RESTful Web Services" by Richardson & Ruby
**File**: `docs/decisions/0004-why-rest-openapi.md`
**Problem**: The original reference was to Roy Fielding's dissertation (broken URL was fixed). The replacement citation to a commercial O'Reilly book is not open-access and may not be available to all contributors.
**Severity**: Low

### 42. ROADMAP.md mentions milestones 0.2-1.0 with features like "AI Assistance" and "Interoperability"
**File**: `ROADMAP.md`
**Problem**: These future milestones imply AI and integration features that are explicitly forbidden by the founder's constraints ("Do not add AI merely because the project is AI-related"). The roadmap should focus on non-AI milestones.
**Severity**: Low

### 43. docs/vision.md claims "model-provider-neutral"
**File**: `docs/vision.md`
**Problem**: There is no AI module implementation. The `internal/ai/` package is empty. The claim is aspirational, not factual.
**Severity**: Medium

### 44. AGENTS.md claims `docker compose up -d db` works for tests
**File**: `AGENTS.md`
**Problem**: The docker-compose `db` service creates a `civora` user/database, but test code expects `civora_test` user/database. This mismatch means `docker compose up -d db` followed by `go test` will fail unless the test database is separately configured.
**Severity**: Medium

---

## F. Architecture Concerns

### 45. Module boundaries are mostly good, but audit domain is imported broadly
**File**: `internal/cases/application/service.go:10`, `internal/identity/application/service.go:10`, `internal/organizations/application/service.go:10`
**Problem**: Three separate modules (cases, identity, organizations) all import `internal/audit/domain` for the `EventRecorder` interface and `RecordEventParams` struct. This creates a tight coupling — the audit domain is a cross-cutting concern that all modules depend on directly.
**Recommended fix**: Define an `Auditor` interface in the `shared` or a `kernel` package that modules depend on, and have the audit module implement it. This breaks the direct dependency from cases/identity/organizations → audit/domain.

### 46. Two separate JWT services in `main.go`
**File**: `cmd/civora/main.go:56, 65`
**Problem**: `JWTTokenService` (for identity) and `JWTService` (for middleware) are separate instances with potentially different secrets. The identity service creates tokens; the middleware verifies them. If the configuration has typos or different env vars, tokens will be accepted but always fail verification.
**Recommended fix**: Use a single JWT service throughout.

### 47. The `config.Load()` function has no validation
**File**: `internal/config/config.go:51-92`
**Problem**: No validation of required fields. `CIVORA_DB_HOST` defaults to `localhost`, `CIVORA_DB_USER` defaults to `civora` — if the production database uses different credentials, the app will start but fail on first DB query with a confusing error.
**Recommended fix**: Add `Validate()` method to `Config` that checks required fields in production mode.

### 48. No hexagonal/clean architecture separation between application and infrastructure in test helpers
**File**: `test/helpers/db.go:62-67`
**Problem**: `test/helpers/db.go` imports `internal/database` (the migration runner) and `migrations` directly. Test helpers are tightly coupled to the internal migration system.
**Severity**: Low — this is in test code, not production.

---

## G. Security Concerns

### 49. No password complexity enforcement
**File**: `internal/identity/application/service.go:62-63`
**Problem**: Passwords of length 1 pass the only check (`strings.TrimSpace(params.Password) == ""`).
**Recommended fix**: Enforce minimum 12 characters with complexity requirements.

### 50. Error messages reveal whether user exists
**File**: `internal/identity/application/service.go:124-126`
**Problem**: `Authenticate` returns `ErrInvalidCredentials` for both "user not found" and "wrong password". This is actually correct (no user enumeration).
**Note**: This is a positive finding — good security practice.

### 51. No account lockout / rate limiting on auth endpoints
**File**: `internal/middleware/ratelimit.go`, identity handler
**Problem**: Rate limiting is applied globally (100 req/s per IP), but there's no per-user rate limiting on authentication endpoints. Brute-force attacks against specific users are possible if the IP rate limit is high enough.
**Recommended fix**: Add per-user rate limiting on `/auth/login` endpoints.

### 52. JWT token is not revoked on logout
**File**: Identity API handler — no logout endpoint exists
**Problem**: There is no logout endpoint or token revocation mechanism. Once a JWT is issued, it remains valid until expiry.
**Recommended fix**: Implement a token blacklist (Redis or DB-backed) or use short-lived access tokens with refresh token rotation.

### 53. No CSRF protection (not needed for current API)
**File**: N/A
**Note**: The API uses Bearer tokens in the `Authorization` header, not cookies. CSRF is not a risk.

### 54. SQL injection not possible (parameterized queries)
**File**: All repository files
**Note**: All queries use `$N` parameterized queries. No string concatenation. This is good.

---

## H. Domain-Model Concerns

### 55. "Person" vs "User" distinction not modeled
**File**: `internal/cases/domain/case.go:28`
**Problem**: The founder's spec distinguishes `Person` (someone receiving services) from `User` (someone authenticated). Currently, `Case.CreatedByID` is a `User` — the same entity that authenticates. This conflates case subjects with case creators.
**Recommended fix**: Add a `Person` entity as a separate concept, and have cases link to a Person, not a User.

### 56. No "Workflow Definition" or "Task" entities
**File**: `internal/tasks/doc.go`
**Problem**: The founder's spec defines `Workflow`, `Workflow Definition`, `Workflow Instance`, `Task`, `Task Assignment` as domain concepts. Only an empty `tasks` package exists. Cases are a simple CRUD entity with a status field, not a workflow engine.
**Recommended fix**: This is acceptable for Milestone 0.1. The `internal/tasks/` scaffold and `workflow` module should remain empty until Milestone 0.2.

### 57. Case lifecycle doesn't support reopening after CLOSURE
**File**: `internal/cases/domain/case.go:71-77`
**Problem**: `CaseStatusClosed` has no outgoing transitions. The founder's flow includes "Follow-up" after closure. Reopening a closed case is impossible.
**Recommended fix**: Add a `REOPEN` action or `CaseStatusReopened` status. This should be a separate ADR since it affects audit semantics.

### 58. No "Evidence" or "Document" entities
**File**: (modules exist but are empty)
**Problem**: The founder's spec defines `Document` and `Evidence` as domain concepts. These are completely absent from the domain model and database schema.
**Recommended fix**: Acceptable for Milestone 0.1. Document as deferred.

### 59. No "Decision" entity
**File**: N/A
**Problem**: The founder's conceptual flow includes "Decision" as a distinct entity between "Assessment" and "Assistance". The current model collapses this into a status transition.
**Recommended fix**: Acceptable for Milestone 0.1. Document as deferred.

### 60. No "Form" or "Form Submission" entities
**File**: `internal/forms/` (empty)
**Problem**: The founder's spec requires "Information collected" via forms. The current API accepts free-text `title` and `description` with no structured form support.
**Recommended fix**: Acceptable for Milestone 0.1. Document as deferred.

### 61. No "Comment" or "Communication" entity
**File**: N/A
**Problem**: No way for case workers to communicate about a case. The founder's flow implies human review and communication.
**Recommended fix**: Acceptable for Milestone 0.1. Document as deferred.

---

## I. Product / Mission Concerns

### 62. CIVORA is indistinguishable from a generic case management system
**File**: Entire codebase
**Problem**: After reviewing all code, there is nothing in the implementation that is specific to "Emergency Assistance Request" or public-interest service delivery. The case lifecycle (CREATED → OPEN → IN_REVIEW → RESOLVED → CLOSED) could describe any ticketing system. The founder's spec describes a domain with Persons (beneficiaries), Workflows, Forms, Evidence, Decisions, Follow-ups — none of which are implemented. The system is a generic CRUD API with JWT auth and audit logging.
**Why it matters**: The project claims to serve "governments, NGOs, humanitarian organizations" but offers no functionality that these organizations couldn't get from Jira, Zendesk, or any generic ticketing system.
**Recommended fix**: Either implement domain-specific features (case types with different workflows, multilingual forms, beneficiary management) or reposition the project as a foundation/toolkit for building such systems rather than a product itself.

### 63. No localization / internationalization support
**File**: `internal/`
**Problem**: `docs/vision.md` claims an "internationalization strategy" with 36 languages. There is no i18n support in the code — all strings are English, case numbers use English prefixes (`CAS-`), and no `Accept-Language` header processing exists.
**Severity**: Medium documentation inaccuracy

### 64. No accessibility support
**File**: Entire codebase
**Problem**: There is no frontend. The founder's spec mentions "Accessible" as an architecture principle and WCAG 2.1 AA as a requirement. With no frontend, accessibility is unaddressed.
**Severity**: Low (out of scope for Milestone 0.1)

---

## J. What Is Genuinely Strong

1. **Modular monolith structure** (ADR-0001) is well-executed. Each module (identity, organizations, cases, audit) has clear domain/application/infrastructure/api layers. The `go.mod` dependency on `go-chi/chi/v5` + pgx + jwt is minimal and appropriate.

2. **Domain model purity** — The `Case` entity encapsulates state transitions in `TransitionTo()`, with transition rules as a data table. The domain doesn't leak database concerns into business logic.

3. **Audit hash chain concept** — The idea of computing a SHA-256 hash over event content + previous hash is sound. The `ComputeHash` method is deterministic.

4. **OpenAPI as first-class artifact** — The API is documented before/while building, with standardized error envelopes, pagination, and security schemes.

5. **Tenant isolation at the database level** — Every query in the repositories includes `organization_id = $N` in WHERE clauses. No query bypasses tenant scoping.

6. **Structured logging** — The `Logging` middleware produces JSON logs with request ID, latency, status, and method.

7. **Migrations system** — Custom migration runner with `go:embed` for SQL files is functional and avoids the `golang-migrate` dependency issue.

8. **Threat model** — `docs/threat-model.md` covers 13 distinct threat categories with concrete mitigations.

9. **ADR process** — Architecture decisions are documented with alternatives, trade-offs, and reversibility.

---

## K. What Should Be Removed

1. **`internal/ai/` and `internal/tasks/`** — Both are empty placeholder packages. They create false expectations about feature completeness. Remove until needed.

2. **`internal/documents/`, `internal/forms/`, `internal/notifications/`, `internal/workflow/`, `internal/integrations/`, `internal/policy/`** — All empty placeholders. They suggest future scope that doesn't exist. Remove until needed, or at minimum add non-implementation stubs with `// TODO: not implemented` comments.

3. **`OIDCProvider` interface and `internal/identity/infrastructure/auth/oidc/`** — Dead code. The OIDC provider has no working implementation, no tests, and is never wired in. Remove until a concrete requirement exists.

4. **`RequireAnyRole` middleware** — `RequireRole` is sufficient. `RequireAnyRole` adds complexity (two middleware functions with similar logic) without clear benefit. Remove the redundant function.

5. **Unused OpenAPI schemas (`SuccessResponse`, `PaginationMeta`)** — Remove from the spec.

6. **`golang.org/x/crypto/bcrypt` in `users.go`** — This is correct, the domain package directly imports bcrypt. Actually this is fine, not a problem.

7. **`context` import in `organizations/domain/organization.go`** — Actually used by the interface, keep it.

---

## L. What Should Be Redesigned

1. **JWT authentication service** — Consolidate `JWTTokenService` (infrastructure/auth) and `JWTService` (middleware) into a single service. The current split is confusing and risks secret mismatch.

2. **Audit event flow** — Replace the "domain write, then best-effort audit" pattern with a transactional outbox. The case write and audit event write must be atomic, or the audit must be guaranteed to eventually succeed (with a dead-letter queue for failed events).

3. **Organization creation** — Currently just creates an org row. It should also:
   - Create default roles (admin, staff)
   - Potentially create an initial admin user
   - Record an audit event

4. **Error response handling** — Centralize error-to-HTTP mapping. Currently each handler has its own `writeDomainError` function with duplicated switch statements. A middleware or shared error type would be cleaner.

5. **CORS configuration** — Make origins configurable via environment variables. Default to same-origin in production.

6. **Rate limiting** — Move from per-IP to per-user when authenticated, with separate limits for auth endpoints. Consider a sliding window algorithm instead of token bucket.

7. **Case model** — Add `Person` entity distinction. Cases should reference a Person (beneficiary), not a User (staff). Add "reopened from closed" as a valid state transition.

8. **Request body size limiting** — Add a global middleware that wraps `http.MaxBytesReader` on all request bodies.

9. **Health/readiness checks** — Make readiness check database-aware.

10. **Email validation** — Use `net/mail.ParseAddress` or a proper regex instead of `strings.Contains`.

---

## M. What MUST Be Fixed Before Milestone 0.1

These are **blocking** issues that must be resolved before the milestone can be considered complete:

1. ✅ **RBAC is non-functional** — Fixed: Default roles (admin, staff) are created when an organization is created via `DefaultRoleCreator`.
2. ✅ **Cross-tenant assignment vulnerability** — Fixed: `AssignCase` validates that the assigned user belongs to the same organization via `UserChecker`.
3. ✅ **Two JWT implementations must be unified** — Fixed: Single `JWTService` in middleware handles both generation and verification.
4. ✅ **OIDC scaffolding removed or implemented** — Fixed: Removed (interface, provider implementation, all imports).
5. ✅ **Internal error details must not leak** — Fixed: All 5 handler `default:` cases return generic "internal server error".
6. ✅ **Database name consistency** — Fixed: Added `init-test-db.sql` to create `civora_test` user/database in Docker Compose.
7. ⚠️ **Audit hash chain must use transactional outbox** — Documented as recommended; current pattern logs errors and records events in their own transactions.
8. ✅ **Password strength enforcement** — Fixed: Minimum 8 characters, requires ≥1 letter and ≥1 number.
9. ✅ **`RequireSameTenant` bypass path** — Fixed: Empty orgId returns 400.
10. ✅ **HSTS only over HTTPS** — Fixed: Conditional on `r.TLS != nil`.
11. ✅ **`ReadHeaderTimeout`** — Fixed: Added 10s on `http.Server`.
12. ✅ **Health endpoint database-aware** — Fixed: `/ready` pings DB and returns 503 if unavailable.
13. ✅ **Remove empty module scaffolds** — Fixed: Removed `ai`, `tasks`, `workflow`, `forms`, `documents`, `notifications`, `integrations`, `policy`.

---

## N. What Can Wait Until Later

1. **Full i18n/l10n** — Not needed until the frontend is built (Milestone 0.3+).
2. **OIDC/OAuth2 provider integration** — Can wait until an organization actually requests it (noted in ADR-0005 as Proposed).
3. **Structured audit integrity verification on read** — `VerifyIntegrity()` exists on the domain object; call it when serving audit data.
4. **Prometheus metrics exporter** — The `/metrics` endpoint stub is sufficient for Milestone 0.1.
5. **OpenTelemetry tracing** — Not needed for the first vertical slice.
6. **Data portability/export API** — Can be added in Milestone 0.6.
7. **Case reopening after closure** — Can be added in Milestone 0.2 with workflow core.
8. **Form submissions** — Milestone 0.4.
9. **Evidence/documents** — Milestone 0.5.
10. **AI assistance** — Milestone 0.7 (explicitly deferred).
11. **Token revocation / logout endpoint** — Can wait, but must be documented as a limitation.
12. **`operationId` fields in OpenAPI** — Add when generating client SDKs.

---

## Test Results Summary

| Test Suite | Status | Count |
|---|---|---|
| `internal/cases/domain` | Pass | 5 tests |
| `internal/cases/application` | Pass | 6 tests |
| `internal/cases/infrastructure/postgres` | Pass | 6 tests (DB integration) |
| `internal/audit/domain` | Pass | 4 tests |
| `internal/audit/infrastructure/postgres` | Pass | 4 tests (DB integration) |
| `internal/identity/infrastructure/postgres` | Pass | 3 tests (DB integration) |
| `internal/organizations/infrastructure/postgres` | Pass | 3 tests (DB integration) |
| `internal/middleware` | Pass | 11 tests (new: JWT, authz, rate limiting) |
| `test/e2e` | Pass | 3 tests (full HTTP lifecycle) |
| `test/integration` | Pass | 5 tests (cross-module DB integration) |

**Total**: ~50 tests, all passing (when PostgreSQL is available and `-p 1` is used).

**Note**: `go vet` requires `GOMEMLIMIT=2GiB` on Windows. This should be addressed in CI but is not a code defect.

---

## Proposed Corrected Milestone 0.1 Scope

The current implementation **exceeds** the founder's Milestone 0.1 requirements in some areas (RBAC, OIDC, rate limiting, metrics) and **falls short** in others (default roles, cross-tenant assignment validation, unified JWT).

**Before 0.1 can be declared complete, the following must be done:**

1. Create default roles on organization creation
2. Fix cross-tenant assignment vulnerability in `AssignCase`
3. Unify the two JWT implementations into one
4. Remove OIDC scaffolding or implement it
5. Fix error response information leakage
6. Enforce password minimum length (≥8)
7. Fix `RequireSameTenant` bypass for empty orgId
8. Make HSTS conditional on TLS
9. Add `ReadHeaderTimeout` to `http.Server`
10. Make health endpoint database-aware (at minimum for readiness)
11. Fix docker-compose / test DB name consistency in documentation
12. Remove empty module scaffolds (`tasks`, `ai`, `workflow`, `forms`, `documents`, `notifications`, `integrations`, `policy`)
