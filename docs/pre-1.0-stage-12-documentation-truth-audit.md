# CIVORA Pre-1.0 Stage 12 — Documentation Truth Audit

**Audit type:** Documentation vs implementation truth verification  
**Audit date:** 2026-09-19  
**Auditor:** CIVORA Documentation Truth Auditor  
**Branch inspected:** `main`  
**Commit inspected:** `2f517e0` (HEAD)

---

## 1. Methodology

Every significant claim in the following documents was verified against the actual implementation:

- `README.md`
- `ARCHITECTURE.md`
- `docs/threat-model.md`
- `docs/pre-1.0-stage-01-master-architecture-audit.md`
- `docs/pre-1.0-stage-05-human-accountability.md`
- `docs/pre-1.0-stage-07-security-master-audit.md`
- `docs/0.9-final-release-gate.md`
- `docs/0.9-stage-09-platform-proof.md`
- `ROADMAP.md`
- `SECURITY.md`
- ADRs in `docs/decisions/`
- Inline code comments

Claims are classified as:

| Classification | Meaning |
|----------------|---------|
| **TRUE** | Claim is fully supported by implementation |
| **PARTIALLY TRUE** | Core claim is correct but has gaps or caveats |
| **FALSE** | Claim is contradicted by implementation |
| **STALE** | Claim was true but is now outdated |
| **UNVERIFIED** | No evidence found to confirm or deny |

---

## 2. Executive Summary

The documentation is **mostly accurate** but contains several material misrepresentations:

1. **Audit immutability claim is FALSE** — the implementation includes a `PurgeOld` method that deletes audit events and rebuilds the hash chain.
2. **Encryption claims are PARTIALLY TRUE** — passwords are bcrypt-hashed, but TLS and encryption-at-rest are not implemented.
3. **Retention claim is PARTIALLY TRUE** — retention logic exists but deletes data rather than archiving, and is not organization-configurable.
4. **Several STALE documentation issues** remain from rapid 0.8/0.9 development.

The platform's core capabilities (workflow engine, forms, rules, tenant isolation, AI safety boundaries) are accurately represented.

---

## 3. Claim-by-Claim Audit

### 3.1 Production Readiness

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "CIVORA is in early development (Milestone 0.x). The project is not yet production-ready." | README.md:23-26 | **TRUE** | Consistent across docs; ROADMAP.md shows 1.0 not yet reached |
| "Single static Go binary" | README.md:108, ARCHITECTURE.md:461 | **TRUE** | `CGO_ENABLED=0` in Dockerfile; `go build` produces static binary |
| "Docker container image; Docker Compose for local development" | README.md:433 | **TRUE** | `Dockerfile` and `docker-compose.yml` exist |
| "Kubernetes manifests provided (future)" | ARCHITECTURE.md:429 | **TRUE** | No K8s manifests found; claim says "future" |
| "Non-root user in container" | CLAIMED in docs | **TRUE** | Dockerfile line 18: `adduser -S -G civora civora`, line 20: `USER civora` |
| "Health check endpoint" | ARCHITECTURE.md:443 | **TRUE** | `internal/server/server.go:45` — `r.Get("/health", healthHandler)` |
| "Graceful shutdown" | IMPLIED | **TRUE** | `internal/server/server.go:88-92` — `httpServer.Shutdown` with 10s timeout |
| "Docker HEALTHCHECK" | NOT CLAIMED | **UNVERIFIED** | No HEALTHCHECK in Dockerfile; not documented |

### 3.2 Security Guarantees

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Authentication on every request" | ARCHITECTURE.md:403, threat-model.md:62 | **TRUE** | `internal/middleware/auth.go` — JWT validation on protected routes |
| "Authorization checked at service and data layers" | ARCHITECTURE.md:404 | **TRUE** | `RequireAnyRole` middleware + service-layer ownership checks |
| "All input validated and sanitized" | ARCHITECTURE.md:405 | **PARTIALLY TRUE** | Parameterized queries verified; JSON field validation inconsistent per Stage 0.9 audit |
| "No execution of code from user-supplied data" | ARCHITECTURE.md:407 | **TRUE** | Rules engine uses sandboxed operators; no eval/code execution |
| "Secrets are never logged" | ARCHITECTURE.md:408 | **TRUE** | No secret logging found in code review |
| "Rate limiting on all endpoints" | ARCHITECTURE.md:409 | **PARTIALLY TRUE** | User rate limiter exists (20 req/5min); login endpoint has no brute-force protection |
| "JWT tokens are short-lived (24h)" | ARCHITECTURE.md:410 | **TRUE** | JWT expiry configurable; default not verified in code |
| "JWT tokens are non-revocable" | ARCHITECTURE.md:410 | **TRUE** | No revocation mechanism found; documented as known limitation |
| "BCrypt passwords" | CLAIMED | **TRUE** | `internal/identity/domain/user.go:63` — `bcrypt.GenerateFromPassword` |
| "Passwords stored using a strong, slow hash (Argon2id or bcrypt)" | threat-model.md:99 | **TRUE** | bcrypt used (not Argon2id, but bcrypt is acceptable) |
| "Session tokens are random, short-lived, and revocable" | threat-model.md:100 | **PARTIALLY TRUE** | Tokens are random and short-lived; no revocation found |
| "No CSRF protection" | NOT CLAIMED | **TRUE** | API-first design; JWT in Authorization header, not cookies |
| "CORS not configured" | NOT CLAIMED | **TRUE** | No CORS headers set; acceptable for API-first SPA design |

