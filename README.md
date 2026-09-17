# CIVORA — Open Infrastructure for Public-Service Workflows & Decisions

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![CI](https://img.shields.io/github/actions/workflow/status/alrazihi/civora/ci.yml?branch=main&label=CI)](https://github.com/alrazihi/civora/actions)
[![Code of Conduct](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa)](./CODE_OF_CONDUCT.md)

CIVORA is open-source infrastructure for organizations that need to configure,
operate, evaluate, and audit public-service processes. It combines a
**configurable workflow engine**, **dynamic forms**, a **deterministic rules
engine**, and **human-in-the-loop decision making** into a single, auditable
platform for case management and service delivery.

## Mission

Governments, NGOs, humanitarian organizations, social organizations, and
other public-interest institutions should be able to create, operate, audit,
and improve complex service-delivery processes without proprietary lock-in.

CIVORA provides the open digital infrastructure to make that possible.

## Status

CIVORA is in **early development** (Milestones 0.x). The project is not yet
production-ready. See the [roadmap](./ROADMAP.md) and
[governance](./GOVERNANCE.md) for details on how to follow along or
contribute.

Current focus: **Milestone 0.8** — AI Observations & Evidence (0.7 complete).
(document upload, evidence chains, audit integrity verification, drag-and-drop
upload UI).

## Quick Start

```bash
# Start PostgreSQL
docker compose up -d db

# Run the server
go run ./cmd/civora

# Seed demo data (in another terminal)
go run ./cmd/seed
```

Open http://localhost:8080 and sign in with the demo credentials printed by the seed command.

## What Is CIVORA?

CIVORA is an **open-source workflow platform** for public-service and
humanitarian organizations that need to configure, operate, evaluate, and
audit multi-step service-delivery processes. It is built as **open
infrastructure** — not a single-purpose application — so that governments,
NGOs, and social-service organizations can define their own **case
management** workflows, **dynamic forms**, and **eligibility rules** without
modifying core code.

CIVORA brings four capabilities together:

1. **Workflow engine** — a configurable state machine that drives the case
   lifecycle. Organizations define states, transitions, and authorization
   requirements as data (versioned, tenant-scoped), then execute transitions
   through a single, auditable API.
2. **Dynamic forms** — structured data collection with 13 field types, form
   versioning, and the ability to assign forms to specific workflow states
   (required or optional). Forms feed the rules engine as facts.
3. **Rules engine** — a deterministic, auditable **eligibility engine** that
   evaluates rule sets against case facts. Rule sets are configured as JSON,
   versioned, and produce full trace trees explaining every comparison. Rules
   never make decisions; they inform them.
4. **Human-in-the-loop decisions** — rules evaluate, but humans decide. Every
   consequential decision is recorded by an authorized actor, with rationale,
   and is immutable once finalized.

All of this is backed by a **tamper-evident, hash-chained audit trail** that
records every state transition, form submission, rule evaluation, evidence
addition, and human decision.

### Who CIVORA serves

| Audience | Need |
|----------|------|
| **Governments** | Deliver social services, emergency response, permits, and benefits at scale while maintaining auditability and citizen trust. |
| **NGOs / Humanitarian** | Coordinate aid delivery, track beneficiaries, and maintain evidence chains in resource-constrained or crisis environments. |
| **Social service organizations** | Manage case workflows, collect documentation, and coordinate among multiple staff roles. |
| **Developers & integrators** | Extend CIVORA, write custom workflows, and integrate with existing institutional systems through a REST/OpenAPI interface. |
| **Auditors & researchers** | Inspect system behavior, verify compliance, and study service-delivery outcomes. |

CIVORA is **API-first**, **multi-tenant**, and runs on **Go** with a
**PostgreSQL** database. It is self-hosted, Docker-packaged, and released under
the Apache License 2.0.

## Why CIVORA exists

Public-interest institutions typically deliver complex services using
proprietary platforms, spreadsheets, email, and shared drives. The result is
vendor lock-in, fragmented tooling, and no meaningful audit trail for the
decisions that affect people's lives.

CIVORA addresses this by providing a single, open, modular platform that
institutions can self-host, audit, and extend:

- **Configurable, not coded** — workflows, forms, and rules are configuration,
  not source code. New processes (education assistance, health services,
  housing programs) are added by creating definitions through the API.
- **Auditable by design** — every action that affects state produces an
  immutable, hash-chained audit event.
- **Human authority is preserved** — rules inform, but humans decide. CIVORA
  never makes consequential decisions about people autonomously.
- **Open and portable** — Apache 2.0, single static Go binary, PostgreSQL, and
  Docker. No cloud-specific dependencies, no mandatory SaaS.

## Demo Scenario: Emergency Food and Shelter Assistance

The following scenario demonstrates the complete CIVORA workflow from request through closure.

### Organization Setup

An NGO called **"Emergency Response Org"** registers on CIVORA with slug `emergency-demo`. The first registered user becomes the organization administrator. Additional staff members are invited and assigned the `staff` role.

### Step 1: Create a Person

A field worker registers **Amina Hassan**, a 34-year-old mother of four who was displaced by flooding. Her preferred language is set to English, and her contact details are recorded.

### Step 2: Create an Assistance Request

The field worker creates a new **Emergency Food and Shelter Assistance** case:

- **Service type:** Emergency
- **Priority:** Urgent
- **Linked person:** Amina Hassan
- **Description:** "Family of 4 displaced by flooding, needs immediate food and shelter support"

The case is assigned case number `CAS-20260909-DEMO001` and enters status `NEW`.

### Step 3: Open and Review

The case worker opens the case (`NEW → OPEN`) and moves it into review (`OPEN → IN_REVIEW`). During review, the worker:

- **Adds eligibility assessment:** records criteria (displacement verified, income below threshold) and explanation.
- **Attaches evidence:** uploads identity document and proof-of-residence with storage references.

### Step 4: Assessment

The case moves to `ASSESSMENT`. A case manager records a needs assessment:

- **Findings:** Household of 4 displaced by flood. Verified identity and residence documents. Income below threshold.
- **Needs identified:** Emergency food, temporary shelter, clothing
- **Recommendation:** Approve emergency shelter placement and food package

### Step 5: Human Decision

The case enters `DECISION_PENDING`. A human reviewer (not an automated system) records the final decision:

- **Decision:** APPROVED
- **Reason:** "Meets all eligibility criteria. Assessment supports immediate shelter and food assistance."

**CIVORA enforces that AI cannot automatically approve, reject, or distribute assistance.** Every decision must be explicitly recorded by an authorized human actor and is immutable once finalized.

### Step 6: Assistance

After approval, the case moves to `IN_PROGRESS`. The organization creates an assistance record:

- **Type:** Shelter
- **Description:** Emergency shelter placement at City Shelter Center for 30 days
- **Responsible staff:** Assigned case manager
- **Status:** Planned → In Progress → Completed

Assistance records what was provided, who was responsible, and when it was delivered.

### Step 7: Follow-up

A follow-up is scheduled for 2026-10-15. After the visit, the case worker records the outcome:

- **Outcome:** Family stably housed and receiving ongoing support
- **Notes:** Weekly check-ins scheduled with case manager

### Step 8: Closure

Once the follow-up is complete and all obligations are satisfied, the case is moved to `FOLLOW_UP` and then closed (`CLOSED`). Closed cases cannot be reopened or transitioned to any other status.

### Audit Trail

Every state transition, evidence addition, assessment, decision, assistance action, and follow-up is recorded in a
tamper-evident, hash-chained audit log. The audit trail can be verified at any
time to confirm no event has been altered, and reviewed to verify who did what,
when, and why.

## Configurable Workflow Engine

CIVORA includes a **configurable workflow engine** that allows organizations to define service-delivery workflows without modifying core code. It is a first-class component of the platform, not an afterthought: every case is bound to a workflow instance, and every state transition is executed through the engine.

### Workflow Definitions

A **Workflow Definition** describes how a service-delivery process operates. It is a versioned, tenant-scoped configuration that includes:

- **States** — the stages a case can be in (e.g., `NEW`, `OPEN`, `IN_REVIEW`, `APPROVED`, `CLOSED`)
- **Transitions** — explicit, validated moves between states (e.g., `OPEN` → `IN_REVIEW`)
- **Branching** — support for decision points like `DECISION_PENDING` → `APPROVED` or `REJECTED`
- **Terminal states** — states that cannot transition further (e.g., `CLOSED`)
- **Authorization metadata** — optional role requirements per transition

### Workflow Instances

A **Workflow Instance** is the execution of a Workflow Definition for a specific Case. The instance retains the definition/version it started with, so existing cases are never affected by definition changes.

### Emergency Assistance as a Workflow

The existing Emergency Assistance workflow is now represented as a real Workflow Definition:

```
NEW → OPEN → IN_REVIEW → ASSESSMENT → DECISION_PENDING
                                     ├── APPROVED → IN_PROGRESS → FOLLOW_UP → CLOSED
                                     └── REJECTED → CLOSED
```

This means:

- The Emergency Assistance flow is **configuration, not code**.
- New workflows (e.g., Education Assistance, Health Services) can be added by creating new Workflow Definitions.
- The core case engine remains unchanged.

### API

The workflow engine exposes REST endpoints for:

- `GET /api/v1/organizations/{orgId}/workflows` — list definitions
- `POST /api/v1/organizations/{orgId}/workflows` — create definition
- `GET /api/v1/organizations/{orgId}/workflows/{id}` — get definition
- `PUT /api/v1/organizations/{orgId}/workflows/{id}` — update a draft definition
- `DELETE /api/v1/organizations/{orgId}/workflows/{id}` — delete a draft definition
- `POST /api/v1/organizations/{orgId}/workflows/{id}/activate` — activate definition
- `POST /api/v1/organizations/{orgId}/workflows/{id}/archive` — archive definition
- `GET /api/v1/organizations/{orgId}/cases/{caseId}/workflow` — get case workflow instance
- `GET /api/v1/organizations/{orgId}/cases/{caseId}/workflow/transitions` — list valid transitions
- `POST /api/v1/organizations/{orgId}/cases/{caseId}/workflow/transitions/{key}` — execute transition
- `GET /api/v1/organizations/{orgId}/cases/{caseId}/workflow/history` — get transition history

### Frontend

The frontend consumes the generic workflow API to:

- Display the current workflow state
- Show only valid available transitions
- Execute transitions through the backend
- Refresh workflow state after transitions
- Display transition history

The backend remains the sole authority for workflow state and transition validation.

### Versioning

Workflow Definitions are versioned. When a new version is created, existing cases continue using the version they started with. Only new cases use the latest active version.

### Security

- All workflow operations require authentication.
- Transitions can optionally require specific roles (`AllowedRoles`).
- Tenant isolation is enforced at the data and service layers.
- Invalid transitions are rejected server-side with appropriate error codes.

## Rules & Eligibility Engine

CIVORA includes a **deterministic rules engine** for evaluating eligibility and
decision logic as data, not code. Rule sets are authored via API as JSON,
evaluated against case facts, and produce explainable outcomes with full
trace trees.

### What rules do

A **Rule Set** is a versioned, tenant-scoped collection of rules. Each rule has:

- **Conditions** — logical expressions (AND/OR/NOT) comparing facts using
  typed operators (equality, numeric comparison, membership, existence, string
  matching)
- **Outcome** — one of: `ELIGIBLE`, `INELIGIBLE`, `REQUIRES_REVIEW`,
  `INFORMATION_REQUIRED`, `FLAG`, `SCORE`, or `ERROR`
- **Priority** — evaluation order (0 = first); **first match wins** with
  short-circuit

An **Evaluation** runs a published Rule Set against a frozen snapshot of facts
(form submissions, case attributes, person data) and produces:

- A human-readable outcome
- A full trace tree (what was checked, what values were used, why each
  condition passed/failed)
- An immutable audit record

### What rules are NOT

| Not a... | Explanation |
|----------|-------------|
| **Workflow engine** | Rules don't own case state or transitions. The Workflow Engine remains authoritative. |
| **Form builder** | Rules reference form fields as facts; they don't define forms. |
| **AI/ML system** | No prediction, scoring models, or natural-language processing. Pure deterministic logic. |
| **Code execution** | No arbitrary code, scripts, or expressions. Only whitelisted operators on typed data. |
| **Human decision replacement** | Outcomes are advisory. Final decisions are made by humans via the `decisions` module. |

### Versioning and lifecycle

Rule Set versions follow a DRAFT → PUBLISHED → ARCHIVED lifecycle. Published
versions are immutable; new versions are created by cloning. Every evaluation
stores both the rule set ID and version, so past evaluations are forever
reproducible from their stored facts snapshot and trace tree.

### Integration with workflow

Rule Sets can declare `triggers` (e.g., `form_submitted`). When a form is
submitted, the Workflow Engine's observer calls the Rules Engine to evaluate
the case. Evaluation outcomes are advisory — they inform human reviewers and
can gate workflow transitions, but never bypass them.

## Dynamic Forms

CIVORA's **dynamic forms** platform lets organizations collect structured
data without writing code. Form definitions specify fields, types, validation,
and options; forms are versioned and can be assigned to specific workflow
states (required or optional).

### Capabilities

- **13 field types**: text, textarea, number, decimal, date, datetime,
  boolean, select, multiselect, radio, checkbox, email, phone
- **Form versioning**: publish new versions without breaking existing
  submissions
- **State assignment**: pin a form version to a specific workflow state;
  required forms must be completed before transitioning out of that state
- **Validation**: required fields, min/max, regex, type checking — enforced
  on both client and server
- **Fact source**: submitted forms become structured facts that the rules
  engine evaluates (e.g., `form.income.amount`, `person.age`)

### Configuration independence

Organizations can define any combination of fields and forms, version them
independently, and assign them to workflow states — all through the API, with
no source code changes required.

## Auditability & Evidence

Every action that affects state in CIVORA produces an **audit event** recorded
in a dedicated PostgreSQL schema (`audit`), isolated from the `public` schema
that holds business data. Audit events are tamper-evident:

- **Hash chain**: each event carries a SHA-256 hash linked to the previous
  event for the same organization
- **Transactional consistency**: audit writes occur in the same database
  transaction as the state change they record — if the audit write fails, the
  state change is rolled back
- **Integrity verification**: every event returned by the audit API includes an
  `integrity_valid` flag; the system verifies all chains on startup and on
  background ticks

### What CIVORA records

| Action | Audit event type |
|--------|-----------------|
| Case created | `case.created` |
| Case status changed | `case.status_changed` |
| Workflow transition | `workflow.transitioned` |
| Rule set published | `ruleset.published` |
| Rule evaluation | `evaluation.created` |
| Evidence/document added | `evidence.created` |
| Decision recorded | `decision.created` |
| User authenticated | `auth.success` |
| Organization created | `organization.created` |

## Human-in-the-Loop Decisions

**Rules evaluate. Humans decide.** This is a core architectural principle of
CIVORA.

The workflow is: a case enters a decision state → the **rules engine**
evaluates eligibility and produces an advisory outcome with a full trace → a
**human reviewer** records the final decision with rationale → the workflow
proceeds only on authorized transitions.

- Rules never approve, reject, or allocate assistance
- Every consequential decision is recorded with `decision_maker`, `reason`,
  and `decided_at`
- Decisions are immutable once finalized
- CIVORA does not use AI for consequential decisions (AI assistance is
  planned for a future milestone behind human review gates)

## Multi-Tenant Architecture

CIVORA is a **multi-tenant platform**. All data is scoped by `organization_id`.
Tenant isolation is enforced at every layer:

- **Database level**: all queries are filtered by organization at the
  data-access layer; foreign keys enforce ownership
- **API level**: middleware validates that the `orgId` path parameter matches
  the JWT's `organization_id` claim
- **Service level**: all service methods are organization-scoped
- **Authorization**: role-based — each organization has admins and staff with
  appropriate permissions

## Use Cases

CIVORA is a configurable platform, not a single-domain application. It can
support service-delivery processes across many domains:

- **Humanitarian assistance** — coordinate emergency food, shelter, and medical
  aid; track beneficiaries; maintain evidence chains
- **NGO service delivery** — manage case workflows, collect documentation, and
  coordinate among staff roles
- **Social services** — process benefits eligibility, manage case loads, and
  coordinate care
- **Public benefit programs** — administer eligibility rules, assistance
  provisioning, and follow-up
- **Education assistance** — evaluate grant eligibility and manage student
  support cases
- **Healthcare support** — coordinate patient referrals, assessments, and
  follow-up care
- **Emergency assistance** — triage and track emergency requests through
  assessment, decision, and response
- **Community services** — handle permits, licenses, and community support
  workflows
- **Government service workflows** — automate and audit multi-step public
  service processes

The Emergency Assistance workflow is included as a demo scenario and starter
template. All workflows are configuration — new processes are created through
the API.

## Technical Overview

CIVORA is built as a **modular monolith** with clear module boundaries:

- **Backend**: Go 1.23+, compiled to a single static binary
- **Database**: PostgreSQL 16
- **API**: HTTP REST with JSON, OpenAPI 3.0 specification as the source of
  truth
- **Authentication**: Bearer tokens (JWT or opaque session tokens)
- **Deployment**: Docker container image; Docker Compose for local development
- **Frontend**: Lightweight static HTML/CSS/JS SPA served from `web/`
- **License**: Apache 2.0

The API exposes endpoints for all functionality. See `api/openapi/openapi.yaml`
for the full specification.

## FAQ

### What is CIVORA?

CIVORA is open-source infrastructure for configurable, auditable public-service
workflows, case management, forms, rules, and human decisions. It provides the
digital platform that governments, NGOs, and public-interest institutions need
to configure, operate, and audit service-delivery processes.

### Is CIVORA open source?

Yes. CIVORA is released under the Apache License 2.0. All development happens
in the open, and no functionality is reserved for paid tiers or private forks.

### What is CIVORA used for?

CIVORA is used to build, operate, and audit service-delivery processes.
Common use cases include humanitarian assistance coordination, NGO service
delivery, social services case management, public benefit program
administration, and government service workflow automation.

### Is CIVORA a workflow engine?

Yes. CIVORA includes a configurable workflow engine that models
service-delivery processes as versioned state machines with states,
transitions, terminal states, and role-based authorization. Workflow
definitions are configuration, not code.

### Does CIVORA support dynamic forms?

Yes. CIVORA's dynamic forms system supports 13 field types. Form definitions
are versioned and assignable to workflow states. Submissions become facts
for the rules engine.

### Does CIVORA have a rules engine?

Yes. CIVORA includes a deterministic, auditable rules engine for evaluating
eligibility and decision logic. Rule sets are configured as JSON via the API,
evaluated against case facts, and produce full trace trees. Rules never make
final decisions — they inform human decision-makers.

### Can organizations configure their own workflows?

Yes. All workflow, form, and rule configuration is stored in the database and
managed through the API. Organizations can create new workflows, define
forms, and write rules without modifying CIVORA source code.

### Can CIVORA support NGO or humanitarian workflows?

Yes. CIVORA is a configurable platform. The Emergency Assistance workflow is
provided as a demo, but organizations can configure any workflow they need —
including humanitarian coordination, NGO service delivery, and emergency
response processes.

### Does CIVORA support multi-tenancy?

Yes. CIVORA is a multi-tenant platform. All data is scoped by organization,
queries are filtered at the data-access layer, and role-based authorization
controls access within each organization.

### How are rules audited?

Every rule evaluation produces an immutable audit record with the rule set
version, facts snapshot, full trace tree, and outcome. Past evaluations are
forever reproducible. The audit trail is hash-chained and tamper-evident.

### Does CIVORA use AI?

AI observation assistance is available in 0.8, optional and off by default via
`CIVORA_AI_ENABLED` (default off). It supports Noop, Local, and OpenAI providers
and generates observations that always require human review. CIVORA's foundation
remains deterministic infrastructure: workflow, forms, rules, auditability, and
human decisions. AI is an observation aid, never an autonomous actor. CIVORA is
not an AI platform.

### Is CIVORA production-ready?

CIVORA is in early development (Milestones 0.x). It is not yet production-ready.
See the [roadmap](./ROADMAP.md) for details.

## Documentation

| Topic | Location |
|-------|----------|
| Vision and mission | [README](./), [docs/vision.md](./docs/vision.md) |
| Project principles | [docs/principles.md](./docs/principles.md) |
| Architecture overview | [ARCHITECTURE.md](./ARCHITECTURE.md) |
| Architecture decision records | [docs/decisions](./docs/decisions) |
| Rules engine | [docs/architecture/rules-engine.md](./docs/architecture/rules-engine.md), [docs/product/rules.md](./docs/product/rules.md) |
| Threat model | [docs/threat-model.md](./docs/threat-model.md) |
| Hostile review | [docs/hostile-review.md](./docs/hostile-review.md) |
| Development roadmap | [ROADMAP.md](./ROADMAP.md) |
| Changelog | [CHANGELOG.md](./CHANGELOG.md) |
| Contribution guide | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Governance model | [GOVERNANCE.md](./GOVERNANCE.md) |
| Security policy | [SECURITY.md](./SECURITY.md) |
| Code of conduct | [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) |

## License

[Apache License 2.0](./LICENSE)

Copyright 2026 CIVORA contributors.
