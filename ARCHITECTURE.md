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
│   Events (in-process) │ Config │ Logging │ Metrics    │
├─────────────────────────────────────────────────────────┤
│                    Data & Storage Layer                 │
│  Primary DB (PostgreSQL/SQLite) │ Object Storage (S3)   │
└─────────────────────────────────────────────────────────┘
```

---

## Module boundaries

Each module has a single responsibility and owns its data. Modules
communicate through:

- **Internal service interfaces** (in-process function calls with
  defined contracts).
- **Events** (in-process event bus) for cross-module notifications.
- **Shared database** for persistence, with per-module schema ownership.

| Module | Owns | Communicates with |
|--------|------|-------------------|
| **Identity** | Users, credentials, sessions, tokens, authz policies | Organizations (for tenant-scoped auth) |
| **Organizations** | Tenants, settings, organization-level config | Identity (auth context) |
| **Cases** | Cases, case status, case assignments | Workflow, Tasks, Documents |
| **Workflow** | Workflow definitions, workflow instances, state machines | Cases, Tasks, Forms, Documents, Policy |
| **Forms** | Form definitions, form submissions | Workflow (rendering in task steps) |
| **Documents** | Uploaded files, metadata, evidence chains | Cases, Forms, Workflow, Audit |
| **Tasks** | Tasks, task assignments, deadlines | Cases, Workflow, Notifications |
| **Notifications** | Notification templates, delivery, channels | Tasks, Cases, Workflow |
| **Audit** | Immutable audit log, event records | All modules (writes); external consumers (reads) |
| **Policy** | Policy definitions, rule evaluation | Workflow (gate conditions), Tasks (assignment rules) |
| **AI** | AI recommendation providers, prompt templates | (stubbed; activated in milestone 0.7) |
| **Integrations** | Adapter registry, external service connectors | Identity, Documents, Notifications |

### Communication rules

1. Modules do not access another module's database tables directly.
   They go through the owning module's service interface.
2. Module-to-module calls use the internal service interface.
3. Events are published for side effects and cross-module state changes.
   Consumers should handle events idempotently.
4. The Audit module is write-only from the perspective of other
   modules: all modules emit audit events, but only the Audit module
   writes to and reads from the audit store.
5. Direct cross-module calls are allowed for synchronous operations
   within a transaction boundary. For non-critical side effects, events
   are preferred.

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
  Belongs to one organization.
- **Workflow Instance** — a running execution of a Workflow Definition.
  Associated with a Case.
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

### Events

- `CaseCreated`, `CaseStatusChanged`, `CaseClosed`
- `WorkflowStarted`, `WorkflowCompleted`, `WorkflowFailed`
- `TaskAssigned`, `TaskCompleted`, `TaskEscalated`
- `FormSubmitted`
- `DocumentUploaded`, `DocumentDeleted`
- `DecisionMade`
- `PolicyEvaluated`
- (Future) `AIRecommendationGenerated`, `AIRecommendationReviewed`

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

## API

- **Protocol**: HTTPS (HTTP/1.1 or HTTP/2)
- **Format**: JSON request/response bodies
- **Authentication**: Bearer tokens (JWT or opaque session tokens)
- **Versioning**: URL path prefix (`/api/v1/`)
- **Specification**: OpenAPI 3.0, documented in
  `docs/architecture/api-spec.md` (to be created)
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

- Personal data is identified by a `data_classification` label.
- Personal data is stored encrypted at rest where configured.
- Personal data export and deletion are supported.
- Audit logs containing personal data follow the same retention and
  access controls.

### Retention

- Data retention is configurable per organization and data type.
- Default retention: 7 years for audit records (changeable by operator).
- Deleted data is soft-deleted by default; hard deletion is governed
  by retention schedule.

### Encryption

- TLS 1.2+ in transit (required in production).
- Encryption at rest is supported via database column-level encryption
  for personal data and via object-storage server-side encryption.
- Encryption keys are managed by the operator's key management system
  (KMS) where available.

### Backups

- Backup strategy is documented for operators.
- Backups include database and object storage.
- Backup encryption and access controls are the operator's
  responsibility.
- Audit log backups are append-only and separately retained.

### Export and portability

- Data export is available in JSON format via the API.
- Bulk export endpoints are available for administrators.
- Export respects tenant isolation and authorization.

---

## Security

See [docs/threat-model.md](docs/threat-model.md) for the full threat
model. Key security properties:

- Authentication on every request.
- Authorization checked at the service and data layers.
- All input validated and sanitized.
- No execution of code from user-supplied data (including workflow
  definitions).
- Audit log is append-only and tamper-evident.
- Secrets are never logged.
- Rate limiting on all endpoints.

---

## Deployment

### Minimum requirements

- CPU: 1 core minimum (2+ recommended)
- RAM: 512 MB minimum (1 GB+ recommended)
- Disk: 1 GB minimum
- Network: HTTP/HTTPS access

### Supported configurations

- **Development**: SQLite + local file storage (single command).
- **Production**: PostgreSQL + S3-compatible object storage.
- **Container**: Docker image, runnable with `docker run`.
- **Orchestrated**: Kubernetes manifests provided (future).

### Configuration

Configuration is via environment variables and/or a YAML config file.
See `docs/architecture/configuration.md` (to be created).

---

## Observability

- **Logging**: Structured JSON logs to stdout/stderr.
- **Metrics**: Prometheus-format metrics endpoint (`/metrics`).
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
**Database**: PostgreSQL 16 (SQLite supported for local development).
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
