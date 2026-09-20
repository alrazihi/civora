# CIVORA 1.0.0 — Verification Record

**Release:** CIVORA 1.0.0  
**Tag:** v1.0.0  
**Release SHA:** `5021316f752e5716159585af2dfd6cee7cb9a412`  
**Post-release documentation commit:** `4a61285`  
**Date:** 2026-09-20  
**Status:** Frozen — no further changes expected before 1.0.x patch or 1.1.0

This document preserves the engineering evidence behind the 1.0.0 release.
It is an internal record, not marketing material.

---

## 1. Release Identity

| Field | Value |
|-------|-------|
| Version | 1.0.0 |
| Tag | v1.0.0 |
| Release commit SHA | `5021316f752e5716159585af2dfd6cee7cb9a412` |
| Post-release docs commit | `4a61285` |
| OpenAPI spec version | 1.0.0 |
| Status | Production-ready baseline |

---

## 2. Verification Gates

### 2.1 Repository Integrity

| Check | Result |
|-------|--------|
| Working tree clean | PASS |
| All tags present | PASS |
| Release SHA verified | PASS |

### 2.2 Build

| Command | Result |
|---------|--------|
| `go build ./...` | PASS |

### 2.3 Formatting

| Command | Result |
|---------|--------|
| `gofmt -l .` | CLEAN (workspace files only; `.kilo/worktrees` artifacts excluded) |

### 2.4 Lint / Vet

| Command | Result |
|---------|--------|
| `go vet ./...` | PASS on Linux CI; Windows OOM documented limitation (see §6) |

### 2.5 Unit Tests

| Command | Packages | Result |
|---------|----------|--------|
| `go test -short ./...` | 60 | ALL PASS |

### 2.6 Integration Tests

| Command | Packages | Result |
|---------|----------|--------|
| `go test -p 1 -count=1 ./test/integration/...` | 66 | ALL PASS |

Integration tests require a PostgreSQL instance with a `civora_test` user and
database. They are run serially (`-p 1`) to avoid database conflicts.

### 2.7 Security Tests

| Test | Result |
|------|--------|
| `TestLogin_ReturnsAccessAndRefreshTokens` | PASS |
| `TestRefreshToken_RotatesTokens` | PASS |
| `TestRefreshToken_ReuseDetected` | PASS |
| `TestStolenRefreshToken_DetectedAndRevoked` | PASS |
| `TestConcurrentRefresh` | PASS (exactly 1 success, 1 failure) |
| `TestRevokedSession_AccessDenied` | PASS |
| `TestWrongTenantSession_Denied` | PASS |
| `TestRoleChange_TakesEffectOnNextLogin` | PASS |
| `TestSigningKeyRotation_ValidatesTokens` | PASS |
| `TestTenantIsolationAtAPI` | PASS |
| `TestUnauthorizedDecisionOnAnotherOrg` | PASS |
| `TestEvidenceSecurity_CrossTenantRead` | PASS |
| `TestEvidenceSecurity_CrossCaseRead` | PASS |
| `TestEvidenceSecurity_StorageReferenceNotLeaked` | PASS |
| `TestAuditTrailForServiceRequest` | PASS |
| `TestConcurrentCaseTransitions` | PASS (exactly 1 success) |
| `TestConcurrentWorkflowInstanceTransitions` | PASS (exactly 1 success) |
| `TestConcurrentDuplicateAssistance` | PASS (all succeed, idempotent) |
| `TestConcurrentDuplicateDecision` | PASS (exactly 1 success) |
| `TestConcurrentFormArchiving` | PASS (≥1 success) |
| `TestConcurrentVersionPublishing` | PASS (≥1 success) |
| `TestConcurrentDuplicateFieldKeys` | PASS (≤1 success) |

### 2.8 Playwright E2E

| Command | Specs | Result |
|---------|-------|--------|
| `cd web/e2e && npm test` | 120 | 120 PASS |

The Playwright suite serves `web/` as a static site and mocks the API via
request interception, running fully offline.

### 2.9 Go E2E

| Command | Result |
|---------|--------|
| `go test ./test/e2e/...` | Compiled and runnable; requires PostgreSQL (skipped with `-short`) |

### 2.10 OpenAPI Validation

| Command | Result |
|---------|--------|
| `npx @redocly/cli lint api/openapi/openapi.yaml` | VALID |

### 2.11 Migrations

| Check | Result |
|-------|--------|
| Count | 44 files (0001–0044) |
| Order | Correct |
| Reversibility | All migrations have `down` components |
| Data-loss migrations | None detected |
| Migration tests | PASS (`internal/database/...`) |

### 2.12 Documentation

| Check | Result |
|-------|--------|
| README.md | Updated for 1.0.0 |
| ARCHITECTURE.md | Updated to describe 1.0.0 architecture |
| SECURITY.md | Updated to full security reference |
| CHANGELOG.md | 1.0.0 entry verified |
| This verification record | Created |

---

## 3. Security Verification

### 3.1 Authentication

| Scenario | Evidence |
|----------|----------|
| Invalid JWT rejected | Integration test + middleware unit test |
| Expired JWT rejected | Integration test |
| Missing token rejected | Middleware unit test |
| Revoked session rejected | `TestRevokedSession_AccessDenied` |
| Refresh token rotation | `TestRefreshToken_RotatesTokens` |
| Refresh token reuse detection | `TestRefreshToken_ReuseDetected` |
| Stolen token revocation | `TestStolenRefreshToken_DetectedAndRevoked` |
| Concurrent refresh | `TestConcurrentRefresh` (exactly 1 success) |
| Password-change invalidation | `ChangePassword` calls `RevokeAllUserSessions` |
| Role-change invalidation | `TestRoleChange_TakesEffectOnNextLogin` |
| Signing-key rotation | `TestSigningKeyRotation_ValidatesTokens` |

