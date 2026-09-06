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

### 0.2 Workflow Core

**Goal**: Implement the core workflow engine and workflow definition
model.

**Scope**:
- Workflow Definition model (state machines, transitions).
- Workflow Instance execution.
- Basic rule evaluation.
- Webhook/event notifications for workflow events.

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

### 0.6 Audit & Governance

**Goal**: Mature auditing, governance controls, and compliance features.

**Scope**:
- Tamper-evident audit log (signed/hashed).
- Granular access logs.
- Policy management and enforcement.
- Data retention and deletion automation.
- Audit export and reporting.

**Dependencies**: 0.5

### 0.7 AI Assistance

**Goal**: Introduce AI assistance with human-in-the-loop safeguards.

**Scope**:
- AI adapter framework (model-provider-neutral).
- Prompt management and version tracking.
- Confidence/uncertainty reporting.
- Human review gates for consequential decisions.
- AI audit logging (provenance, model identification).
- Opt-out / disable AI at org or system level.

**Dependencies**: 0.6

### 0.8 Interoperability

**Goal**: Standards-based interoperability and integration.

**Scope**:
- Open standards for data exchange (JSON-LD, standard formats).
- Integration adapter framework.
- External system connectors (IdP, payment, document verification).
- Import/export tooling for data portability.
- API federation / cross-instance references.

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

Last updated: 2026-09-06
