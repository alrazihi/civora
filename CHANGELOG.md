# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-09-20

### Added

- Configurable workflow engine with state machines, transitions, and optimistic concurrency control
- Dynamic forms with 13 field types, versioning, and workflow state assignments
- Deterministic rules engine with JSON-configured rule sets, trace generation, and temporal operators
- Human-in-the-loop decision making with review queue, provenance links, and immutable history
- Evidence domain with document storage, verification lifecycle, and audit integration
- AI observation assistance with architectural isolation from consequential functions
- Case summaries and document intelligence integration
- Operations metrics, workflow analysis, and impact intelligence dashboards
- Two-level session repository API with tenant-aware validation
- Refresh token rotation, reuse detection, and stolen token revocation
- Cross-tenant access rejection at middleware, service, and repository layers
- Tamper-evident SHA-256 hash-chained audit trail in dedicated `audit` schema
- Platform configurability validation with zero source-code changes for distinct processes
- Comprehensive security test suite covering 30+ attack vectors
- Playwright frontend E2E suite with 120 passing specs
- OpenAPI 3.0.3 specification with full API coverage

### Security

- Authentication and authorization on every request
- Tenant isolation at database, service, and API layers
- Timing-safe authentication paths and generic rate-limit responses
- Evidence state-machine enforcement and decision supersession integrity
- Concurrent workflow transitions: exactly one success per race
- Concurrent duplicate assistance creation: all succeed (idempotent)
- Concurrent form field creation: ≤1 success (unique constraint)
- Stolen refresh token detection and automatic revocation
- Password change and role change invalidate all active sessions
- JTI validation and signing-key rotation support
- Cross-tenant session and resource access rejection
- Audit hash-chain integrity verification with background maintenance

### Changed

- OpenAPI spec version bumped to 1.0.0
- Project status updated to production-ready 1.0.0

### Known Limitations

- No encryption at rest by default (operator responsibility)
- No built-in backup/DR (operator responsibility)
- No GDPR data export/deletion APIs (post-1.0)
- No SOC 2 / HIPAA technical safeguards (operator responsibility)
- Audit chain is tamper-evident, not tamper-proof against privileged insider/DBA
- Frontend transition mapping is heuristic for custom workflows (backend still enforces authorization)
- Windows `go vet` may fail due to platform OOM limits (CI on Ubuntu passes)

[Unreleased]: https://github.com/alrazihi/civora/compare/v0.9.0...HEAD
[v1.0.0]: https://github.com/alrazihi/civora/releases/tag/v1.0.0
[v0.9.0]: https://github.com/alrazihi/civora/releases/tag/v0.9.0

## [0.9.0] - 2026-09-19

### Evidence Domain

- `evidence` module: full evidence domain with document storage, verification
  lifecycle, and audit integration
- Evidence CRUD: create, read, list by case, update metadata
- Document storage: upload (multipart), download (streaming by document ID),
  list, delete with server-generated storage keys
- Verification state machine: UNVERIFIED → VERIFIED / REJECTED / NEEDS_REVIEW
  with immutable append-only history
- Local filesystem storage provider with server-generated keys, SHA-256
  checksum, 10MB size limit, MIME allow-list, filename sanitization, and path
  traversal prevention
- Audit integration: hash-chained events for all evidence operations
- Drag-and-drop upload UI in the evidence modal with preview thumbnails
- OpenAPI documentation for all evidence and document endpoints

### Security

- Tenant isolation enforced at repository, service, and API layers
- IDOR prevention via organization-scoped queries
- Upload security: filename sanitization, content-type validation, size limits,
  checksum verification
- Download security: storage keys never exposed in responses
- Path traversal prevention in file retrieval and deletion

### Documentation

- 9 stage reports documenting the evidence domain implementation
- Final release gate with architecture, security, and test verification
- OpenAPI version bumped to 0.7.0

### Changed

- OpenAPI spec: added evidence and document endpoints; version bumped to 0.7.0
- Download route: changed to per-document download
  (`/evidence/{evidenceId}/documents/{documentId}/download`)

### Tests

- `go test -short ./...` — PASS
- `npx playwright test --workers=1` — PASS (69 tests)
- `npx @redocly/cli lint` — VALID
- `go vet ./...` — PASS
- `gofmt -l .` — CLEAN
- `go build ./...` — PASS

---

## [0.6.0] - 2026-09-16

### Added

- `decisions` module: human decision domain with full provenance (case, workflow state, rule evaluations, evidence, form submissions)
- `review_queue` module: human review work queue with state machine (PENDING → ASSIGNED → IN_REVIEW → COMPLETED/ESCALATED/WAITING_INFORMATION)
- Decision versioning and supersession: immutable history with audit trail
- Decision types: APPROVED, REJECTED, NEEDS_MORE_INFORMATION, ESCALATE with configurable reason requirements
- Secure review assignment with PostgreSQL SKIP LOCKED for concurrency safety
- Human decision actions: CompleteReview (approve/reject), EscalateReview, RequestInformation
- Decision-to-workflow-transition mapping via `decision_type` column on workflow transitions
- Atomic decision + workflow transition + audit in single database transaction
- Workflow transition observer pattern for extensibility
- Comprehensive security test suite (30 attack vectors covering cross-tenant access, IDOR, impersonation, replay, concurrency, tampering, audit manipulation)
- Platform configurability validation: two distinct processes (Emergency Assistance + Education Assistance) with zero source-code changes
- Reviewer workspace frontend: case context, rule evaluations with trace, evidence, form submissions, previous decisions, decision controls
- Historical reproducibility: decisions reference exact rule/form/evidence versions; supersession preserves audit chain

