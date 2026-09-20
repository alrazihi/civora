# CIVORA — Open Infrastructure for Public-Service Workflows & Decisions

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![CI](https://img.shields.io/github/actions/workflow/status/alrazihi/civora/ci.yml?branch=main&label=CI)](https://github.com/alrazihi/civora/actions)
[![Code of Conduct](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa)](./CODE_OF_CONDUCT.md)

CIVORA is an open-source, compliance-oriented workflow platform designed around
structured cases, configurable workflows, dynamic forms, deterministic rules,
evidence management, human decisions, auditability, multi-tenancy, authorization,
and observability.

## Mission

Governments, NGOs, humanitarian organizations, and other public-interest
institutions should be able to create, operate, audit, and improve complex
service-delivery processes without proprietary lock-in.

CIVORA provides the open digital infrastructure to make that possible.

## Current Status

**CIVORA 1.0.0 — Production-ready baseline.**

The 1.0 release establishes the core platform foundation: a configurable
workflow engine, dynamic forms, deterministic rules, evidence management,
human decisions, and a tamper-evident audit trail, all operating within a
multi-tenant, authorization-enforced architecture.

Production-ready baseline does not mean every enterprise or compliance feature
is implemented. Known limitations are documented below. The platform is
suitable for organizations that require auditable, configurable service-delivery
workflows and are prepared to manage operational responsibilities such as
backups, TLS, and secrets management.

## What CIVORA Is

CIVORA is an **open-source workflow platform** for public-service and
humanitarian organizations that need to configure, operate, evaluate, and
audit multi-step service-delivery processes. It is built as **open
infrastructure** — not a single-purpose application — so that governments,
NGOs, and social-service organizations can define their own case-management
workflows, dynamic forms, and eligibility rules without modifying core code.

CIVORA brings seven capabilities together:

1. **Workflow engine** — a configurable state machine that drives the case
   lifecycle. Organizations define states, transitions, and authorization
   requirements as data (versioned, tenant-scoped), then execute transitions
   through a single, auditable API. Transitions are protected by optimistic
   concurrency control and terminal states cannot be overridden.
2. **Dynamic forms** — structured data collection with 13 field types, form
   versioning, and the ability to assign forms to specific workflow states
   (required or optional). Published versions are immutable; forms feed the
   rules engine as facts.
3. **Rules engine** — a deterministic, auditable eligibility engine that
   evaluates rule sets against case facts. Rule sets are configured as JSON,
   versioned, and produce full trace trees explaining every comparison. Rules
   never make final decisions; they inform them.
4. **Evidence** — document and metadata management with a verification lifecycle
   (UNVERIFIED → VERIFIED / REJECTED / NEEDS_REVIEW), metadata validation,
   storage-reference protection, and concurrent verification safety via
   row-level locking.
5. **Human decisions** — rules evaluate, but humans decide. Every consequential
   decision is recorded by an authorized actor, with rationale, versioned, and
   immutable once finalized. Supersession preserves the full history.
6. **Audit** — every action that affects state produces an immutable,
   hash-chained audit event recorded in a dedicated PostgreSQL schema. The chain
   is tamper-evident within the application/database boundary.
7. **Multi-tenancy and authorization** — all data is scoped by `organization_id`.
   Tenant isolation is enforced at the database, service, and API layers.
   Role-based authorization controls access within each organization.

## Problem Statement

Public-interest institutions typically deliver complex services using
proprietary platforms, spreadsheets, email, and shared drives. The result is
vendor lock-in, fragmented tooling, and no meaningful audit trail for the
decisions that affect people's lives.

CIVORA addresses this by providing a single, open, modular platform that
institutions can self-host, audit, and extend:

- **Configurable, not coded** — workflows, forms, and rules are configuration,
  not source code. New processes are added by creating definitions through the API.
- **Auditable by design** — every action that affects state produces an
  immutable, hash-chained audit event.
- **Human authority is preserved** — rules inform, but humans decide. CIVORA
  never makes consequential decisions about people autonomously.
- **Open and portable** — Apache 2.0, single static Go binary, PostgreSQL, and
  Docker. No cloud-specific dependencies, no mandatory SaaS.

## Architecture at a Glance

CIVORA uses a **modular monolith** architecture with clear module boundaries.
Each module owns its data and communicates through internal service interfaces.

| Layer | Component |
|-------|-----------|
| HTTP API | OpenAPI 3.0.3, versioned at `/api/v1/`, authenticated via Bearer tokens |
| Application Services | 14 domain modules: identity, organizations, cases, workflow, forms, rules, evidence, decisions, assistance, follow-ups, people, assessments, audit, operations |
| AI Module | Observation generation, case context, case summary, document intelligence — architecturally isolated from consequential functions |
| Shared Infrastructure | Configuration, structured logging, metrics (application-level), request ID propagation |
| Data Layer | PostgreSQL 16 primary database; object storage (S3-compatible) for documents |

The backend is the sole authority for workflow state, transition validation,
authorization, and audit. The frontend is a lightweight static HTML/CSS/JS SPA
served from `web/`.

## Security Model

### Authentication

Authentication uses short-lived JWT access tokens (default 20-minute expiry)
validated against server-side session state on every request. Refresh tokens
are hashed in PostgreSQL, rotated on each use with a new token family, and
tracked in the `auth_sessions` table.

### Session Lifecycle

The session architecture uses a two-level repository API:

- **Pre-authentication (refresh-token lookup):** `FindByRefreshTokenHash(...)`
  does not require `organization_id` because the refresh-token flow legitimately
  occurs before tenant context is established from the request.
- **Authenticated context:** `FindByIDForOrganization(...)`,
  `FindActiveByUserIDForOrganization(...)`, `RevokeForOrganization(...)`,
  `MarkUsedForOrganization(...)`, and related org-aware methods provide
  defense-in-depth by enforcing tenant boundaries at the data layer.

This design exists because refresh-token validation happens before the JWT's
`organization_id` claim can be extracted. Once authenticated, every session
operation is organization-scoped.

### Refresh-Token Rotation

Each refresh issues a new refresh token and a new token family. The old
`refresh_token_hash` is overwritten in the database, so reuse of a stolen
refresh token results in `ErrSessionNotFound` and is rejected with HTTP 401.
The token family is also rotated, enabling family-level revocation.

### Password-Change and Role-Change Invalidation

`ChangePassword` and role changes call `RevokeAllUserSessions(userID, orgID)`,
invalidating all active sessions for that user within the organization. This
forces re-authentication and ensures stale credentials or elevated privileges
cannot be reused.

### JTI/Session Binding

Each access token carries a JWT ID (`jti`) claim that matches the `auth_sessions.id`
column. The `AuthRequired` middleware looks up the session via
`FindByIDForOrganization(jti, orgID)` and validates that the session is not
revoked, not expired, and that the JWT's `user_id` and `organization_id` claims
match the session record. The session's `last_used_at` is updated on each request.

### Cross-Tenant Protection

Cross-tenant access is rejected at three layers:

- **API middleware:** `RequireSameTenant` ensures the path `orgId` matches the
  JWT's `organization_id` claim.
- **Service layer:** All service methods are organization-scoped.
- **Repository layer:** All queries filter by `organization_id`.

## Workflow Engine

CIVORA includes a first-class configurable workflow engine. Workflow Definitions
are versioned, tenant-scoped configurations that describe states, transitions,
and authorization requirements.

### Key Properties

- **Deterministic state machine:** `ValidateWorkflowDefinition` enforces unique
  state keys, unique transition keys, valid state references, and no outgoing
  transitions from terminal states.
- **Optimistic concurrency:** `ExecuteTransition` uses a `version` column.
  Conflicting simultaneous transitions return `ErrConcurrentModification`.
- **Transition history:** Every transition is recorded in
  `WorkflowTransitionHistory` with actor, reason, timestamp, and optional
  decision linkage.
- **Terminal-state protection:** States marked as terminal cannot have outgoing
  transitions. The engine rejects attempts to transition from a terminal state.
- **Authorization:** Transitions can declare `allowed_roles`. The workflow
  service checks the actor's role against this list server-side.

### Versioning

Workflow Definitions are versioned. Existing cases retain the definition/version
they started with. Only new cases use the latest active version.

## Dynamic Forms

CIVORA's dynamic forms platform supports 13 field types (text, textarea, number,
decimal, date, datetime, boolean, select, multiselect, radio, checkbox, email,
phone). Form definitions are versioned and can be assigned to specific workflow
states (required or optional).

