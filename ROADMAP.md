# CIVORA Roadmap

> **Status**: Draft — milestone definitions for the foundation phase
> and beyond. **No dates are promised.** Milestones are ordered by
> dependencies and priority, not by schedule.

---

## Milestones

### 0.1 Foundation

**Goal**: Establish the project foundation: build system, CI, testing
framework, initial API specification, and the first vertical slice
("Emergency Assistance Request") — minimal end-to-end.

**Scope**:
- Project structure, build configuration, and dependency management.
- CI pipeline (lint, test, security scan).
- Basic Identity module (authentication, authorization, users).
- Basic Organizations module (tenant scoping).
- Basic Audit module (immutable event log).
- Minimal API layer with authentication.
- Initial OpenAPI specification.
- First vertical slice: Emergency Assistance Request (request → case →
  decision → assistance).

**Out of scope**: AI, forms, documents, integrations, advanced workflow
features.

### 0.2 Service Delivery Lifecycle

**Goal**: Implement the core public-interest service delivery lifecycle:
eligibility assessment, assistance provisioning, and follow-up management.

**Status**: Complete.

**Implemented**:
- Eligibility module: eligibility checks with JSONB criteria storage
- Assistance module: assistance provisioning linked to cases
- Follow-up module: follow-up scheduling and tracking
- UPSERT repositories for all three modules
- Extended OpenAPI specification with new schemas and endpoints
- Configurable rate limiter for E2E test compatibility
- Workflow Definition model (state machines, transitions) — configurable
  via API, tenant-scoped, with draft→active→archived lifecycle
- Workflow Instance execution — creates instance on case creation,
  executes transitions, records history, syncs case status
- Case binding to workflow via `workflow_id` (explicit selection)
- Generic terminal-state handling — any state can be terminal;
  `closed_at` set for any terminal state, not just "CLOSED"/"REJECTED"

**Dependencies**: 0.1

### 0.3 Case Management

**Goal**: Full case lifecycle management.

**Scope**:
- Case creation, status tracking, assignment.
- Case linking (parent/child cases).
- Case templates.
- Case search and filtering.

**Dependencies**: 0.2

### 0.4 Forms

**Goal**: Structured data collection via forms.

**Scope**:
- Form Definition model (various field types).
- Form rendering and validation.
- Form submission storage.
- Form-to-case binding.

**Dependencies**: 0.3

### 0.5 Evidence / Documents

**Goal**: Document and evidence management with audit chains.

**Scope**:
- Document upload, storage, and metadata.
- Evidence chains (linking documents to cases/instances).
- Content-type validation and anti-malware scanning.
- Document versioning and retention.

**Dependencies**: 0.4

### 0.6 Human Review & Decision

**Goal**: Implement human-in-the-loop decision-making with full provenance, review queue, and workflow integration.

**Status**: Complete.

**Implemented**:
- Decision domain: immutable decision records with versioning, supersession, and full provenance (case, workflow state, rule evaluations, evidence, form submissions)
- Decision types: APPROVED, REJECTED, NEEDS_MORE_INFORMATION, ESCALATE
- Review queue with state machine: PENDING → ASSIGNED → IN_REVIEW → COMPLETED/ESCALATED/WAITING_INFORMATION
- Secure review assignment with PostgreSQL SKIP LOCKED for concurrency safety
- Human decision actions: APPROVE, REJECT, ESCALATE, REQUEST_INFORMATION with reason requirements
- Decision-to-workflow-transition mapping via `decision_type` column on workflow transitions
- Atomic decision + workflow transition + audit in single database transaction
- Workflow transition observer integration for automatic review queue creation
- Comprehensive security test suite (30 attack vectors: cross-tenant, IDOR, impersonation, replay, concurrency, tampering, audit manipulation)
- Platform configurability proof: Emergency Assistance + Education Assistance workflows with different states, forms, rules, and decision flows
- Historical reproducibility: decisions reference exact rule/form/evidence versions; supersession preserves audit chain
- Reviewer workspace frontend with case context, rule evaluations, evidence, forms, previous decisions

**Dependencies**: 0.5

### 0.7 Evidence & Documents

**Goal**: Document and evidence management with audit chains.

**Status**: Complete.

**Implemented**:
- Evidence domain: create, read, list by case, update metadata
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
- Security: tenant isolation, IDOR prevention, upload/download security,
  path traversal prevention

**Dependencies**: 0.6

### 0.8 AI Assistance

**Goal**: Introduce AI assistance with human-in-the-loop safeguards.

**Scope**:
- AI adapter framework (model-provider-neutral).
- Prompt management and version tracking.
- Confidence/uncertainty reporting.
- Human review gates for consequential decisions.
- AI audit logging (provenance, model identification).
- Opt-out / disable AI at org or system level.

**Dependencies**: 0.7

### 0.9 Production Hardening

**Goal**: Production-readiness, scalability, and performance.

**Scope**:
- Performance testing and optimization.
- Security hardening and penetration testing.
- Backup and disaster recovery procedures.
- Monitoring, alerting, and observability.
- Horizontal scaling (if monolith-to-service extraction is needed).
- Operator tooling (upgrade, migration, rollback).

**Dependencies**: 0.8

### 1.0 Community Release

**Goal**: First stable, community-maintained release suitable for
production deployment by public-interest institutions.

**Scope**:
- API stability guarantees.
- Documentation completeness.
- Security audit by external reviewer (if feasible).
- Packaging and distribution (container images, binary releases).
- Community readiness (contributor guides, mentorship).
- Case studies from early adopter institutions (if any).

**Dependencies**: 0.9

---

## Process for milestone changes

- Milestones and their scope are reviewed at each steward meeting.
- Scope can be adjusted, but removal of completed work or addition of new
  work requires consensus among active contributors.
- Dependencies between milestones are maintained unless there is a
  strong technical justification to change them.

---

Last updated: 2026-09-16
