# CIVORA Architecture

## Overview

CIVORA uses a **modular monolith** architecture. See
[ADR-001: Initial Architecture](docs/decisions/0001-initial-architecture.md)
for the decision record and rationale.

This document provides a high-level overview of the system's structure,
module boundaries, data flow, and deployment model.

---

## Architecture diagram

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP API Layer                       │
│         (OpenAPI-defined, versioned, authenticated)      │
├─────────────────────────────────────────────────────────┤
│                    Application Services                 │
│                                                         │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │ Ident. │ │ Org/Tenant│ │ Cases  │ │Workflow│        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐ ┌────────┐        │
│  │ Forms  │ │Documents  │ │ Tasks  │ │Notify. │        │
│  └────────┘ └───────────┘ └────────┘ └────────┘        │
│  ┌────────┐ ┌───────────┐ ┌────────┐                   │
│  │ Audit  │ │  Policy   │ │  AI    │                   │
│  └────────┘ └───────────┘ └────────┘                   │
│  ┌────────┐                                             │
│  │Integrat│                                             │
│  └────────┘                                             │
├─────────────────────────────────────────────────────────┤
│                    Shared Infrastructure                │
│   Config │ Logging │ Metrics    │
├─────────────────────────────────────────────────────────┤
│                    Data & Storage Layer                 │
│  Primary DB (PostgreSQL) │ Object Storage (S3)   │
└─────────────────────────────────────────────────────────┘
```

---

## Module boundaries

Each module has a single responsibility and owns its data. Modules
communicate through:

- **Internal service interfaces** (in-process function calls with
  defined contracts).
- **Shared database** for persistence, with per-module schema ownership.

| Module | Owns | Communicates with |
|--------|------|-------------------|
| **Identity** | Users, credentials, sessions, tokens, authz policies | Organizations (for tenant-scoped auth) |
| **Organizations** | Tenants, settings, organization-level config | Identity (auth context) |
| **Cases** | Cases, case status, case assignments | Workflow (state machine) |
| **Workflow** | Workflow definitions, instances, state machines, transition history | Cases, Audit |
| **Audit** | Immutable audit log, event records | All modules (writes); external consumers (reads) |
| **Eligibility** | Eligibility assessments | Cases |
| **Evidence** | Evidence items, document references | Cases |
| **Assessments** | Needs assessments, recommendations | Cases |
| **Decisions** | Human decisions, rationale | Cases, Workflow |
| **Assistance** | Assistance actions, service delivery | Cases |
| **Follow-ups** | Follow-up scheduling, completion | Cases |
| **People** | Person records, contact details | Cases |

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

## Domain model

The conceptual domain model distinguishes between **entities**,
**value objects**, **aggregates**, **events**, **projections**, and
**configuration**. Not all concepts map to database tables.

### Entities (aggregate roots)

- **Organization** — the top-level tenant. All other entities belong
  to exactly one organization.
- **User** — an individual with an account. Belongs to one or more
  organizations.
- **Case** — a container for a service-delivery process. Belongs to
  one organization.
- **Workflow Definition** — the template/blueprint for a workflow.
   Belongs to one organization. Versioned. Contains states, transitions,
   and metadata.
- **Workflow State** — a named stage in a workflow definition. Has a key,
   display order, terminal flag, and optional responsible role.
- **Workflow Transition** — an explicit, validated move between two states.
   Has a key, optional conditions, and optional allowed roles.
- **Workflow Instance** — a running execution of a Workflow Definition.
   Associated with a Case. Retains the definition/version it started with.
- **Workflow Transition History** — an immutable record of every transition
   executed on a workflow instance.
- **Task** — a unit of work within a Workflow Instance. Has assignments,
  deadlines, and status.
- **Form Definition** — a schema for collecting structured data.
- **Form Submission** — a completed form for a specific Task.
- **Document** — a file with metadata, stored in object storage with
  a reference in the database.
- **Policy** — a rule or policy definition used for evaluation.
- **Integration** — a configured external service connection.

### Value objects

- **Role** — a named set of permissions within an organization.
- **Evidence** — a reference to a Document with provenance metadata.
- **Decision** — a recorded decision with rationale, timestamp, and
  actor.
- **Comment** — a time-stamped annotation on a Case, Task, or Document.
- **Audit Event** — an immutable record of an action.

### Audit event types

CIVORA records audit events for important state changes. These are recorded
in the audit log via synchronous calls from each module (no event bus).
Audit event types include:

- `case.created`, `case.status_changed`, `case.closed`, `case.assigned`
- `workflow.definition.created`, `workflow.definition.activated`, `workflow.definition.archived`
- `workflow.instance.created`, `workflow.transitioned`, `workflow.completed`
- `user.created`, `auth.success`, `auth.failed`
- `organization.created`
- (Future) additional event types as features are implemented

Note: An event bus is not currently implemented. Cross-module communication
uses direct synchronous function calls. Future milestones may introduce
an event bus if asynchronous patterns become necessary.

### Projections / read models

Read-side views optimized for queries, derived from events and/or
direct writes. Examples:

- Case summary view (for listing/searching cases)
- Task inbox view (per-user task list)
- Audit log view (filtered, paginated audit entries)
- Analytics/dashboard views (future)

### Configuration

- System configuration (environment variables)
- Organization settings (features enabled, retention policies, etc.)
- Workflow definitions (process blueprints)
- Policy definitions (rules for evaluation)
- Form definitions (data-collection schemas)
- Integration configurations (API keys, endpoints)

### External integrations

- Identity providers (future: OIDC, SAML, LDAP)
- Document verification services
- Payment systems
- Email/SMS providers
- Storage backends
- AI model providers (future)

---

## Workflow Engine

CIVORA includes a first-class **configurable workflow engine** that allows
organizations to define service-delivery workflows as data rather than code.

### Core concepts

- **Workflow Definition** — a versioned, tenant-scoped template describing
  a service-delivery process. Contains states, transitions, and metadata.
  Statuses: `DRAFT`, `ACTIVE`, `ARCHIVED`.
- **Workflow State** — a named stage in a workflow definition. Has a key,
  display order, terminal flag, and optional responsible role.
- **Workflow Transition** — an explicit, validated move between two states.
  Has a key, optional conditions, optional allowed roles, and an active flag.
- **Workflow Instance** — the execution of a Workflow Definition for a specific
  Case. Retains the definition/version it started with.
- **Workflow Transition History** — immutable record of every transition
  executed on a workflow instance.

### Key properties

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
   The workflow service checks the actor's role against this list.

6. **Conditions are extension points only.** The current implementation
   supports the data model for conditions but does not execute arbitrary code.
   No eval-like functionality is permitted.

7. **Audit integration is native.** Every transition creates an audit event
   using the existing audit vocabulary (`workflow.transitioned`).

8. **Concurrency is safe.** Optimistic concurrency control (version column)
   prevents conflicting simultaneous transitions.

### Emergency Assistance as the first workflow

The Emergency Assistance workflow is now a seeded Workflow Definition:

```
NEW → OPEN → IN_REVIEW → ASSESSMENT → DECISION_PENDING
                                             ├── APPROVED → IN_PROGRESS → FOLLOW_UP → CLOSED
                                             └── REJECTED → CLOSED
