# CIVORA Architecture

## Overview

CIVORA uses a **modular monolith** architecture. See
[ADR-001: Initial Architecture](docs/decisions/0001-initial-architecture.md)
for the decision record and rationale.

This document describes the architecture that exists in CIVORA 1.0.0: module
boundaries, data flow, request lifecycle, session architecture, workflow
engine, dynamic forms, rules engine, evidence subsystem, human decisions,
audit subsystem, observability, persistence, transactions, and deployment
model.

---

## Architectural Principles

1. **Configuration over code** — workflows, forms, and rules are data, not
   source code. New processes are added through the API.
2. **Deterministic evaluation** — the rules engine is a pure function. Given
   the same inputs, it always produces the same output with a reproducible
   trace.
3. **Human authority** — rules inform decisions but never make them.
   Consequential decisions require an authorized human actor.
4. **Tamper-evident audit** — every state-changing action produces an
   immutable, hash-chained audit event in a dedicated database schema.
5. **Multi-tenant by default** — all data is scoped by `organization_id`.
   Tenant isolation is enforced at the database, service, and API layers.
6. **Backend authority** — the backend is the sole authority for workflow
   state, transition validation, authorization, and audit. The frontend
   renders but does not decide.
7. **Defense in depth** — tenant isolation, authorization, and input
   validation are enforced at multiple independent layers.

---

## System Components

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP API Layer                       │
│         (OpenAPI-defined, versioned, authenticated)      │
├─────────────────────────────────────────────────────────┤
│                    Application Services                 │
│                                                         │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │Ident.  │ │Org/Tenant │ │ Cases  │ │Workflow│        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │ Rules  │ │ Forms     │ │Eviden. │ │Audit   │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │Assess. │ │Decisions  │ │Assistan│ │People  │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │Followup│ │FormSubmit │ │CaseCtx │ │CaseSum │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │DocIntel│ │ AI Obs    │ │RulesAI │ │Context │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │Operat. │ │ReviewQueue│ │Privacy │ │Shared  │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
├─────────────────────────────────────────────────────────┤
│                    Shared Infrastructure                │
│   Config │ Logging │ Metrics    │ Session │ Auth       │
├─────────────────────────────────────────────────────────┤
│                    Data & Storage Layer                 │
│  Primary DB (PostgreSQL) │ Object Storage (S3)   │
└─────────────────────────────────────────────────────────┘
```

### Module boundaries

Each module has a single responsibility and owns its data. Modules
communicate through:

- **Internal service interfaces** (in-process function calls with
  defined contracts).
- **Shared database** for persistence, with per-module schema ownership.

| Module | Owns | Communicates with |
|--------|------|-------------------|
| **Identity** | Users, credentials, sessions, tokens, authz policies | Organizations (for tenant-scoped auth) |
| **Organizations** | Tenants, settings, organization-level config | Identity (auth context) |
| **Cases** | Cases, case status, assignments | Workflow (state machine) |
| **Workflow** | Workflow definitions, instances, state machines, transitions, history | Cases, Audit |
| **Rules** | Rule sets, rules, rule assignments, evaluations against facts | Cases, FormSubmission |
| **Forms** | Form definitions (field schemas, field-type vocab) | Organizations |
| **Form Submission** | Completed form submissions scoped to organizations | Cases, Rules (fact source) |
| **Evidence** | Evidence items, document references | Cases, Storage |
| **AI Observations** | Observation entities, human review, provider abstraction | Evidence, Audit |
| **Case Context** | Aggregated case context from multiple domains | Cases, Evidence, Rules, Workflow, Decisions, AI Observations |
| **Case Summary** | AI-generated case summaries with provider abstraction | Case Context, Cases, AI Provider |
| **Document Intelligence** | AI-generated document analyses with provider abstraction | Evidence, AI Provider |
| **Assessments** | Needs assessments, recommendations | Cases |
| **Decisions** | Human decisions, rationale, versioning, supersession | Cases, Workflow, Rules, Evidence, ReviewQueue |
| **Assistance** | Assistance actions, service delivery | Cases |
| **Follow-ups** | Follow-up scheduling, completion | Cases |
| **People** | Person records, contact details | Cases |
| **Review Queue** | Human review work queue, assignment, state machine | Decisions, Cases |
| **Audit** | Tamper-evident hash-chained audit log, event records | All modules (writes); external consumers (reads) |
| **Operations** | Metrics, dashboards, exports, AI intelligence | Cases, Workflow, Decisions, Evidence, Assistance |
| **Privacy** | Data classification, export/deletion workflows (post-1.0) | All modules |

### Communication rules

1. Modules do not access another module's database tables directly.
   They go through the owning module's service interface.
2. Module-to-module calls use the internal service interface.
3. The Audit module is write-only from the perspective of other
   modules: all modules record audit entries via the Audit module's
   interface, but only the Audit module writes to and reads from the
   audit store.
4. Direct cross-module calls are allowed for synchronous operations
   within a transaction boundary.

---

## Request / Authentication Flow

```
Client Request
  → Request ID middleware (generate or validate X-Request-ID)
  → Trusted proxy IP detection (RealIP, X-Forwarded-For)
  → Logging middleware (structured JSON log after response)
  → Rate limit middleware
  → Recovery middleware (panic catch)
  → AuthRequired middleware (if route requires auth)
      → VerifyAccessToken (JWT signature, issuer, expiry)
      → FindByIDForOrganization(jti, orgID)  [session lookup]
      → Validate !revoked && !expired && user/org match
      → MarkUsedForOrganization(jti, orgID)
  → RequireSameTenant middleware (orgId path param == JWT org claim)
  → RequireAnyRole middleware (if route requires specific role)
  → Handler
  → Response