### Key Properties

- **Versioning:** Published versions are immutable. New versions are created by
  cloning. Existing submissions reference the version they were created against.
- **Validation:** Required fields, min/max constraints, regex patterns, and type
  checking are enforced on both client and server.
- **Concurrency protection:** Concurrent duplicate field-key creation is
  prevented by unique constraints (≤1 success in concurrent attempts).
- **Publication behavior:** Only `DRAFT` definitions can be modified. `ACTIVE`
  and `ARCHIVED` definitions are immutable.

## Rules

The rules engine is designed as a deterministic, pure evaluation engine. Rule
sets are configured as JSON, versioned, and evaluated against a frozen snapshot
of case facts.

### Key Properties

- **Deterministic evaluation:** `Evaluate(ruleSet, facts, evaluatedAt, evaluatedBy, trigger)`
  performs no I/O and produces the same result for the same inputs.
- **Trace generation:** Every evaluation produces a full trace tree showing
  what was checked, what values were used, and why each condition passed or
  failed.
- **Versioning:** Rule sets follow a DRAFT → PUBLISHED → ARCHIVED lifecycle.
  Published versions are immutable.
- **No autonomous decisions:** Rules produce advisory outcomes (ELIGIBLE,
  INELIGIBLE, REQUIRES_REVIEW, INFORMATION_REQUIRED, FLAG, SCORE, ERROR).
  Final decisions are always made by authorized humans.

