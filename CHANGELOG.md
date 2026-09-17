# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] - 2026-09-16

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

## [0.8.0] - 2026-09-16

### AI Observations

- AI observation domain: observation entities with human review and provider
  abstraction (Noop, Local, OpenAI)
- Observations are off by default via `CIVORA_AI_ENABLED` (default off)
- Transactional audit integration: all AI observations produce hash-chained
  audit events
- Document content retrieval: extract text content from stored documents for
  observation input
- AI audit event types: `observation_created`, `observation_reviewed`,
  `observation_provided`

## [Unreleased]

---

## [0.1.0] - 2026-09-06

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