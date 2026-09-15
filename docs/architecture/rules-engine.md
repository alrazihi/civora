# CIVORA 0.5: Rules & Eligibility Engine — Architecture Specification

Version: 0.5 (design) · Status: Proposal · Branch: main (clean, HEAD `4a4bd7`)

## 1. Summary

The Rules & Eligibility Engine introduces deterministic, multi-tenant, auditable
**eligibility evaluation** for CIVORA cases. It replaces the legacy
`internal/eligibility` module with a declarative **rule-set** model authored
via API (JSON), evaluated by an embedded rule engine, and observed by the
**Workflow Engine** so that form/state transitions remain the authoritative
source of truth for workflow progression.

Design constraints honored:
- **No redesign** of Workflow Engine, Dynamic Forms, Form Submissions,
  Multi-tenancy, or Audit architecture.
- **No AI implementation**; the rules engine is a pure, deterministic function.
- **Frontend/visual builder** is explicitly out of scope — rule sets are created
  via API as JSON documents.

### Non-goals (explicitly excluded)

- Visual rule builder, drag-and-drop UI, or natural-language authoring.
- AI/ML-based evaluation, prediction, or recommendation.
- Replacement of human decision authority (`decisions` module).
- Redesign of multi-tenancy, audit, or workflow engines.

## 2. Goals & Non-Goals

| Goal | Non-goal |
|------|----------|
| Deterministic, auditable eligibility evaluation bound to a case. | Replacing human `decisions` authority. |
| Multi-tenant isolation (per-org rule sets). | Visual rule builder / UI. |
| Reuse existing workflow state + form submission data as fact sources. | AI/ML evaluation or prediction. |
| Audit every evaluation + rule-set change via existing `audit_events`. | Redesigning workflow/forms/audit. |
| API-driven rule-set CRUD + manual & automatic evaluation. | Native SQL rule language. |
| Rules remain advisory; workflow remains authoritative. | |

## 3. Stakeholders & Use Cases

### Stakeholders
- **Caseworker/Staff** (`role: staff`): runs manual evaluations, views
  eligibility results, overrides outcomes as a **human decision**.
- **Administrator** (`role: admin`): creates/updates **rule sets**, configures
  automatic evaluation triggers, views evaluation history.
- **System (Workflow Engine)**: consumes automatic evaluation results to drive
  deterministic state transitions.

### Use Cases
1. Admin creates a **rule set** bound to a case, referencing form fields and
   case attributes as facts.
2. Admin configures **automatic evaluation** on form submission (or case events).
3. Staff triggers a **manual evaluation** for a case and inspects the detailed
   reasoning trace.
4. Workflow Engine reads evaluation results to enable/disable transitions.
5. Admin **deactivates** a rule set without deleting history (archival).
6. Auditor queries `audit_events` for rule-set changes and evaluations.

## 4. Terminology

- **Rule Set**: A versioned, named collection of rules bound to an organization
  and (optionally) a case. Replaces one legacy eligibility config.
- **Rule**: A single conditional expression with an outcome (`eligible` /
  `ineligible` / `manual_review`) and optional priority.
- **Fact**: An observable datum used in evaluation — form submission field
  values, case attributes, person attributes (read-only references).
- **Evaluation**: A single run of a rule set against a case's current facts,
  yielding an outcome + reasoning trace.
- **Trigger**: A condition that causes an automatic evaluation
  (`form_submitted`, `form_version_published`, `manual`).

## 5. Architecture Overview