## Evidence

The evidence subsystem manages document references and metadata linked to cases.

### Verification Lifecycle

Evidence items move through a state machine: UNVERIFIED → VERIFIED / REJECTED /
NEEDS_REVIEW. Transitions are validated server-side and recorded in an
immutable append-only history.

### Metadata Validation

- Maximum metadata size: 100,000 bytes (JSON-serialized).
- Maximum description length: 5,000 characters.
- Storage references must be valid URIs with schemes `s3`, `gs`, `azureblob`,
  or `civora`. Local paths, credentials, query parameters, and path-traversal
  sequences are rejected.

### Storage-Reference Protection

The `storage_reference` field is accepted on input but intentionally excluded
from API responses. The `Document` entity's `StorageKey` field is tagged with
`json:"-"` and is never serialized. This prevents internal storage keys from
leaking to clients.

### Concurrent Verification Protection

Verification updates use row-level locking (`SELECT ... FOR UPDATE`) inside a
database transaction. This prevents concurrent verification races where two
actors might attempt to verify or reject the same evidence simultaneously.

## Human Decisions

The decisions module records consequential human decisions with full provenance.

### Versioning and Supersession

Each decision has a `Version` integer. `NewSupersedingDecision` creates a new
decision with `version = supersededVersion + 1` and sets the old decision's
`SupersededByID`. The unique index on `(organization_id, service_request_id, version)`
ensures version integrity. `FindByServiceRequest` returns the latest version.

### Provenance

Decisions link to the workflow state, rule evaluation IDs, evidence IDs, form
submission ID, and review queue entry ID at the time of decision. This preserves
the complete context for auditors.

### Relationship to Other Subsystems

- **Workflow:** Decisions trigger workflow transitions atomically (decision +
  transition + audit in a single database transaction).
- **Rules:** Rule evaluation outcomes inform decisions but never bypass them.
- **Evidence:** Decisions reference the evidence items that supported them.
- **Review queue:** Review queue entries track the assignment, review, and
  decision workflow for human reviewers.

## Audit

Every action that affects state in CIVORA produces an audit event recorded in a
dedicated PostgreSQL schema (`audit`), isolated from the `public` schema that
holds business data.

### Hash-Chain Integrity

Each event carries a SHA-256 hash linked to the previous event for the same
organization. The chain is computed over: organization ID, actor, action,
resource type, resource ID, outcome, request ID, metadata, timestamp, and
previous hash. `VerifyChain` validates the entire chain for an organization.
Background maintenance verifies integrity periodically.