### 3.2 Authorization

| Scenario | Evidence |
|----------|----------|
| Staff accessing admin-only routes | DENIED (403) |
| Unauthenticated access | DENIED (401) |
| Cross-tenant resource access | DENIED (403/404) |
| Role injection via registration | BLOCKED — registration ignores `role_name` |
| Stale role after promotion | DENIED until re-login |

### 3.3 Multi-Tenancy (Cross-Tenant Attacks)

| Attack Vector | Result |
|---------------|--------|
| Tenant A → Tenant B case | DENIED (404/403) |
| Tenant A → Tenant B evidence | DENIED (403) |
| Tenant A → Tenant B session | DENIED (404) |
| Tenant A → Tenant B audit data | DENIED (404) |
| Tenant A → Tenant B workflow | DENIED (404) |
| Tenant A → Tenant B rules | DENIED (404) |

### 3.4 Session Security

| Property | Status |
|----------|--------|
| JTI/session binding | ENFORCED |
| Session ownership | ENFORCED |
| Organization ownership | ENFORCED (org-aware DB queries) |
| Revoked-session rejection | ENFORCED |
| Expired-session rejection | ENFORCED |
| Refresh-token family behavior | ENFORCED |
| Password-change invalidation | ENFORCED |
| Role-change invalidation | ENFORCED |

### 3.5 Concurrency

| Property | Status |
|----------|--------|
| Workflow transitions | DETERMINISTIC (exactly 1 success per race) |
| Concurrent assistance creation | IDEMPOTENT (all succeed) |
| Concurrent decision creation | EXACTLY 1 success |
| Concurrent form field creation | PREVENTED (≤1 success) |
| Evidence verification | SAFE (row-level locking) |

### 3.6 Audit Integrity

| Property | Status |
|----------|--------|
| Hash chain computation | VERIFIED |
| Chain verification on startup | IMPLEMENTED |
| Background maintenance | IMPLEMENTED |
| Transactional recording | ENFORCED (`RecordEventInTx`) |
| Schema isolation | ENFORCED (`audit` schema separate from `public`) |

---

## 4. Architecture Frozen at 1.0

The following architectural invariants must be preserved in all future 1.0.x
patches and must not be broken in 1.1.0 without a recorded ADR:

1. **Backend authority:** The backend is the sole authority for workflow state,
   transition validation, authorization, and audit. The frontend renders but
   does not decide.
2. **Multi-tenancy:** All data is scoped by `organization_id`. Cross-tenant
   access is rejected at the API, service, and repository layers. This must
   not be weakened.
3. **Session two-level API:** `FindByRefreshTokenHash` does not require
   `organization_id`. All authenticated session operations use org-aware
   methods. This design must not be simplified.
4. **Audit hash chain:** Events are recorded in the `audit` schema with
   SHA-256 chaining. Audit writes are transactional with business operations.
   This must not be bypassed or made optional.
5. **Workflow determinism:** Transitions go through `ExecuteTransition` only.
   No module may bypass the engine. Terminal states cannot be overridden.
6. **Human authority:** Rules produce advisory outcomes only. Final decisions
   require an authorized human actor. No code path may allow AI or automated
   systems to make consequential decisions.
7. **Evidence integrity:** Storage references are excluded from API responses.
   Verification uses row-level locking. Published forms and rule sets are
   immutable.
8. **OpenAPI as source of truth:** API behavior must match the OpenAPI
   specification. Changes to API behavior require corresponding OpenAPI updates.

---

## 5. Known Limitations

### P2 (Desirable for 1.0.x, Not Blocking Release)

| # | Limitation | Mitigation |
|---|-----------|------------|
| 1 | Windows `go vet` fails due to platform OOM limits | CI on Ubuntu passes; documented in AGENTS.md |
| 2 | Audit chain is tamper-evident, not tamper-proof against privileged insider/DBA | Documented in SECURITY.md; external append-only anchor is post-1.0 |
| 3 | Frontend custom-workflow transition mapping is heuristic | Backend still enforces authorization server-side |

### P3 (Explicitly Deferred to Post-1.0)

| # | Limitation |
|---|-----------|
| 1 | Encryption at rest by default |
| 2 | Built-in backup/DR |
| 3 | GDPR data export/deletion APIs |
| 4 | SOC 2 / HIPAA technical safeguards |
| 5 | Prometheus `/metrics` endpoint |
| 6 | Real-time updates / WebSocket |
| 7 | Offline support / service worker |
| 8 | Bulk operations |
| 9 | Frontend unit test framework (Jest/Vitest) |

---

## 6. Platform-Specific Notes

### Windows

The Go race detector (`-race`) may fail on Windows due to memory limits. If
you encounter `fatal error: runtime: out of memory` or `The paging file is too
small`, either increase the Windows paging file size or omit `-race` on that
platform. CI runs on Ubuntu and includes race testing where applicable.

### Linux

All verification gates pass on Linux CI, including `go vet`, `go test -race`
(where memory permits), and all integration tests.

---

## 7. Post-Release Documentation

Post-release documentation commits:

| Commit | Description |
|--------|-------------|
| `4a61285` | Finalize CIVORA 1.0.0 release documentation |

---

*This document was created as part of the CIVORA 1.0.0 release process.
It records the engineering evidence behind the release and is not a
guarantee of future behavior.*