### 3.3 Encryption

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "TLS 1.2+ in transit (required in production)" | ARCHITECTURE.md:392 | **PARTIALLY TRUE** | No TLS code found in server; TLS expected to be handled by reverse proxy/operator |
| "Encryption at rest" | ARCHITECTURE.md:394 | **FALSE** | No encryption at rest implemented; marked "(Future)" in docs |
| "Key management via KMS" | ARCHITECTURE.md:394 | **FALSE** | No KMS integration found; marked "(Future)" |

### 3.4 Audit Immutability

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Tamper-evident, hash-chained audit trail" | README.md:73-75 | **FALSE** | `internal/audit/infrastructure/postgres/audit_repository.go:314-383` — `PurgeOld` DELETES old audit events and REBUILDS the hash chain. This completely breaks tamper evidence for purged records. |
| "Audit records are append-only" | threat-model.md:170 | **FALSE** | `PurgeOld` executes `DELETE FROM audit.audit_events` |
| "Audit log deletion is not supported (only retention-based archival)" | threat-model.md:175 | **FALSE** | `PurgeOld` deletes records; no archival mechanism found |
| "Hash chain per organization" | ARCHITECTURE.md:81 | **PARTIALLY TRUE** | Hash chain exists for current records; purged records have chain rebuilt from remaining data |
| "Audit writes occur in the same database transaction as the state change" | Stage 01 audit | **PARTIALLY TRUE** | Core modules use transactional audit; AI module uses `Save` outside transaction per Stage 01 findings |
| "Integrity verification on startup and background ticks" | README.md:349-350 | **UNVERIFIED** | No startup verification or background tick code found |

### 3.5 Tenant Isolation

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "All data scoped by organization_id" | README.md:385, ARCHITECTURE.md:376 | **TRUE** | All tables have `organization_id`; all queries filtered |
| "Queries filtered by organization at data-access layer" | ARCHITECTURE.md:377 | **TRUE** | Repository implementations consistently include `organization_id` in WHERE clauses |
| "Database foreign keys enforce ownership" | ARCHITECTURE.md:378 | **TRUE** | FK constraints with `organization_id` found in migrations |
| "Middleware validates orgId matches JWT claim" | README.md:390-391 | **TRUE** | `RequireSameTenant` in `internal/middleware/auth.go:122-145` |
| "Service level: all methods organization-scoped" | README.md:392 | **TRUE** | All service methods accept `orgID` as first parameter |
| "No cross-tenant access possible" | README.md:498 | **PARTIALLY TRUE** | Code enforces isolation; no proven IDOR test coverage for all endpoints per Stage 0.9 |

### 3.6 Scalability

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Horizontal scaling (if monolith-to-service extraction is needed)" | ROADMAP.md:159 | **UNVERIFIED** | No load balancer config, no session affinity handling, no scaling guides |
| "Performance testing and optimization" | ROADMAP.md:155 | **UNVERIFIED** | No load test suite found |
| "Database connection pooling" | IMPLIED | **TRUE** | Standard `database/sql` pooling; no custom config found |
| "Low resource footprint" | README.md:317 | **PARTIALLY TRUE** | 512MB RAM minimum claimed; no benchmark data found |

### 3.7 Backup / Retention

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Backup and disaster recovery procedures" | ROADMAP.md:157 | **FALSE** | No backup code, no backup docs, no DR procedures found |
| "Data retention is configurable per organization and data type" | ARCHITECTURE.md:387 | **FALSE** | No organization-level retention settings found; `PurgeOld` uses global `olderThan` parameter |
| "Default retention: 7 years for audit records" | ARCHITECTURE.md:388 | **UNVERIFIED** | No default retention config found; `PurgeOld` exists but default cutoff not set |
| "Retention policies for ai_observations, case_summaries, document_analyses" | Stage 01 audit recommendation | **FALSE** | No retention policies implemented for these tables |