### Transactional Recording

Audit writes occur in the same database transaction as the state change they
record. If the audit write fails, the state change is rolled back. This ensures
audit completeness.

### Important Limitation

The hash chain is tamper-evident within the application/database boundary. It
is not an externally anchored immutable ledger. A compromised application
process or database administrator with direct database access can write a valid
chain with false contents. External append-only anchoring is a post-1.0
enhancement.

## Observability

### Request IDs

Every incoming request is assigned a `X-Request-ID` UUID. If the client
supplies one, it is validated and preserved; otherwise a new UUID is generated.
The ID is propagated through the request context and included in all log entries
and audit events, enabling end-to-end tracing.

### Structured Logging

Logs are emitted as JSON to stdout/stderr with the following fields: `time`
(RFC3339Nano UTC), `level` (derived from HTTP status), `method`, `path`,
`status`, `bytes_written`, `latency_microseconds`, `request_id`, `remote_ip`,
and `user_agent`. Request and response bodies are not logged.

### Health and Readiness Endpoints

- `GET /health` — liveness probe. Returns `{"status":"ok"}`. No authentication
  required. Suitable for container health checks.
- `GET /ready` — readiness probe. Pings the database and returns `{"status":"ready"}`
  or `{"status":"not ready","error":"..."}` with HTTP 503 if the database is
  unreachable. No authentication required.

### Metrics

Application-level operational metrics are exposed via authenticated REST
endpoints under `/api/v1/organizations/{orgId}/operations/metrics/*`. Metrics
include case volume, pending reviews, workflow throughput, state duration, case
cycle time, aging cases, decision outcomes, assistance outcomes, evidence
verification status, and dashboard aggregations.

A Prometheus `/metrics` endpoint is not currently implemented.

### Sensitive-Data Logging Restrictions

- Request/response bodies are never logged.
- Export endpoints apply field-name-based sanitization, replacing values of
  fields matching patterns like `password`, `secret`, `token`, `api_key`,
  `credential`, `ssn`, `session_id`, `jti`, `jwt`, `authorization`, `cookie`,
  `csrf`, and similar with `"[REDACTED]"`.
- AI document processing applies PII redaction (SSN, credit card numbers, email,
  phone) before forwarding document content to external LLM providers, gated by
  `CIVORA_AI_PII_SANITIZATION` (default: `true`).

## Testing

### Verification Commands

```bash
go build ./...
gofmt -l .
go vet ./...
go test -short ./...
go test -p 1 -count=1 ./test/integration/...
npx @redocly/cli lint api/openapi/openapi.yaml
cd web/e2e && npm test
```

### 1.0 Verification Results

| Gate | Result | Notes |
|------|--------|-------|
| Build | PASS | `go build ./...` compiles cleanly |
| Formatting | PASS | `gofmt -l .` reports no unformatted files |
| Vet | PASS on Linux CI | Windows OOM limits prevent `go vet` on that platform (documented in AGENTS.md) |
| Unit tests | 60 packages, ALL PASS | `go test -short ./...` |
| Integration tests | 66 packages, all pass | `go test -p 1 -count=1 ./test/integration/...` |
| Security tests | 22 scenarios, all pass | Cross-tenant, session lifecycle, concurrency, tampering |
| Playwright E2E | 120 specs, all pass | `web/e2e/` — static mock, runs fully offline |
| Go E2E | Compiled | `test/e2e/` requires PostgreSQL; skipped with `-short` |
| OpenAPI lint | Valid | `npx @redocly/cli lint api/openapi/openapi.yaml` |
| Migrations | 44 files, correct order | Reversible; no data-loss migrations |

### Security Test Evidence