```
                    ┌──────────────────────────────────────────┐
                    │  API Layer  (internal/rules/api)         │
                     │   POST/GET/PUT /rule-sets            │
                     │   POST /cases/{id}/evaluations       │
                     └───────────┬────────────────────────────┘
                                 │ gRPC/internal interfaces
                     ┌───────────▼────────────────────────────┐
                     │  Application (internal/rules/application)│
                     │   - CreateRuleSet                      │
                     │   - Evaluate                           │
                     │   - ListEvaluations                    │
                     └───────────┬────────────────────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
        ▼                        ▼                        ▼
┌─────────────────┐   ┌──────────────────┐   ┌──────────────────┐
│ Form Submission │   │ Workflow Domain  │   │ People/Cases     │
│ (read-only)     │   │ (TransitionObsd.)│   │ (read-only)      │
│ shared interface│   │ shared interface │   │ shared interface │
└─────────────────┘   └──────────────────┘   └──────────────────┘
                                 │
                     ┌───────────▼────────────────────────────┐
                     │  Domain (internal/rules/domain)          │
                     │   - RuleSet, Rule, Evaluation, Trace   │
                     │   - Engine (pure function)             │
                     └───────────┬────────────────────────────┘
                                 │
                     ┌───────────▼────────────────────────────┐
                     │  Infrastructure                      │
                     │   - postgres repository              │
                     │   - audit event emission (existing)  │
                     │   - migrations (0019/0020)           │
                     └────────────────────────────────────────┘
```

### Layering
Follows the established modular-monolith convention
(`internal/<module>/{domain,application,infrastructure/postgres,api}`). The
module is `internal/rules`.

### Cross-module dependencies (read-only)
The rules engine **reads** from other modules via **shared interfaces** defined
in `internal/shared`, per the existing pattern (`shared.CaseFinder`,
`shared.PersonFinder`, `shared.UserChecker`). New interfaces:

```go
// shared/rules.go (proposed — added to shared pkg, no change to existing contracts)
type FormSubmissionFinder interface {
    FindLatestByFormAndCase(ctx context.Context, orgID, formID, caseID uuid.UUID) (*submissionsdomain.Submission, error)
}
type FormFieldSource interface {
    // Returns the published form version's fields keyed by field key for a form.
    FindPublishedFields(ctx context.Context, orgID, formID uuid.UUID) (map[string]FormFieldView, error)
}
```

> The rules module must never write to forms/submissions/workflow/cases. It is a
> consumer; persistence of outcomes (if any) is via human `decisions` or
> workflow transitions, both outside rules-engine authority.

## 6. Module Structure (Go)

```
internal/rules/
├── domain/
│   ├── rule_set.go        // RuleSet, RuleSetStatus, Version
│   ├── rule.go            // Rule, Operator, Condition, Outcome, Priority
│   ├── evaluation.go      // Evaluation, EvaluationStatus, Outcome
│   ├── engine.go          // Evaluate(ctx, RuleSet, Facts) -> Evaluation (pure)
│   ├── trace.go           // Step-by-step reasoning trace nodes
│   ├── errors.go          // ErrorCode-style domain errors
│   └── rules.go           // domain-level factory validators
├── application/
│   ├── service.go         // RuleSet CRUD, ScheduleEvaluation, RunManual
│   ├── evaluator.go       // orchestrates facts gathering + engine + audit
│   └── dto.go             // request/response DTOs (JSON rule-set schema)
├── infrastructure/
│   └── postgres/
│       └── repository.go  // rule_set + evaluation persistence
└── api/
    └── rules.go           // chi router + handlers
```

### Conventions inherited
- UUIDs generated server-side (`uuid.New()`).
- All mutations audited via `internal/audit` domain (`AuditEvent`).
- Errors mapped to `shared.ErrorCode` sentinel pattern (e.g.
  `shared.ErrInvalidInput`, `shared.ErrConflict`).
- `shared.APIResponse[T]` envelope reused for handlers.
- API prefix: `/api/v1/organizations/{orgId}/...`.
- Auth middleware: `RequireSameTenant`, roles `admin`/`staff` enforced in
  handlers (not middleware) per existing modules.

## 7. Domain Model

### Rule Set (`rulesets`)
```
id              uuid PK
organization_id uuid   (FK orgs)
name            text
description     text
key             text   (lowercase, unique per org)
case_id         uuid?  (FK cases, nullable — optional binding)
version         int
status          enum { DRAFT, ACTIVE, ARCHIVED }
created_by      uuid
created_at      timestamptz
updated_at      timestamptz
```
- A rule set may be **global** (`case_id IS NULL`) or **case-bound**.
- Only one `ACTIVE` version per (`organization_id`, `key`, `case_id`).
- Archiving an active set is allowed; new edits create a new version (no
  in-place mutation of ACTIVE history).