### Changed

- OpenAPI spec: extended with review queue and decision endpoints; version bumped to 0.5.0
- Workflow transitions: added `decision_type` column for human decision integration
- Review queue provenance: added `evidence_ids` and `form_submission_id` columns
- Decision provenance: linked to review queue entries and workflow transition history

### Security

- Cross-tenant isolation enforced at repository, service, and API layers
- IDOR prevention via organization-scoped queries
- Authorization checked server-side on every operation
- Concurrent assignment/decision safety via DB-level locking
- Replay, duplicate, and stale-state protection via state machine
- Payload size limits (5000 char reason max)
- Error leakage controlled

### Fixed

- Decision domain: reason validation for REJECTED/ESCALATE/NEEDS_MORE_INFORMATION
- Review queue: state machine transitions enforce valid flows
- Workflow integration: atomic decision+transition prevents orphaned records

---

## [0.8.0] - 2026-09-17

### AI Platform

- AI module: first-class platform capability with provider abstraction
  (`OpenAIProvider`, `LocalProvider`, `NoopProvider`)
- AI is off by default via `CIVORA_AI_ENABLED` (default: `false`)
- Evidence-level observations: generate AI summaries and entity extractions
  from uploaded documents
- Case-level observations: generate case-centric summaries, missing information
  detection, and relevant evidence identification
- Human-in-the-loop verification: accept, reject, correct, or dismiss
  observations before creating verified facts
- Verified facts: immutable provenance records linking accepted AI output
  to cases with reviewer attribution and input/output hashes
- PII sanitization: mandatory redaction of sensitive data before sending
  document content to external LLM providers
- Prompt injection defense: system-prompt instructions plus architectural
  response validation via `PromptInjectionGuard`
- Document size limits: 100KB per document to prevent excessive context
- Input/output hashing: SHA-256 hashes for reproducibility and tamper detection

### AI Modules

- `internal/ai/domain` — observation and verified-fact aggregates
- `internal/ai/application` — AI service orchestration, provider port,
  sanitizer port, document content provider port
- `internal/ai/infrastructure/postgres` — observation and verified-fact
  repositories with tenant-scoped queries
- `internal/ai/infrastructure/provider` — OpenAI, Local/Ollama, and Noop
  providers
- `internal/ai/infrastructure/sanitizer` — PII redaction sanitizer
- `internal/ai/infrastructure/guard` — prompt injection guard
- `internal/ai/infrastructure/adapter` — document content provider adapter
- `internal/ai/api` — REST endpoints for observation generation and review

### Case Context

- `internal/casecontext/domain` — case context aggregation interfaces
  (case, person, evidence, documents, form submissions, rules, workflow,
  decisions, AI observations)
- `internal/casecontext/application` — case context builder service
- `internal/casecontext/infrastructure/postgres` — case context repository
- `internal/casecontext/infrastructure/auth` — authorization checker
- `internal/casecontext/api` — REST endpoints for building and retrieving
  case context

### Case Summary

- `internal/casesummary/domain` — case summary domain types
- `internal/casesummary/application` — case summary generation service
  with provider abstraction
- `internal/casesummary/infrastructure/postgres` — case summary repository
- `internal/casesummary/infrastructure/provider` — OpenAI case summary provider
- `internal/casesummary/api` — REST endpoint for case summary generation

### Document Intelligence

- `internal/documentintelligence/domain` — document analysis domain types
- `internal/documentintelligence/application` — document analysis service
  with provider abstraction, PII sanitization, and document content retrieval
- `internal/documentintelligence/infrastructure/postgres` — document analysis
  repository
- `internal/documentintelligence/infrastructure/provider` — OpenAI document
  analysis provider
- `internal/documentintelligence/api` — REST endpoints for document analysis
  generation, listing, accept, and reject

### Security

- Stage 8 hostile AI security review: 28 attack vectors tested, all critical
  and high-severity gaps resolved
- Prompt injection defense at both prompt layer and response validation layer
- PII sanitization enforced by default for text content types
- AI decision boundary architecturally enforced: AI has no code path to
  decisions, workflow, rules, forms, or evidence modification
- Tenant isolation enforced at all layers for AI operations
- Human verification is mandatory before any consequential AI output use
- Comprehensive audit trail for all AI operations with full provenance

### Platform Proof

- Stage 9 AI platform proof: cross-process, cross-organization verification
- Two unrelated processes tested: Emergency Assistance (Org A) and
  Education Assistance (Org B) with genuinely different forms, fields,
  evidence, rules, workflows, and decisions
