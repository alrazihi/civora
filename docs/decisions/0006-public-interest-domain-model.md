# ADR-0006: Public-Interest Service Delivery Domain Model

## Status

Accepted

## Date

2026-09-07

## Context

CIVORA's existing `Case` module implements a generic CRUD case lifecycle (CREATED → OPEN → IN_REVIEW → RESOLVED → CLOSED). The hostile engineering review (finding G-1) identified that CIVORA is indistinguishable from a generic case management system. There is no beneficiary entity, no evidence model, no eligibility assessment, no human decision record, and no assistance tracking.

The purpose of CIVORA is to help public-interest organizations deliver structured, transparent, auditable, and accountable services to real people. The current data model does not express this purpose.

## Decision

We will introduce a first-class public-interest service delivery domain model built on top of the existing modular monolith. The model consists of the following bounded contexts:

1. **People** — First-class `Person` (beneficiary) entity. Distinct from `User` (authenticated staff). A Person does not require a CIVORA account.
2. **Cases (Service Requests)** — Extend the existing `Case` entity to represent a `ServiceRequest` with a public-interest service delivery lifecycle and a `person_id` foreign key.
3. **Eligibility** — Structured eligibility assessment per service request. Records criteria, result, explanation, assessor, and timestamp. Eligibility is advisory, not final.
4. **Evidence** — Evidence items linked to a service request (identity documents, proof of residence, referrals, staff notes, photographs).
5. **Assessment** — Staff assessment of the service request. Captures findings, needs identified, recommendation, assessor, and timestamp. Advisory.
6. **Decisions** — Explicit human decision entity. The only path to APPROVED/REJECTED. Requires an authorized human actor. No AI decision-making.
7. **Assistance** — Action/assistance recorded after approval. Tracks type, description, status, responsible staff, and timestamps.
8. **Follow-up** — Post-assistance follow-up record. Captures outcome, notes, scheduled/completed dates, and performer.
9. **Closure** — Domain-rule-gated closure of a service request.

### Service Request Lifecycle

```
NEW → IN_REVIEW → ASSESSMENT → DECISION_PENDING → APPROVED / REJECTED → IN_PROGRESS → FOLLOW_UP → CLOSED
```

### Key Principles

- **Human-centered**: Every consequential decision requires a human actor.
- **Auditable**: Every lifecycle transition generates an audit event.
- **Tenant-isolated**: Every query is organization-scoped. No cross-tenant data access.
- **No AI decisions**: AI may later assist with summarization, translation, or recommendations, but never makes consequential decisions.
- **PII-aware**: Sensitive personal information is never logged into audit metadata.

## Consequences

### Positive

- CIVORA now models a real public-interest service process, making its purpose obvious.
- The domain enforces structured workflow rather than arbitrary status mutation.
- Audit trail is complete and transactional.
- Tenant isolation is enforced at the data layer for all new entities.

### Negative

- The existing `Case` status enum and transition rules must change, which is a breaking change to the API contract.
- Existing tests and OpenAPI documentation must be updated.
- Migration `0003` adds new tables and alters the `cases` table.

### Neutral

- No new infrastructure dependencies are introduced.
- The existing audit hash chain is preserved and extended to new entities.
- All new modules follow the established domain/application/infrastructure/api pattern.

## Alternatives Considered

### Alternative 1: Create a parallel Service Request module

Keep the existing `Case` module unchanged and create a new `servicerequests` module with the full public-interest workflow.

**Rejected**: This would create two parallel case systems, confusing operators and developers. It would also be speculative infrastructure.

### Alternative 2: Extend Case with new statuses only

Add the new statuses to `CaseStatus` but keep the existing modules flat without new bounded contexts.

**Rejected**: This would create a giant service package, violating the bounded context principle. Eligibility, evidence, assessment, decisions, assistance, and follow-up are distinct domain concepts that deserve their own modules.

### Alternative 3: Build a workflow engine

Create a generic workflow definition and execution engine.

**Rejected**: Too speculative. The user explicitly said "Do NOT build a generic rules engine yet." Hard-coded domain rules are appropriate for v0.2.