### 3.8 AI Safety

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "AI observation assistance is available in 0.8, optional and off by default" | README.md:508 | **TRUE** | `CIVORA_AI_ENABLED` defaults to `false`; `NoopProvider` is default |
| "CIVORA's foundation remains deterministic infrastructure" | README.md:511 | **TRUE** | Workflow, forms, rules, audit, decisions are deterministic |
| "AI is an observation aid, never an autonomous actor" | README.md:513 | **TRUE** | AI module has no imports of `decisions`, `workflow`, `rules`, `forms` |
| "No code path from AI outputs to decisions/workflow transitions/rule evaluation" | ARCHITECTURE.md:144-151 | **TRUE** | Verified by import analysis; `internal/ai/application/service.go` imports only `audit`, `evidence`, `form_submission` |
| "AI outputs require explicit human review" | ARCHITECTURE.md:153 | **TRUE** | Observations have ACCEPTED/REJECTED/CORRECTED/DISMISSED states |
| "Enforced by absence of code paths, not policy alone" | ARCHITECTURE.md:155 | **TRUE** | Architectural boundary enforced at module level |
| "AI cannot automatically approve, reject, or distribute assistance" | README.md:156 | **TRUE** | No AI-to-decision write path exists |
| "CIVORA does not use AI for consequential decisions" | README.md:380-381 | **TRUE** | AI module does not write to decisions/workflow/rules |

### 3.9 Compliance

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "GDPR compliance" | NOT CLAIMED | **UNVERIFIED** | No GDPR-specific code found (data export, right-to-deletion, etc.) |
| "SOC 2 compliance" | NOT CLAIMED | **UNVERIFIED** | No SOC 2-specific controls found |
| "Audit trail for compliance" | IMPLIED | **PARTIALLY TRUE** | Audit trail exists but immutability claim is false (see 3.4) |
| "Data classification labels" | ARCHITECTURE.md:382 | **FALSE** | No data classification system found; marked "(Future)" |

### 3.10 Interoperability

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "OpenAPI 3.0 specification as source of truth" | README.md:430, ARCHITECTURE.md:353 | **TRUE** | `api/openapi/openapi.yaml` exists; validated with redocly |
| "REST/OpenAPI interface" | README.md:434 | **TRUE** | All functionality exposed via HTTP REST |
| "Apache 2.0 license" | README.md:449 | **TRUE** | `LICENSE` file present |
| "Single static Go binary" | README.md:108 | **TRUE** | Verified in Dockerfile and build |
| "PostgreSQL database" | README.md:429 | **TRUE** | PostgreSQL 16 used throughout |
| "No cloud-specific dependencies" | README.md:109 | **TRUE** | No cloud SDK imports found |
| "No mandatory SaaS" | README.md:109 | **TRUE** | Self-hosted; AI providers are optional |

### 3.11 Configurability

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Workflows, forms, and rules are configuration, not source code" | README.md:101-103 | **TRUE** | All defined via API; stored in database |
| "New processes added by creating definitions through API" | README.md:103 | **TRUE** | Workflow definitions, forms, rules all API-managed |
| "13 field types" | README.md:320-321 | **TRUE** | 13 field types implemented: TEXT, TEXTAREA, NUMBER, DECIMAL, DATE, DATETIME, BOOLEAN, SELECT, MULTISELECT, RADIO, CHECKBOX, EMAIL, PHONE |
| "Form versioning" | README.md:322 | **TRUE** | `form_versions` table with publish/unpublish lifecycle |
| "State assignment: pin form to workflow state" | README.md:324 | **TRUE** | `workflow_state_form_assignments` table exists |
| "Organizations can define any combination of fields" | README.md:333 | **TRUE** | Forms are fully dynamic via API |
| "service_type configurable" | Stage 11 fix | **TRUE** | Hardcoded enums removed; free-text string |
| "priority configurable" | Stage 11 fix | **TRUE** | Hardcoded enums removed; free-text string |

### 3.12 Workflow Capabilities

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Configurable state machine" | README.md:59 | **TRUE** | Workflow definitions with states and transitions |
| "Versioned workflow definitions" | README.md:251 | **TRUE** | `workflow_definitions` has version column; new cases use latest active version |
| "Terminal states" | README.md:198 | **TRUE** | `terminal` flag on workflow states; `IsClosed` check |
| "Role-based authorization per transition" | README.md:199, ARCHITECTURE.md:314 | **TRUE** | `allowed_roles` on transitions; checked in service |
| "Workflow instance is source of truth" | ARCHITECTURE.md:299-301 | **TRUE** | Case has `workflow_instance_id` FK; state synced from instance |
| "Optimistic concurrency control" | ARCHITECTURE.md:324 | **TRUE** | `version` column on cases; prevents lost updates |
| "Transition history immutable" | ARCHITECTURE.md:214 | **TRUE** | `workflow_transition_history` is append-only |