```

### Pre-authentication vs. Authenticated Context

The session repository uses a two-level API to handle both contexts:

- **Pre-authentication:** `FindByRefreshTokenHash(hash)` — base lookup without
  `organization_id`. Used in the refresh-token flow where the organization is
  not yet established from the request context.
- **Authenticated context:** `FindByIDForOrganization(jti, orgID)`,
  `RevokeForOrganization(...)`, `MarkUsedForOrganization(...)`, and related
  org-aware methods. Used after the JWT is verified and the organization is
  known. These provide defense-in-depth by enforcing tenant boundaries at
  the data layer.

This design exists because refresh-token validation legitimately occurs before
tenant context is established. Once authenticated, every session operation is
organization-scoped.

---

## Tenant Boundary

All CIVORA data is scoped by `organization_id`. The tenant boundary is enforced
at three independent layers:

1. **API middleware:** `RequireSameTenant` ensures the `orgId` path parameter
   matches the `organization_id` claim in the authenticated JWT.
2. **Service layer:** All service methods accept and validate `organizationID`.
3. **Repository layer:** All data-access methods filter queries by
   `organization_id` and accept `organizationID` as a parameter.

Cross-tenant access is rejected with 403 or 404 depending on the path. No
module bypasses the tenant boundary.

---

## Session Architecture

### Session Table

The `auth_sessions` table (migration 0044) stores:

| Column | Purpose |
|--------|---------|
| `id` (UUID) | JWT `jti` claim — binds the access token to a session |
| `user_id` (UUID) | Owning user |
| `organization_id` (UUID) | Tenant — enforced in org-aware lookups |
| `refresh_token_hash` (BYTEA) | SHA-256 hash of the current refresh token |
| `token_family` (UUID) | Rotated on each refresh; enables family-level revocation |
| `expires_at` (TIMESTAMPTZ) | Session expiry |
| `last_used_at` (TIMESTAMPTZ) | Updated on each authenticated request |
| `created_at` / `updated_at` | Lifecycle timestamps |

### Two-Level Repository API

The `SessionRepository` interface declares paired methods:

**Base (no org filter):**
- `FindByRefreshTokenHash(ctx, hash)` — used in refresh flow (pre-auth)
- `FindByID(ctx, id)` — internal use only
- `Revoke(ctx, id)` — internal use only
- `RevokeAllByUserID(ctx, userID)` — internal use only
- `RevokeAllByUserIDTx(ctx, tx, userID)` — internal use only
- `MarkUsed(ctx, id)` — internal use only

**Org-aware (with `organizationID`):**
- `FindByIDForOrganization(ctx, id, organizationID)` — auth middleware
- `FindActiveByUserIDForOrganization(ctx, userID, organizationID)` — session enumeration
- `RevokeForOrganization(ctx, id, organizationID)` — logout / password change
- `RevokeAllByUserIDForOrganization(ctx, userID, organizationID)` — role/password change
- `RevokeAllByUserIDTxForOrganization(ctx, tx, userID, organizationID)` — transactional revocation
- `MarkUsedForOrganization(ctx, id, organizationID)` — request-time update

### Why Not Every Call Requires organization_id

`FindByRefreshTokenHash` is the critical exception. During the refresh-token
flow, only the hashed token is available — the organization is not yet
established from the request context because no JWT has been verified yet.
The service layer re-fetches the user and validates org membership after the
base lookup.

Once a request is authenticated, the organization is known from the JWT, and
all subsequent session operations use org-aware methods.

---

## Workflow Architecture

### Core Concepts

- **Workflow Definition** — a versioned, tenant-scoped template describing a
  service-delivery process. Contains states, transitions, and metadata.
  Statuses: `DRAFT`, `ACTIVE`, `ARCHIVED`.
- **Workflow State** — a named stage in a workflow definition. Has a key,
  display order, terminal flag, and optional responsible role.
- **Workflow Transition** — an explicit, validated move between two states.
  Has a key, optional conditions, optional allowed roles, and an active flag.
- **Workflow Instance** — the execution of a Workflow Definition for a specific
  Case. Retains the definition/version it started with.
- **Workflow Transition History** — immutable record of every transition
  executed on a workflow instance.

### Key Properties

1. **Workflow Instance is the source of truth for case workflow state.**
   The Case table stores a `workflow_instance_id` foreign key. All state
   changes go through the workflow engine.

2. **Definitions are versioned.** Existing cases retain the definition/version
   they started with. New cases use the latest active version.

3. **Transition execution is centralized.** All state changes go through
   `WorkflowService.ExecuteTransition`. No module bypasses the engine.

4. **Tenant isolation is enforced.** Definitions and instances are scoped by
   `organization_id`. Cross-tenant access is prevented at the repository and
   service layers.

5. **Authorization is data-driven.** Transitions can declare `allowed_roles`.
   The workflow service checks the actor's role against this list server-side.

6. **Conditions are extension points only.** The current implementation
   supports the data model for conditions but does not execute arbitrary code.
   No eval-like functionality is permitted.

7. **Audit integration is native.** Every transition creates an audit event
   using the existing audit vocabulary (`workflow.transitioned`).

8. **Concurrency is safe.** Optimistic concurrency control (version column)
   prevents conflicting simultaneous transitions.

### Terminal States

States marked as terminal cannot have outgoing transitions. The engine enforces
this at the definition validation level (`ValidateWorkflowDefinition`) and at
execution time. Attempting to transition from a terminal state returns an error.

### API

The workflow engine exposes REST endpoints for managing definitions and
executing transitions. See `api/openapi/openapi.yaml` for the full specification.

---

## Dynamic Form Architecture

### Core Concepts

- **Form Definition** — a tenant-scoped schema for collecting structured data.
  Contains field definitions with types, validation constraints, and display
  metadata.
- **Form Version** — an immutable snapshot of a form definition. Form
  definitions follow a DRAFT → PUBLISHED → ARCHIVED lifecycle. Published
  versions cannot be modified.
- **Form Submission** — a completed form for a specific Case, referencing a
  specific Form Version. Submissions become structured facts for the rules
  engine.

### Field Types

13 field types are supported: text, textarea, number, decimal, date, datetime,
boolean, select, multiselect, radio, checkbox, email, phone.

### Validation

- **Required fields:** Enforced server-side.
- **Min/max constraints:** Enforced on numeric and string fields.
- **Regex patterns:** Enforced on text fields.
- **Type checking:** Enforced server-side for all field types.

### State Assignment

Forms can be assigned to specific workflow states. Required forms must be
completed before the case can transition out of that state.

### Concurrency Protection

Form field keys are unique per version. Concurrent duplicate key addition is
prevented by database unique constraints (≤1 success in concurrent attempts).

---

## Rules Engine Architecture

### Core Concepts

- **Rule Set** — a versioned, tenant-scoped collection of rules. Follows a
  DRAFT → PUBLISHED → ARCHIVED lifecycle. Published versions are immutable.
- **Rule** — a single condition tree with an outcome. Conditions are logical
  expressions (AND/OR/NOT) comparing facts using typed operators.
- **Evaluation** — a published Rule Set evaluated against a frozen snapshot of
  case facts (form submissions, case attributes, person data).

### Key Properties

1. **Pure function:** `Evaluate(ruleSet, facts, evaluatedAt, evaluatedBy, trigger)`
   performs no I/O. Given identical inputs, it always produces identical
   outputs.
2. **Deterministic trace:** Evaluation uses a counter, not random state.
   Every evaluation produces a full trace tree.
3. **Type-safe operators:** Operators are validated against fact types before
   evaluation.
4. **Short-circuit evaluation:** Rules are evaluated in priority order (0 =
   first). First match wins.
5. **Advisory only:** Outcomes are ELIGIBLE, INELIGIBLE, REQUIRES_REVIEW,
   INFORMATION_REQUIRED, FLAG, SCORE, or ERROR. Rules never make final
   decisions.

### Integration

Rule Sets can declare `triggers` (e.g., `form_submitted`). When a form is
submitted, the Workflow Engine's observer calls the Rules Engine to evaluate
the case. Evaluation outcomes are advisory — they inform human reviewers and
can gate workflow transitions, but never bypass them.

---

## Evidence Subsystem

### Core Concepts

- **Document** — a file with metadata, stored in object storage with a
  server-generated storage key. The `StorageKey` is tagged `json:"-"` and
  never appears in API responses.
- **Evidence** — a Document with provenance metadata (verification status,
  verifier, verification reason, method), linked to a Case.

### Verification Lifecycle

Evidence items move through a state machine:

```
UNVERIFIED → VERIFIED
UNVERIFIED → REJECTED
UNVERIFIED → NEEDS_REVIEW
NEEDS_REVIEW → VERIFIED
NEEDS_REVIEW → REJECTED
VERIFIED → REJECTED
VERIFIED → NEEDS_REVIEW
REJECTED → VERIFIED
REJECTED → NEEDS_REVIEW
```

Every transition appends an immutable record to `evidence_verification_history`.
The transition is validated server-side against the current state.

### Metadata Validation

- Maximum metadata size: 100,000 bytes (JSON-serialized).
- Maximum description length: 5,000 characters.
- Storage references must be valid URIs with schemes `s3`, `gs`, `azureblob`,
  or `civora`. Local paths, credentials in URIs, query parameters, fragments,
  and path-traversal sequences (`..`) are rejected.
- Document upload size limit: 10 MB.
- Content type allow-list enforced at upload.

### Storage-Reference Protection

The `storage_reference` field is accepted on input but excluded from API
responses. The `serializeEvidence` function does not include it. Document
`StorageKey` is tagged `json:"-"`. This prevents internal storage references
from leaking to API clients.

### Concurrent Verification Protection

Verification updates use row-level locking (`SELECT ... FOR UPDATE`) inside a
database transaction. This prevents concurrent verification races where two
actors might attempt to verify or reject the same evidence simultaneously.

---

## Human Decisions

### Core Concepts

- **Decision** — a recorded human decision with rationale, timestamp, actor,
  version, and provenance links to workflow state, rule evaluations, evidence,
  form submissions, and review queue entries.
- **Supersession** — when a decision is superseded, a new Decision is created
  with `Version = supersededVersion + 1` and the old decision's `SupersededByID`
  is set. The unique index on `(organization_id, service_request_id, version)`
  ensures integrity.

### Versioning

Each decision has a `Version` integer. `NewSupersedingDecision` increments from
the superseded version. `FindByServiceRequest` returns the latest version. The
full history is preserved and queryable.

### Integration with Other Subsystems

- **Workflow:** Decisions trigger workflow transitions atomically — decision +
  transition + audit in a single database transaction.
- **Rules:** Rule evaluation outcomes inform decisions but never bypass them.
- **Evidence:** Decisions reference the evidence items that supported them.
- **Review queue:** Review queue entries track the assignment, review, and
  decision workflow for human reviewers.

---

## Audit Subsystem

### Core Concepts

- **Audit Event** — an immutable record of an action, stored in the dedicated
  `audit` schema, isolated from the `public` schema.

### Hash-Chain Integrity

Each event carries a SHA-256 hash linked to the previous event for the same
organization. The chain is computed over: organization ID, actor, action,
resource type, resource ID, outcome, request ID, metadata, timestamp, and
previous hash.

`VerifyChain` validates the entire chain for an organization. Background
maintenance verifies integrity periodically.

### Transactional Recording

Audit writes occur in the same database transaction as the state change they
record (`RecordEventInTx`). If the audit write fails, the state change is
rolled back. This ensures audit completeness.

### Limitation

The hash chain is tamper-evident within the application/database boundary. It
is not an externally anchored immutable ledger. A compromised application
process or database administrator can write a valid chain with false contents.
External append-only anchoring is a post-1.0 enhancement.

---

## Persistence / Repositories

### Database

CIVORA uses PostgreSQL 16 as the primary database. The database contains:

- `public` schema — business data (cases, workflow definitions, forms,
  evidence, decisions, etc.)
- `audit` schema — tamper-evident audit events, isolated from business data
- `auth_sessions` table — session state for JWT validation

### Object Storage

Documents are stored in an S3-compatible object storage backend. The storage
backend is abstracted behind an interface (`StorageProvider`) with a local
filesystem implementation available for development. Production deployments
should use S3 or compatible storage.

### Migration Architecture

Database migrations are managed as ordered SQL files in `migrations/`. Each
migration has an `up` and `down` component. Migrations are applied in order
and are reversible. The current set contains 44 migrations (0001–0044).

Key migrations include:
- 0001–0019: Initial schema (organizations, users, cases, workflow tables,
  forms, rules, evidence, decisions, assistance, follow-ups, people,
  assessments)
- 0020–0035: Feature additions (audit schema, AI observations, case context,
  case summary, document intelligence, decision provenance links)
- 0036–0044: Hardening (workflow form assignments, review queue, auth sessions)

---

## Transactions / Concurrency

### Optimistic Concurrency

Workflow transitions use optimistic concurrency control via a `version` column.
`ExecuteTransition` reads the current version, applies the transition, and
writes back with `WHERE version = $oldVersion`. If the row was modified between
read and write, the update affects zero rows and `ErrConcurrentModification` is
returned.

### Row-Level Locking

Evidence verification updates use `SELECT ... FOR UPDATE` inside a transaction
to lock the evidence row during the verification state change. This prevents
concurrent verification races.

### Idempotency

The idempotency middleware ensures that duplicate requests with the same
`Idempotency-Key` header produce the same result without creating duplicate
resources.

### Concurrent Duplicate Prevention

- **Assistance creation:** Concurrent duplicate assistance creation succeeds
  for all callers (idempotent behavior).
- **Decision creation:** Concurrent duplicate decisions return exactly one
  success (unique constraint on `(organization_id, service_request_id, version)`).
- **Form field keys:** Concurrent duplicate field-key addition is prevented
  by unique constraints (≤1 success).

---

## Observability

### Request ID Propagation

Every request is assigned a `X-Request-ID` UUID. If the client supplies one,
it is validated and preserved; otherwise a new UUID is generated. The ID is
propagated through the request context and included in all log entries and
audit events.

### Structured Logging

Logs are emitted as JSON to stdout/stderr with fields: `time` (RFC3339Nano UTC),
`level` (derived from HTTP status: error ≥500, warn ≥400, else info), `method`,
`path`, `status`, `bytes_written`, `latency_microseconds`, `request_id`,
`remote_ip`, `user_agent`. Request and response bodies are not logged.

### Health and Readiness

- `GET /health` — liveness probe. Returns `{"status":"ok"}`. No authentication.
- `GET /ready` — readiness probe. Pings the database. Returns 200 if reachable,
  503 if not. No authentication.

### Application Metrics

Authenticated metrics endpoints under `/api/v1/organizations/{orgId}/operations/metrics/*`
provide case volume, pending reviews, workflow throughput, state duration, case
cycle time, aging cases, decision outcomes, assistance outcomes, evidence
verification status, and dashboard aggregations.

A Prometheus `/metrics` endpoint is not currently implemented.

### Sensitive Data Restrictions

- Request/response bodies are never logged.
- Export endpoints apply field-name-based sanitization, replacing values of
  fields matching sensitive patterns with `"[REDACTED]"`.
- AI document processing applies PII redaction before forwarding to external
  LLM providers.

---

## Frontend / Backend Boundary

The frontend is a lightweight static HTML/CSS/JS SPA served from `web/`. It
consumes the OpenAPI-defined REST API. The backend is the sole authority for:

- Workflow state and transition validation
- Authorization decisions
- Input validation
- Audit recording

The frontend renders available transitions based on API responses but cannot
override backend decisions. For custom workflows, the frontend uses a heuristic
mapping for transition labels; the backend still enforces the actual allowed
roles and state machine.

Playwright end-to-end tests under `web/e2e/` serve `web/` as a static site and
mock the API via request interception, running fully offline.

---

## AI Module Architecture

The AI module is a first-class, platform-level capability that provides
intelligence features across all CIVORA processes. It is designed as an
isolated, configurable, multi-tenant module with strict boundaries against
consequential system functions.

### AI Decision Boundary

There is **no code path** from AI outputs to:

- Decisions
- Workflow transitions
- Rule evaluation results
- Form submission data
- Evidence modification
- Case status changes

AI outputs require explicit human review (accept/reject/correct/dismiss)
before any downstream system may use them. This is enforced by the absence of
code paths, not by policy alone.

### AI Provider Abstraction

```
AIProvider interface
├── GenerateObservations(ctx, req) ([]ObservationResult, error)
├── GenerateDocumentAnalyses(ctx, req) ([]AnalysisResult, error)
├── GenerateCaseSummary(ctx, req) (*CaseSummaryResult, error)
└── ProviderInfo() ModelInfo

Implementations:
├── OpenAIProvider — external API calls to OpenAI
├── LocalProvider — local Ollama-compatible endpoint
└── NoopProvider — disabled mode, returns ErrProviderDisabled
```

### AI Data Flow

```
Document → Evidence → DocumentContentProvider → PII Sanitizer
    → AI Provider → PromptInjectionGuard → Observation
    → Human Review → Verified Fact → Audit Event
```

---

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.23+, compiled to a single static binary |
| Database | PostgreSQL 16 |
| API | HTTP REST with JSON, OpenAPI 3.0.3 as source of truth |
| Authentication | JWT access tokens + opaque refresh tokens |
| Frontend | Lightweight static HTML/CSS/JS SPA served from `web/` |
| Object Storage | S3-compatible (local filesystem for development) |
| Deployment | Docker container image; Docker Compose for local development |
| License | Apache 2.0 |

---

## Deployment

### Minimum Requirements

- CPU: 1 core minimum (2+ recommended)
- RAM: 512 MB minimum (1 GB+ recommended)
- Disk: 1 GB minimum
- Network: HTTP/HTTPS access

### Supported Configurations

- **Development:** PostgreSQL + local file storage.
- **Production:** PostgreSQL + S3-compatible object storage.
- **Container:** Docker image, runnable with `docker run`.

### Transport Security

CIVORA supports two deployment models:

1. **Reverse-proxy TLS termination (recommended for production):** A reverse
   proxy terminates TLS and forwards plain HTTP to CIVORA. Configure
   `CIVORA_SERVER_TRUSTED_PROXIES` and `CIVORA_SERVER_FORCE_HTTPS=true`.
2. **Direct TLS termination:** Provide certificate and key files via
   `CIVORA_SERVER_TLS_CERT` and `CIVORA_SERVER_TLS_KEY`.

Exposing CIVORA on plain HTTP to the Internet is a security vulnerability.

---

## Security

- Authentication on every request (except `/health` and `/ready`).
- Authorization checked at the service and data layers.
- All input validated and sanitized.
- No execution of code from user-supplied data.
- Secrets, tokens, and passwords are never logged.
- Rate limiting on all endpoints.
- JWT access tokens are short-lived (default 20 minutes) and validated against
  server-side session state on every request.
- Refresh tokens are hashed in PostgreSQL, rotated on each use with a new token
  family, and tracked in `auth_sessions`.
- Password changes and role changes invalidate all active sessions.
- Cross-tenant access is rejected at middleware, service, and repository layers.

See [SECURITY.md](SECURITY.md) for the full security reference.

---

## Roadmap

Phase 1 (CIVORA 1.0.0) establishes:

- The modular monolith architecture.
- Configurable workflow engine with optimistic concurrency.
- Dynamic forms with 13 field types and versioning.
- Deterministic rules engine with trace generation.
- Evidence management with verification lifecycle.
- Human decisions with versioning and supersession.
- Tamper-evident hash-chained audit trail.
- Multi-tenancy with defense-in-depth isolation.
- Session management with two-level repository, rotation, and reuse detection.
- Application-level observability (request IDs, structured logs, health/readiness).
- Comprehensive security test suite.
- Playwright frontend E2E suite.
- OpenAPI 3.0.3 specification.