```

This means:

- The Emergency Assistance flow is **configuration, not code**.
- New workflows (e.g., Education Assistance, Health Services) can be added
  by creating new Workflow Definitions without changing core code.
- The frontend renders transitions dynamically from the API.
- Audit and history are consistent across all workflows.

### API

The workflow engine exposes REST endpoints for managing definitions and
executing transitions. See `api/openapi/openapi.yaml` for the full specification.

---

## API

- **Protocol**: HTTPS (HTTP/1.1 or HTTP/2)
- **Format**: JSON request/response bodies
- **Authentication**: Bearer tokens (JWT or opaque session tokens)
- **Versioning**: URL path prefix (`/api/v1/`)
- **Specification**: OpenAPI 3.0, documented in
  `docs/architecture/api-spec.md`
- **Rate limiting**: Enforced per-user and per-IP (configurable)

The API exposes endpoints for all module functionality. The first
vertical slice (Emergency Assistance Request) will define the minimum
API surface needed for that workflow.

---

## Data

### Data ownership

Data belongs to the organization that created it. CIVORA does not
claim ownership of any data stored in the system.

### Tenant isolation

- All data is scoped by `organization_id`.
- All queries are filtered by organization at the data-access layer.
- Database foreign keys enforce organizational ownership.

### Personal data

(Future: data classification labels, encrypted storage, and export/
deletion workflows.)

### Retention

- Data retention is configurable per organization and data type.
- Default retention: 7 years for audit records (changeable by operator).

### Encryption

- TLS 1.2+ in transit (required in production).

(Future: encryption at rest and key management via KMS.)

---

## Security

See [docs/threat-model.md](docs/threat-model.md) for the full threat
model. Key security properties:

- Authentication on every request.
- Authorization checked at the service and data layers.
- All input validated and sanitized.
- No execution of code from user-supplied data (including workflow
  definitions).
- Secrets are never logged.
- Rate limiting on all endpoints.
- JWT tokens are short-lived (24h) and non-revocable (known limitation;
  token revocation/logout endpoint planned for a future milestone).

---

## Deployment

### Minimum requirements

- CPU: 1 core minimum (2+ recommended)
- RAM: 512 MB minimum (1 GB+ recommended)
- Disk: 1 GB minimum
- Network: HTTP/HTTPS access

### Supported configurations

- **Development**: PostgreSQL + local file storage (single command).
- **Production**: PostgreSQL + S3-compatible object storage.
- **Container**: Docker image, runnable with `docker run`.
- **Orchestrated**: Kubernetes manifests provided (future).

### Configuration

Configuration is via environment variables and/or a YAML config file.
See `docs/architecture/configuration.md`.

---

## Observability

- **Logging**: Structured JSON logs to stdout/stderr.
- **Metrics**: Not exposed. A Prometheus exporter will be added in a later milestone.
- **Tracing**: OpenTelemetry support.
- **Health**: Health check endpoint (`/health`).
- **Alerting**: Operators configure alerting based on metrics.

---

## Technology decisions

The following technology decisions have been finalized and recorded as ADRs:

| Decision | ADR |
|----------|-----|
| Why Go for the backend | [ADR-0002](docs/decisions/0002-why-go.md) |
| Why PostgreSQL | [ADR-0003](docs/decisions/0003-why-postgresql.md) |
| Why REST + OpenAPI | [ADR-0004](docs/decisions/0004-why-rest-openapi.md) |
| Why React/TypeScript (future frontend) | [ADR-0005](docs/decisions/0005-why-react-typescript.md) |

**Backend**: Go 1.23+, compiled to a single static binary.
**Database**: PostgreSQL 16.
**API**: HTTP REST with JSON, OpenAPI 3.0 specification as the source of truth.
**Frontend**: Not implemented in Milestone 0.1; React + TypeScript chosen for the future frontend (ADR-0005).
**Deployment**: Docker container image; Docker Compose for local development.

---

## Roadmap / Phase 1 scope

Phase 1 (this document and associated foundation files) establishes:

- The modular monolith architecture (ADR-0001).
- Technology decisions (ADR-0002 through ADR-0005).
- Domain model and module boundaries.
- API-first approach and standards.
- Data, security, and deployment principles.
- Threat model.

Implementation begins in milestone 0.1 (Foundation).