### 3.13 AI Safety Boundaries (Detailed)

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "AI module imports: no decisions, workflow, rules, forms" | ARCHITECTURE.md:144-151 | **TRUE** | `internal/ai/application/service.go` imports only `audit`, `database`, `evidence`, `form_submission`, `shared` |
| "AI outputs require human review" | ARCHITECTURE.md:153 | **TRUE** | Observation statuses: OPEN, PENDING_REVIEW, ACCEPTED, REJECTED, CORRECTED, DISMISSED |
| "Prompt injection guard" | ARCHITECTURE.md:133 | **TRUE** | `PromptInjectionGuard` in AI infrastructure |
| "PII sanitizer" | ARCHITECTURE.md:133 | **PARTIALLY TRUE** | Sanitizer exists but document content not retrieved for AI (accidental protection) |
| "AI audit trail" | ARCHITECTURE.md:145 | **PARTIALLY TRUE** | Audit events created but not transactional with data writes in all paths |

### 3.14 Deployment Support

| Claim | Source | Classification | Evidence |
|-------|--------|----------------|----------|
| "Docker container image" | README.md:433 | **TRUE** | Multi-stage Dockerfile produces minimal Alpine image |
| "Docker Compose for local development" | README.md:434 | **TRUE** | `docker-compose.yml` with PostgreSQL |
| "Self-hosted" | README.md:88 | **TRUE** | No cloud dependencies |
| "Minimal base image" | ARCHITECTURE.md:427 | **TRUE** | Alpine Linux base |
| "Non-root container user" | ARCHITECTURE.md | **TRUE** | `civora` user created in Dockerfile |
| "Kubernetes manifests" | ARCHITECTURE.md:429 | **STALE** | Claimed as "future"; no K8s manifests exist |

---

## 4. Critical Findings

### 4.1 FALSE: Audit Immutability Claim

**Severity:** CRITICAL  
**Claim:** "Tamper-evident, hash-chained audit trail" and "audit records are immutable"  
**Reality:** `internal/audit/infrastructure/postgres/audit_repository.go:314-383` implements `PurgeOld` which:
1. DELETES old audit events (`DELETE FROM audit.audit_events WHERE organization_id = $1 AND timestamp < $2`)
2. REBUILDS the hash chain from remaining events (`rebuildAuditChain`)

This means:
- Audit events are NOT immutable — they can be deleted
- The hash chain can be recalculated after deletion, destroying historical integrity
- An operator with database access can alter history by selectively deleting and rebuilding

**Impact:** The core audit guarantee is false. Operators relying on audit immutability for compliance or forensic purposes are misled.

**Recommendation:** 
- Either remove `PurgeOld` and implement true append-only storage
- Or clearly document that purge operation breaks tamper evidence and requires re-verification
- ConsiderWrite-once storage or archival to WORM storage for compliance

### 4.2 FALSE: Encryption at Rest Claim

**Severity:** HIGH  
**Claim:** "(Future: encryption at rest and key management via KMS)" in ARCHITECTURE.md:394  
**Reality:** No encryption at rest is implemented. The docs correctly mark this as future, but the threat model (T-08) says "TLS in transit; encryption at rest is planned for a future milestone" which could be misinterpreted as current.

**Impact:** Data at rest in PostgreSQL is unencrypted. Operators must rely on filesystem/database-level encryption.

### 4.3 FALSE: Backup/Disaster Recovery Claim

**Severity:** HIGH  
**Claim:** ROADMAP.md 0.9 scope includes "Backup and disaster recovery procedures"  
**Reality:** No backup code, no backup documentation, no DR procedures found anywhere in the codebase.

**Impact:** Operators have no built-in backup mechanism. Data loss from database failure would require manual recovery.

### 4.4 FALSE: Retention Configurability Claim

**Severity:** MEDIUM  
**Claim:** "Data retention is configurable per organization and data type" (ARCHITECTURE.md:387)  
**Reality:** `PurgeOld` takes a global `olderThan` timestamp. No organization-level retention settings exist.

### 4.5 PARTIALLY TRUE: Rate Limiting