| Scenario | Result |
|----------|--------|
| Login returns access and refresh tokens | PASS |
| Refresh token rotation | PASS |
| Refresh token reuse detection | PASS |
| Stolen refresh token revocation | PASS |
| Concurrent refresh (exactly 1 success) | PASS |
| Revoked session access denied | PASS |
| Wrong-tenant session denied | PASS |
| Role change takes effect on next login | PASS |
| Signing key rotation validates tokens | PASS |
| Tenant isolation at API layer | PASS |
| Unauthorized cross-tenant decision blocked | PASS |
| Cross-tenant evidence read denied | PASS |
| Cross-case evidence read denied | PASS |
| Evidence storage reference not leaked | PASS |
| Audit trail recorded for service requests | PASS |
| Concurrent case transitions (exactly 1 success) | PASS |
| Concurrent workflow transitions (exactly 1 success) | PASS |
| Concurrent duplicate assistance (all succeed, idempotent) | PASS |
| Concurrent duplicate decision (exactly 1 success) | PASS |
| Concurrent form archiving (≥1 success) | PASS |
| Concurrent version publishing (≥1 success) | PASS |
| Concurrent duplicate field keys (≤1 success) | PASS |

> **Note:** The Go race detector (`-race`) may fail on Windows due to memory
> limits. CI runs on Ubuntu and includes race testing where applicable.

## Known Limitations

### P2 (Documented, Desirable for Future Release)

- Windows `go vet` may fail due to platform OOM limits; CI on Ubuntu passes.
- Audit chain is tamper-evident, not tamper-proof against a privileged insider
  or database administrator with direct database access. External append-only
  anchoring is a post-1.0 enhancement.
- Frontend custom-workflow transition mapping is heuristic for workflows not
  explicitly mapped; the backend still enforces authorization server-side.

### P3 (Explicitly Deferred to Post-1.0)

- Encryption at rest by default (operator responsibility to configure
  PostgreSQL-level or disk-level encryption).
- Built-in backup and disaster recovery (operator responsibility to configure
  PostgreSQL backups and test restores).
- GDPR data export/deletion APIs.
- SOC 2 / HIPAA technical safeguards (operator responsibility to implement
  through deployment configuration, policies, and controls).
- Prometheus `/metrics` endpoint.
- Real-time updates via WebSocket.
- Offline support / service worker.
- Bulk operations.
- Frontend unit test framework (Jest/Vitest).

## Quick Start

```bash
# Start PostgreSQL
docker compose up -d db

# Run the server
go run ./cmd/civora

# Seed demo data (in another terminal)
go run ./cmd/seed
```

Open http://localhost:8080 and sign in with the demo credentials printed by the
seed command.

### Configuration

CIVORA is configured via environment variables. Key variables:

| Variable | Purpose |
|----------|---------|
| `CIVORA_DB_DSN` | PostgreSQL connection string |
| `CIVORA_JWT_SECRET` | Secret for signing access tokens |
| `CIVORA_SERVER_PORT` | HTTP listen port (default: 8080) |
| `CIVORA_AI_ENABLED` | Enable AI features (default: `false`) |
| `CIVORA_SERVER_TRUSTED_PROXIES` | IP/CIDR of trusted reverse proxies |
| `CIVORA_SERVER_FORCE_HTTPS` | Emit HSTS when behind trusted proxy |
| `CIVORA_SERVER_TLS_CERT` | Direct TLS certificate path |
| `CIVORA_SERVER_TLS_KEY` | Direct TLS key path |

See `docs/architecture/configuration.md` for the full list.

## Documentation

| Topic | Location |
|-------|----------|
| Vision and mission | [docs/vision.md](docs/vision.md) |
| Project principles | [docs/principles.md](docs/principles.md) |
| Architecture overview | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Architecture decision records | [docs/decisions](docs/decisions) |
| Security reference | [SECURITY.md](SECURITY.md) |
| Threat model | [docs/threat-model.md](docs/threat-model.md) |
| Rules engine | [docs/architecture/rules-engine.md](docs/architecture/rules-engine.md) |
| Hostile review | [docs/hostile-review.md](docs/hostile-review.md) |
| Development workflow | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Agent instructions | [AGENTS.md](AGENTS.md) |
| 1.0 Verification record | [docs/releases/CIVORA-1.0.0-VERIFICATION.md](docs/releases/CIVORA-1.0.0-VERIFICATION.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| Governance model | [GOVERNANCE.md](GOVERNANCE.md) |
| Code of conduct | [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) |
| Roadmap | [ROADMAP.md](ROADMAP.md) |

## License

[Apache License 2.0](./LICENSE)

Copyright 2026 CIVORA contributors.