### Rule
```
id              uuid PK
rule_set_id     uuid FK
priority        int  (0 = highest)
outcome         enum { ELIGIBLE, INELIGIBLE, MANUAL_REVIEW }
active          bool
conditions      jsonb  (see §8 Rule Schema)
created_at      timestamptz
```
- Rules are evaluated **in priority order** (ascending). Evaluation stops at the
  first rule whose conditions match (short-circuit). If no rule matches, the
  rule set's **default_outcome** applies.

### Evaluation
```
id              uuid PK
rule_set_id     uuid FK
case_id         uuid FK
status          enum { ELIGIBLE, INELIGIBLE, MANUAL_REVIEW, ERROR }
outcome         text
reason          text?
trace           jsonb  (step-by-step engine trace)
trigger         enum { MANUAL, AUTOMATIC }
performed_by    uuid?  (user id for MANUAL)
evaluated_at    timestamptz
facts_snapshot  jsonb  (frozen input facts at eval time)
```

## 8. Rule Expression Schema (JSON)

Rule conditions use a **constrained, JSON-based** expression language — no
arbitrary code execution. A condition is a single comparison; multiple
conditions combine via `and`/`or` with explicit operator whitelist.

```jsonc
{
  "key": "basic-eligibility-v1",
  "name": "Basic Eligibility",
  "case_id": null,
  "default_outcome": "MANUAL_REVIEW",
  "rules": [
    {
      "priority": 0,
      "outcome": "INELIGIBLE",
      "conditions": { "all": [           // AND
        { "fact": "income.amount",        // path into facts
          "operator": "gt",               // numeric > 
          "value": 0 },
        { "fact": "household.size",
          "operator": "gte", "value": 1 }
      ]}
    },
    {
      "priority": 1,
      "outcome": "ELIGIBLE",
      "conditions": { "any": [           // OR
        { "fact": "program.enrolled",
          "operator": "is", "value": true },
        { "fact": "income.amount",
          "operator": "lte", "value": 10000 }
      ]}
    }
  ]
}
```

### Operators (whitelist)
`eq` · `neq` · `gt` · `gte` · `lt` · `lte` · `in` · `not_in` · `is`
(boolean) · `is_not` · `exists` · `not_exists` · `contains` (substring/array)
· `matches` (regex, anchored) · `before` · `after` · `on_or_before` ·
`on_or_after` (RFC3339 date/date-time, UTC-normalised).

> Operators are **typed**: the engine validates that a numeric operator is only
> applied to numbers, `in`/`not_in` accept arrays, `matches` requires a string
> fact, and temporal operators require RFC3339 date/date-time strings on both
> operands. Mismatches yield `EvaluationStatus.ERROR` with a trace entry — never
> silent truthy/falsey coercion.

> Temporal semantics: both operands are parsed as RFC3339. A bare date
> (`YYYY-MM-DD`) is normalised to midnight UTC. Comparisons are performed in
> UTC so results are deterministic regardless of the caller's timezone.

### Fact paths
Dot-separated paths into the facts document. Supported prefixes:
- `form.<submission_key>.<field_key>` — from the latest form submission for the
  case (per form assignment from workflow).
- `case.<attribute>` — `status`, `created_at`, `workflow_state`, `age_days`,
  etc. (computed fields enumerated in §11).
- `person.<attribute>` — `age`, `income_year`, etc. (read-only, no PII
  authoring).

The exact enumerable fact set is published in the API spec (§10) and validated
against a schema registry at evaluation time.

## 9. Evaluation Lifecycle

1. **Trigger** fires (`form_submitted` or `case.state_changed` or manual).
2. **Fact Assembly** (`application/evaluator.go`): gather facts for the case via
   shared interfaces (`shared.FormSubmissionFinder`, `shared.CaseFinder`,
   `shared.PersonFinder`). Facts are **frozen** into `facts_snapshot`.
