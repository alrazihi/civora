# ADR-001: Initial Architecture — Modular Monolith

- **Date**: 2026-09-06
- **Status**: Accepted
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA needs an architecture that:

1. Is simple enough to build and maintain in early development.
2. Supports clear separation of concerns (Identity, Organizations,
   Cases, Workflow, Forms, Documents, Tasks, Notifications, Audit,
   Policy, AI, Integrations).
3. Can be deployed on-premises and in cloud environments.
4. Is suitable for organizations with limited infrastructure.
5. Can evolve into a distributed architecture if and when the system
   grows into that complexity — but does not pay that cost now.
6. Avoids premature optimization, microservices complexity, and vendor
   lock-in.

Multiple architectural styles were considered.

## Decision

**Use a modular monolith as the initial architecture.**

CIVORA will be a single deployable artifact (a single binary or
container image) with clearly bounded, loosely-coupled internal modules.
Each module encapsulates a single domain (see module boundaries below)
and exposes its functionality through an internal interface layer.
The modules share a common data layer (a single database by default)
but communicate with each other through well-defined APIs or service
interfaces rather than direct database access.

The architecture follows these principles:

### Module boundaries

The following modules are defined with clear responsibilities:

| Module | Responsibility |
|--------|---------------|
| Identity | Authentication, authorization, user identity, token management. |
| Organizations | Tenant/multi-organization management, isolation, settings. |
| Cases | Case lifecycle, status tracking, assignment. |
| Workflow | Workflow engine, workflow definitions, workflow instances. |
| Forms | Form definitions, form rendering, form submission storage. |
| Documents | Document upload, storage, metadata, evidence chains. |
| Tasks | Task lifecycle, task assignment, task deadlines. |
| Notifications | Notification delivery (email, in-app, webhook, etc.). |
| Audit | Immutable audit logging, tamper-evident records. |
| Policy | Policy/rule evaluation engine, policy definitions. |
| AI | AI recommendation/advice engine (added in milestone 0.7; stubbed now). |
| Integrations | Adapter layer for external service providers. |

Modules communicate through:

- **Internal API calls** (in-process function/method calls with defined
  interfaces) for operations within the same process.
- **Database** for persistence, with each module owning its tables and
  foreign keys.
- **Events** (in-process event bus or pub/sub pattern) for
  cross-module notifications (e.g., "case created" triggers a
  "task assigned" action).

### Data layer

- A single primary database is used (PostgreSQL by default; SQLite is
  supported for development and small deployments).
- The database schema is organized by module ownership.
- Migration tooling is included.
- Each module's data access is encapsulated behind a repository
  interface, enabling future database changes or extraction.

### API layer

- A single HTTP API layer exposes all module functionality.
- Public API endpoints are defined via OpenAPI specification.
- All endpoints require authentication and authorization.
- API versioning is handled via URL prefix (`/api/v1/`).

### Deployment

- The system runs as a single process.
- In development, SQLite + local file storage is sufficient.
- In production, PostgreSQL + object storage (S3-compatible) is
  expected.
- Configuration is via environment variables and/or config files.
- Docker container images are published for deployment convenience.

### Testing

- Each module has unit tests for its core logic.
- Integration tests cover cross-module interactions.
- End-to-end tests cover API-level workflows.
- Audit-log tests verify immutability properties.

## Alternatives considered

### A1: Microservices from the start

**Rejected.** Microservices introduce significant complexity in
deployment, debugging, data consistency, and inter-service
communication. For a v0.1–1.0 product that does not yet have confirmed
scale requirements, this complexity is not justified. A modular
monolith can be split into services later if and when the system's
scale or team size warrants it.

### A2: Functional monolith (no module boundaries)

**Rejected.** Without clear module boundaries, the codebase would
degenerate into a tangled mess that is difficult to maintain or reason
about. Explicit module boundaries are required even in a monolith
to ensure long-term maintainability.

### A3: Event-sourced / CQRS-only architecture

**Rejected.** Event sourcing and CQRS provide powerful audit and
replay capabilities but introduce significant complexity and a steep
learning curve for contributors. CIVORA's audit requirements will be
met through an explicit audit-logging module that records relevant
events, without requiring the entire system to be event-sourced.
Event sourcing may be adopted for specific aggregates later if
beneficial.

### A4: Multi-process (separate binaries per module)

**Rejected.** This is effectively microservices without the benefits
of independent deployment. It adds operational complexity without
addressing the core need for separation of concerns, which can be
achieved within a single process.

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| Easier initial development | A monolith is simpler to build, test, and deploy initially. |
| Harder to scale individual components | If one module becomes a bottleneck, the whole application must be scaled. This is acceptable for early milestones. |
| Easier data consistency | A single database allows ACID transactions across modules where needed. |
| Module coupling depends on discipline | Without enforced physical separation, there is risk of modules calling each other directly. This is mitigated by code review and module-boundary documentation. |
| Future extraction is possible but requires care | The modular structure and repository pattern make future service extraction feasible, but it will require migration planning. |

## Assumptions

- The initial user base will be small to moderate in scale (tens to
  low hundreds of concurrent users per deployment).
- Most organizations deploying CIVORA have or can add a database
  administrator.
- Contributors will have familiarity with the chosen implementation
  language and framework.
- The system does not need to handle millions of requests per second
  initially.

## How this decision can be reversed or evolved

This decision establishes a **monolith-with-boundaries** architecture.
If and when scale or organizational needs change:

1. Individual modules can be extracted into separate services by
   replacing internal API calls with HTTP/gRPC calls.
2. The repository pattern allows swapping the database per module.
3. The event-based communication layer allows modules to be moved
   to separate processes with minimal code changes.
4. The OpenAPI API layer already decouples external clients from the
   internal structure.

Reversing this decision entirely (e.g., switching to a fully distributed
system from day one) would require a complete rewrite and is not
anticipated. However, the modular structure is designed to make
incremental evolution possible without a rewrite.

## Consequences

- Contributors can work within a single codebase and deploy with a
  single command.
- The project can deliver value quickly without infrastructure
  complexity.
- The architecture supports the project's stated goals of being
  on-premises deployable, cloud-deployable, and suitable for
  limited-infrastructure organizations.
- Future growth is possible without an immediate rewrite, but will
  require careful planning and migration.

## References

- [CIVORA Vision](docs/vision.md)
- [CIVORA Principles](docs/principles.md)
- [CIVORA Threat Model](docs/threat-model.md)
- [Martin Fowler: "Microservices"](https://martinfowler.com/articles/microservices.html)
- [Martin Fowler: "Monolith First"](https://martinfowler.com/bliki/MonolithFirst.html)