- Tenant isolation proven: Org A cannot access Org B's data at any layer
- AI disable proven: NoopProvider returns disabled error; core CIVORA
  functions normally

### Production Wiring

- `cmd/civora/main.go` now registers `casecontext`, `casesummary`, and
  `documentintelligence` handlers
- Adapter layer bridges postgres repositories to `casecontext` domain
  interfaces
- `casesummary` nil-pointer panic fixed by passing real `ctxService`

### Changed

- OpenAPI spec: added AI observation, case summary, case context, and
  document intelligence endpoints
- Evidence schema: added `custom_type` column for custom evidence types
- AI observations schema: made `evidence_id` nullable for case-level
  observations
- Audit vocabulary: added `ai.verified_fact.created`,
  `ai.observations_generated`, `ai.observation.created`,
  `evidence.document_uploaded`, `evidence.document_downloaded`,
  `evidence.document_deleted`, `case_summary.generated`,
  `document_analyses_generated`, `document_analysis.created`,
  `decision.superseded`

### Tests

- `go test -short ./...` — PASS
- `go test -count=1 -run TestAIPlatformProof ./test/integration/` — PASS
- `go build ./cmd/civora` — PASS
- `go vet ./...` — PASS
- `gofmt -l .` — CLEAN

### Documentation

- Stage 8 hostile AI security review report
- Stage 9 AI platform proof report
- Architecture updated with AI module diagrams and module boundaries

## [0.9.0] - 2026-09-19

### Security & Integrity

- Audit immutability enforced: `PurgeOld` is now a no-op; audit events cannot be deleted, preserving the tamper-evident hash chain
- Login brute-force protection verified: `UserRateLimiter` enforces lockout after failed auth attempts
- AI audit atomicity verified: observation generation and review audit events are recorded within the same database transaction as data writes
- OpenAPI specification restored to valid state after modularization regression; all 200+ component files have correct JSON pointer references

### Reliability

- Database startup retry reduced from 30 to 10 attempts, cutting maximum startup wait from ~8 minutes to ~60 seconds
- Rate limiter memory bounded: LRU eviction at 10,000 entries prevents unbounded growth
- Idempotency store memory bounded: LRU eviction at 100,000 entries prevents unbounded growth
- Audit maintenance alerting: consecutive failures logged with `ALERT:` message after 3 failures

### Configurability

- Removed hardcoded `service_type` enum: organizations can now define any service type string via API
- Removed hardcoded `priority` enum: organizations can now define any priority string via API
- OpenAPI schemas updated to remove enum constraints on `service_type` and `priority`

### Documentation

- Stage 11 platform configurability audit added
- Stage 12 documentation truth audit added
- ARCHITECTURE.md updated: encryption at rest is not implemented, retention is not configurable per org, backup/DR are operator responsibilities
- README.md updated: Milestone 0.9 focus confirmed
- threat-model.md review date updated to 2026-09-19

### Known Limitations

- No per-organization AI settings in database; AI is globally enabled/disabled via environment variable
- Case.Status is a denormalized mirror of workflow state; consistency is enforced at application layer but not at database constraint level
- No built-in backup/restore or encryption-at-rest; operators must provision these through infrastructure
- No `/roles` CRUD API endpoint; roles are created via database seed scripts
- AI observation default status is `OPEN` rather than `PENDING_REVIEW`

### Tests

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l .` — CLEAN
- `go test -short ./...` — PASS
- `npx @redocly/cli lint api/openapi/openapi.yaml` — PASS

[Unreleased]: https://github.com/alrazihi/civora/compare/v0.8.0...HEAD
[v0.9.0]: https://github.com/alrazihi/civora/releases/tag/v0.9.0

---

## [0.8.0] - 2026-09-17

### Added

- Initial project structure with modular monolith architecture
- Identity module: authentication, authorization, user management
- Organizations module: tenant scoping
- Audit module: immutable event log with hash chain
- Cases module: case creation, status tracking, assignment
- OpenAPI specification with all core endpoints
- CI pipeline: `gofmt`, `go vet`, `go test`, `golangci-lint`, `gosec`
- Dockerfile and docker-compose for local development
- PostgreSQL migration system with `go:embed`
- Structured JSON logging with request correlation IDs
- Security headers (HSTS, CSP, X-Frame-Options, etc.)
- Rate limiting (per-IP and per-user)
- Body size limits and request timeouts
- DB-aware health/readiness endpoints
- Comprehensive test suite: unit, integration, E2E

### Security

- RBAC enforcement on all protected routes
- Server-side role assignment (first user = admin, others = staff)
- Atomic audit events with domain writes
- Password policy enforcement (12+ chars, letter + number)
- Email validation with regex
- Correlation IDs propagated to audit events
- Per-user rate limiting on auth endpoints with lockout after 5 failures
- OIDC remnants fully removed
- No internal error leakage in 500 responses
- Cross-tenant assignment validation

[Unreleased]: https://github.com/alrazihi/civora/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/alrazihi/civora/releases/tag/v0.1.0