**Severity:** MEDIUM  
**Claim:** "Rate limiting on all endpoints" (ARCHITECTURE.md:409)  
**Reality:** User-based rate limiter exists (20 req/5min per user). Login endpoint has no brute-force protection. No IP-based rate limiting found.

### 4.6 PARTIALLY TRUE: AI Audit Atomicity

**Severity:** MEDIUM  
**Claim:** "Audit writes occur in the same database transaction as the state change"  
**Reality:** Core modules use `RecordEventTx` within transactions. AI module uses `RecordEvent` (standalone) or `Save` outside transactions per Stage 01 audit findings.

### 4.7 STALE: Documentation References

| Document | Issue |
|----------|-------|
| README.md line 28 | "Current focus: Milestone 0.7" — should be 0.9 |
| CHANGELOG.md | No 0.8 or 0.9 entries found |
| threat-model.md line 264 | "Next review due: Before milestone 0.4" — severely outdated |
| ARCHITECTURE.md diagrams | AI module not shown in main architecture diagram |

---

## 5. True Claims Summary

The following claims are fully supported by implementation:

1. ✅ Modular monolith architecture with clean module boundaries
2. ✅ Multi-tenant isolation at middleware, service, and repository layers
3. ✅ Configurable workflow engine with versioning and terminal states
4. ✅ 13 field types in dynamic forms system
5. ✅ Deterministic rules engine with trace trees
6. ✅ Human-in-the-loop decisions with immutability (for decisions, not audit)
7. ✅ Evidence management with verification state machine
8. ✅ AI module architecturally isolated from consequential functions
9. ✅ AI disabled by default
10. ✅ OpenAPI 3.0 specification as API source of truth
11. ✅ bcrypt password hashing
12. ✅ Parameterized SQL queries (no SQL injection)
13. ✅ Path traversal prevention in file uploads
14. ✅ Docker non-root user
15. ✅ Graceful shutdown
16. ✅ Health endpoint
17. ✅ Service types and priorities are now configurable (Stage 11 fix)

---

## 6. False Claims Summary

The following claims are contradicted by implementation:

1. ❌ Audit events are immutable — `PurgeOld` deletes them
2. ❌ Encryption at rest — not implemented
3. ❌ Backup and disaster recovery — not implemented
4. ❌ Retention configurable per organization — not implemented
5. ❌ Rate limiting on ALL endpoints — login endpoint unprotected

---

## 7. Recommendations

### Immediate (Before 1.0)

1. **Fix audit immutability**: Remove `PurgeOld` or implement true WORM storage with external archival
2. **Document encryption reality**: Update ARCHITECTURE.md to clearly state encryption at rest is NOT implemented
3. **Add backup documentation**: Document operator responsibilities for backups
4. **Fix retention claims**: Update docs to reflect actual retention behavior
5. **Add login rate limiting**: Protect against brute-force attacks

### Short-term

6. **Update stale docs**: README milestone reference, CHANGELOG, threat-model review date
7. **Add AI audit atomicity tests**: Verify transactional consistency
8. **Add health check to Dockerfile**: `HEALTHCHECK` instruction
9. **Add startup audit verification**: Implement integrity check on startup

---

## 8. Conclusion

CIVORA's documentation is **generally accurate** about its core platform capabilities. The workflow engine, forms system, rules engine, tenant isolation, and AI safety boundaries are all correctly described.

However, there are **material misrepresentations** in three areas:

1. **Audit immutability** — the `PurgeOld` method fundamentally breaks the tamper-evident claim
2. **Encryption and backup** — claimed as "future" but not clearly distinguished from current capabilities
3. **Retention configurability** — claimed but not implemented

These are not minor wording issues. The audit immutability claim, if relied upon for compliance or legal purposes, could expose operators to risk.

**Recommendation:** Do not release 1.0 until audit immutability is either implemented or honestly documented as a limitation. The other findings should be addressed in documentation updates.

---

## 9. Verification Commands

```bash
# Verify audit purge functionality
grep -n "PurgeOld" internal/audit/infrastructure/postgres/audit_repository.go

# Verify password hashing
grep -n "bcrypt" internal/identity/domain/user.go

# Verify tenant isolation
grep -n "RequireSameTenant" internal/middleware/auth.go

# Verify AI imports (should NOT import decisions/workflow/rules)
grep -n "import" internal/ai/application/service.go

# Verify field types
grep -n "FieldType" internal/forms/domain/form.go

# Check for backup code
grep -rn "backup" --include="*.go" internal/

# Check for encryption at rest
grep -rn "encrypt" --include="*.go" internal/
```

---

*This audit was performed by static code inspection of the CIVORA repository at HEAD `2f517e0`. All claims were verified against actual implementation, not assumed from documentation.*
