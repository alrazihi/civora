# ADR-0006: Configurable Workflow Engine

## Status

Accepted

## Context

CIVORA’s first vertical slice (Emergency Assistance Request) was implemented
with a hardcoded state machine in the Case module. This works for a single
workflow but makes CIVORA an application rather than a platform.

The architectural question is:

> Can CIVORA execute different service-delivery workflows from a workflow
> definition without changing the core case engine?

If the answer is yes, CIVORA becomes a reusable platform for public-interest
service delivery, not a one-off emergency-assistance app.

## Decision

Introduce a first-class **Workflow Engine** module with the following concepts:

- **Workflow Definition** — a versioned, tenant-scoped template describing
  states, transitions, and metadata.
- **Workflow Instance** — the execution of a definition for a specific case.
- **Workflow State** — a stage in a workflow (key, name, terminal flag,
  display order, responsible role).
- **Workflow Transition** — an explicit, validated move between states
  (key, from/to state, conditions, allowed roles, active flag).
- **Workflow Transition History** — immutable record of every transition.

### Key design choices

1. **Workflow Instance is the source of truth for case workflow state.**
   The Case table stores a `workflow_instance_id` foreign key but does not
   duplicate state logic.

2. **Definitions are versioned.** Existing cases retain the definition/version
   they started with. New cases use the latest active version.

3. **Transition execution is centralized.** All state changes go through
   `WorkflowService.ExecuteTransition`. No module bypasses the engine.

4. **Tenant isolation is enforced.** Definitions and instances are scoped by
   `organization_id`. Cross-tenant access is prevented at the repository and
   service layers.

5. **Authorization is data-driven.** Transitions can declare `allowed_roles`.
   The workflow service checks the actor’s role against this list.

6. **Conditions are extension points only.** The current implementation
   supports the data model for conditions but does not execute arbitrary code.
   No eval-like functionality is permitted.

7. **Audit integration is native.** Every transition creates an audit event
   using the existing audit vocabulary.

8. **Concurrency is safe.** Optimistic concurrency control (version column)
   prevents conflicting simultaneous transitions.

## Consequences

### Positive

- Emergency Assistance becomes a configuration (seeded definition) rather
  than hardcoded logic.
- New workflows (e.g., Education Assistance, Health Services) can be added
  by creating new Workflow Definitions without changing core code.
- The frontend can render transitions dynamically from the API.
- Audit and history are consistent across all workflows.
- Security rules (tenant isolation, RBAC) are enforced uniformly.

### Negative

- Additional database tables and queries for workflow metadata.
- Slightly more complex case creation (workflow instance must be started).
- Frontend must be updated to consume dynamic workflow APIs.

### Neutral

- The existing Emergency Assistance demo continues to work end-to-end.
- The case `status` field is still updated for backward compatibility with
  existing queries and UI assumptions.

## Alternatives considered

1. **Keep the hardcoded state machine.** Rejected because it prevents CIVORA
   from supporting additional workflows without code changes.

2. **External BPMN engine.** Rejected because it adds unnecessary complexity
   and external dependencies for the current scope.

3. **Workflow-as-code (Go structs).** Rejected because it requires code
   deployment for workflow changes, defeating the purpose of configurability.

## References

- [Workflow domain model](internal/workflow/domain/workflow.go)
- [Workflow application service](internal/workflow/application/service.go)
- [Workflow API handlers](internal/workflow/api/handler.go)
- [Database migration](migrations/0009_workflow_engine.up.sql)
- [OpenAPI specification](api/openflow/openapi.yaml)