3. **Engine** (`domain/engine.go`): pure function
   `Evaluate(ctx, ruleSet, facts) -> Evaluation`. No side effects, no I/O —
   strictly deterministic given identical inputs.
4. **Decision**: outcome returned to caller; **no automatic action** on the case.
   The engine may surface the outcome via:
   - An audit event (`rules.evaluation.completed`), and
   - A return value that the **Workflow Engine** may observe (via its own
     observer/adapter, not via rules-engine writes) to gate transitions.
5. **Trace**: each evaluated rule + matched/not-matched condition is recorded
   in `trace` (JSON) for auditability.

### Automatic vs Manual
- **Manual**: staff POSTs `/evaluations`; runs immediately, result returned.
- **Automatic**: rule set declares `triggers: ["form_submitted"]`; an observer
  (implemented as a Workflow Engine `TransitionObserver` or a dedicated event
  listener that reuses the Workflow Engine's existing observer mechanism) invokes
  `RunAutomatic` on the rules application service. The rules engine itself does
  not own the observer — it exposes `Evaluate`.

## 10. API Design

All routes live under `/api/v1/organizations/{orgId}/`.

| Method | Path | Role | Description |
|--------|------|------|-------------|
| `POST` | `/rules/rule-sets` | admin | Create a rule set (v1=DRAFT). |
| `GET`  | `/rules/rule-sets` | any | List (filtered by case/status). |
| `GET`  | `/rules/rule-sets/{id}` | any | Read a rule set (incl. rules). |
| `PUT`  | `/rules/rule-sets/{id}/versions` | admin | Create a new version (copy + patch). Returns new `id`. |
| `POST` | `/rules/rule-sets/{id}/publish` | admin | Transition DRAFT→ACTIVE (validates rules). |
| `POST` | `/rules/rule-sets/{id}/archive` | admin | Transition ACTIVE/DRAFT→ARCHIVED. |
| `GET`  | `/rules/rule-sets/{id}/evaluations` | staff | Paginated evaluation history. |
| `GET`  | `/rules/rule-sets/{id}/evaluations/{evalID}` | staff | Trace + facts snapshot. |
| `POST` | `/cases/{caseId}/evaluations` | staff | Run a manual evaluation using the case-bound (or latest active global) rule set. |
| `POST` | `/rules/rule-sets/{id}/evaluate` | staff | Evaluate against current facts without persisting (preview). |

### OpenAPI
- Add `docs/architecture/api-spec.md` section: `rules` tags, schemas
  `RuleSet`, `RuleSetCreateRequest`, `RuleSetVersionRequest`, `Rule`, `Evaluation`,
  `EvaluationTrace`.
- New error codes: `RULE_SET_INVALID`, `RULE_SET_NOT_ACTIVE`,
  `EVALUATION_FAILED`, `RULE_OUTCOME_INVALID`.
- Follows existing `api/openapi/openapi.yaml` structure (server URL template,
  `securitySchemes: bearerAuth`).

## 11. Fact Catalog

The following computed/read-only facts are available to rules. This list is
extensible but versioned (semver on the rule-set schema) to preserve
determinism.

| Path | Type | Source | Notes |
|------|------|--------|-------|
| `case.status` | string | cases domain | enum |
| `case.workflow_state` | string | cases domain | current workflow state key |
| `case.created_at` | timestamp | cases domain | ISO-8601 |
| `case.age_days` | number | computed | `(now - created_at)/86400` |
| `case.age_at_application` | number | computed | at last form submission |
| `person.age` | number | people domain | years |
| `person.income_year` | number | people domain | most recent |
| `form.<key>.submitted_at` | timestamp | form_submissions | latest submission |
| `form.<key>.<field>` | any | form_submissions | latest submission, field value |

> **Determinism guard**: facts are **not evaluated live** by the engine for
> derived values like `age_days` during evaluation; instead the application
> layer computes and freezes all derived facts into `facts_snapshot` at eval
> time. This guarantees `Evaluate` is a pure function over a frozen JSON
> document.

## 12. Data Model & Persistence

### Schema (PostgreSQL)

Migration `0019_rules_et_tables.up.sql`:

```sql
CREATE TABLE rules.rule_sets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    case_id         UUID REFERENCES cases(id),
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    version         INT NOT NULL DEFAULT 1,
    status          TEXT NOT NULL DEFAULT 'DRAFT',
    default_outcome TEXT NOT NULL DEFAULT 'MANUAL_REVIEW',
    rules           JSONB,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX rule_sets_uniq_active
    ON rules.rule_sets (organization_id, key, case_id)
    WHERE status = 'ACTIVE';

CREATE TABLE rules.evaluations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_set_id     UUID NOT NULL REFERENCES rules.rule_sets(id),
    case_id         UUID NOT NULL REFERENCES cases(id),
    status          TEXT NOT NULL,
    outcome         TEXT NOT NULL,
    reason          TEXT,
    trace           JSONB,
    trigger         TEXT NOT NULL,
    performed_by    UUID REFERENCES users(id),
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    facts_snapshot  JSONB NOT NULL
);

CREATE INDEX evaluations_by_case ON rules.evaluations (organization_id, case_id);
```

- Dedicated `rules` schema for namespace isolation (matches audit schema
  pattern from migration `0008`).
- `rules` JSONB holds the full rule list + default_outcome for that version
  (immutable once ACTIVE). New versions copy+patch.
- Migration `0020_rules_et_audit_actions.up.sql`: extend audit action
  vocabulary with `ruleset.*`, `evaluation.*`, `fact.*` (per pattern in
  `0007`/`0018`).

### Legacy `eligibilities`
Left intact. Migration `0019` adds a comment that `eligibilities` is superseded;
deprecation removal is a future task. No new writes to `eligibilities` from the
rules engine.

## 13. Authorization & Tenancy

- **Tenant isolation**: enforced by `RequireSameTenant` middleware (already
  present) + `organization_id` column on every rules table; every query in the
  repository is scoped by `orgID`. Cross-org reads are impossible by
  construction (no global `GetByID` without `orgID`).
- **Roles**:
  - `admin` — full CRUD on rule sets in their org.
  - `staff` — run/list evaluations; read rule sets; cannot publish/archive.
  - Role checks are done **in handlers** (not middleware), per the existing
    identity conventions. No new `admin`-only middleware introduced.
- **No new permissions model**: reuses the existing JWT `role` claim. No RBAC
  matrix, no granular permissions beyond role.

## 14. Determinism, Auditing & Integrity

### Determinism
- `Evaluate` is a **pure function**: `f(RuleSet, Facts) -> Evaluation`. No RNG,
  no wall-clock inside the engine; the application layer supplies `facts_snapshot`
  as input.
- All derived facts (`age_days`, etc.) computed and frozen before evaluation.
- No floating-point nondeterminism: monetary/percentage comparisons use
  `decimal`/`big.Rat` in Go for exactness (imported via `math/big`); string
  comparisons are byte-exact.

### Auditing
Every rule-set lifecycle event and every evaluation emits an audit event
through the existing `internal/audit` domain:

| Event | Action key (audit.action CHECK vocabulary) |
|-------|--------------------------------------------|
| Create rule set | `ruleset.created` |
| Publish version | `ruleset.published` |
| Archive | `ruleset.archived` |
| Manual evaluation | `evaluation.manual` |
| Automatic evaluation | `evaluation.automatic` |
| Evaluation error | `evaluation.error` |

Audit events include: `organization_id`, `actor_id` (system for automatic),
`target` = rule-set/evaluation id, `detail` = outcome + trace reference.

### Integrity
- Audit hash-chain (`audit_events.previous_hash`/`hash`) covers rule-set and
  evaluation events (reuse existing `0008` isolation + `0018` vocabulary).
- Rule sets are immutable when `ACTIVE`; new versions get new `id`s and audit
  creation. This preserves traceability of what rules produced any evaluation.
- `facts_snapshot` is stored with each evaluation so past evaluations remain
  re-derivable even if source facts change.

## 15. Failure Modes & Resilience

| Failure | Handling |
|---------|----------|
| Rule-set invalid JSON/schema | Rejected at publish; `RULE_SET_INVALID`. |
| Fact missing for a referenced path | `EvaluationStatus.ERROR`, trace notes the missing fact; never coerces to falsey. |
| Operator/type mismatch | `EvaluationStatus.ERROR`, trace notes mismatch. |
| Dependency module (form/submission) temporarily unavailable | Evaluation aborted with `EVALUATION_FAILED`; automatic retry via observer backpressure is the observer's concern (out of rules-engine scope). |
| Concurrent version creation | `rule_sets_uniq_active` partial unique index prevents two ACTIVE versions; conflicts return `409` mapped to `shared.ErrConflict`. |

### Error mapping
All domain errors map to existing `ErrorCode` sentinels:
`ErrInvalidInput` (schema/operator) · `ErrConflict` (active-version clash) ·
`ErrNotFound` (missing rule set/case) · `ErrForbidden` (role) ·
`ErrUnprocessable` (evaluation failure, missing fact).

## 16. Testing Strategy

Follows established patterns from `test/integration/*`, `test/e2e/*`, and
`test/helpers/db.go`:

- **Unit (domain)**: `go test -short ./internal/rules/...` — covers the engine
  pure function, operator typing, trace generation, priority ordering, default
  outcome. No DB. Uses table-driven tests per Go convention.
- **Integration**: `go test -p 1 -count=1 ./internal/rules/...` with
  `civora_test` DB (Docker Compose `db`). Tests CRUD + publish/archive state
  machine + audit emission + end-to-end evaluation with fixture form
  submissions.
- **Engine determinism test**: evaluate the same `(RuleSet, Facts)` twice and
  require identical `trace` + `facts_snapshot` byte output.
- **Audit integrity test**: assert hash-chain continuity across a
  create→publish→evaluate sequence.
- **API integration**: chi `httptest` handlers; role-based auth enforced.
- **E2E (web)**: Not applicable — no frontend. Out of scope per constraints.

### Test fixtures
A `testdata/` package under `internal/rules/application` holds canonical
rule-set JSON examples (basic income, household size, manual_review fallback)
shared by unit + integration tests.

## 17. Implementation Roadmap (0.5)

Phased, branch-per-phase. Each phase gates on `go vet`, `gofmt -l .` == empty,
and `go test -short ./...` passing.

### Phase 1 — Domain + Engine (week 1)
- `internal/rules/domain`: `RuleSet`, `Rule`, `Engine`, `trace`, errors.
- JSON schema validator for rule expressions (operator whitelist + type checks).
- Pure `Evaluate`; unit tests for all operators + trace.

### Phase 2 — Persistence + Audit (week 2)
- Migrations `0019`/`0020`; `internal/rules/infrastructure/postgres` repo.
- Audit event emission wiring (reuse `internal/audit`).
- Integration tests for CRUD + audit.

### Phase 3 — Application + API (week 3)
- `internal/rules/application`: service, evaluator, DTOs.
- `internal/rules/api`: chi handlers + route registration in `server.go`.
- Fact assembly via shared interfaces; fact catalog validation.

### Phase 4 — Trigger Wiring & Workflow Integration (week 4)
- Automatic evaluation trigger via Workflow Engine `TransitionObserver`
  (collaboration, not replacement — rules engine exposes `Evaluate` only).
- Manual evaluation endpoint + UI-less preview.
- End-to-end integration tests with form submissions + workflow state.

### Phase 5 — Legacy Cutover Readiness (post-0.5)
- Migration job for `eligibilities` → `rule_sets` (not implemented here).
- Deprecation flag on legacy endpoints.

### Open questions (to be resolved before Phase 1)
1. Confirm the exact set of **read-only fact paths** the Workflow Engine exposes
   from workflow state (current `workflow_state`, `age_at_application`).
2. Confirm whether `form.<key>` references resolve to the **latest published
   form version** or the workflow-assigned version. (Default: latest published.)
3. Whether automatic evaluation results feed the Workflow Engine via a
   `TransitionObserver` (preferred) or a separate event listener.

---

*Spec authored against current main (HEAD `4a4bd7`). No code changes made.*
