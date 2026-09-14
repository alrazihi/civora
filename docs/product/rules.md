# CIVORA Rules Engine — Product Documentation

Version: 0.5.1

---

## What Rules Are

The **Rules Engine** is a deterministic, auditable decision evaluation system. It lets organizations configure **eligibility and decision logic** as data (JSON) rather than code.

A **Rule Set** is a versioned collection of rules. Each rule has:
- **Conditions** — logical expressions (AND/OR/NOT) comparing facts
- **Outcome** — one of: `ELIGIBLE`, `INELIGIBLE`, `REQUIRES_REVIEW`, `INFORMATION_REQUIRED`, `FLAG`, `SCORE`, `ERROR`
- **Priority** — evaluation order (0 = first)

An **Evaluation** runs a published Rule Set against a frozen snapshot of facts (form submissions, case attributes, person data) and produces:
- Outcome + human-readable explanation
- Full trace tree (what was checked, what values were used, why each condition passed/failed)
- Immutable audit record

---

## What Rules Are NOT

| Not a... | Explanation |
|----------|-------------|
| **Workflow engine** | Rules don't own case state or transitions. Workflow Engine remains authoritative. |
| **Form builder** | Rules reference form fields as facts; they don't define forms. |
| **AI/ML system** | No prediction, scoring models, or natural-language processing. Pure deterministic logic. |
| **Code execution** | No arbitrary code, scripts, or expressions. Only whitelisted operators on typed data. |
| **Human decision replacement** | Outcomes are advisory. Final decisions are made by humans via the `decisions` module. |
| **Case management** | Rules don't create, assign, or close cases. |

---

## Rule Lifecycle

```
DRAFT → PUBLISHED → ARCHIVED
  │         │
  │         └─► Can be evaluated (manual or automatic)
  │
  └─► Editable: add/remove rules, change conditions, outcomes
       New versions created via "Create Version" (clones + increments version)
       Published versions are IMMUTABLE — never modified in place
```

**Key invariants:**
- Only one `PUBLISHED` version per `(org, key, case_id)` at a time
- `DRAFT` versions can be freely edited
- `ARCHIVED` versions are retained for history/audit
- Every lifecycle change emits an audit event (`ruleset.created`, `ruleset.published`, `ruleset.archived`, `ruleset.version_created`)

---

## Versioning

- **Version numbers** are integers (1, 2, 3...), auto-incremented on clone
- **Key** is the stable identifier (e.g., `income_eligibility`, `education_grant_v2`)
- **Evaluation records** store `rule_set_id` AND `rule_set_version` — past evaluations are forever reproducible
- **Trace + facts_snapshot** stored with each evaluation — exact re-execution possible

---

## Operators (Whitelisted)

| Category | Operators | Types |
|----------|-----------|-------|
| Equality | `eq`, `neq` | string, number, boolean, null |
| Numeric | `gt`, `gte`, `lt`, `lte` | number only (exact `big.Rat` comparison) |
| Membership | `in`, `not_in` | value must be array of scalars |
| Boolean | `is`, `is_not` | boolean only |
| Existence | `exists`, `not_exists` | no value required |
| String | `contains`, `matches` (regex) | string only |

**Type safety:** Operators validate at rule-set publish time AND evaluation time. A string `"500"` compared with numeric `lte 500` → `ERROR`, never silent coercion.

---

## Outcomes

| Outcome | Meaning | Typical Use |
|---------|---------|-------------|
| `ELIGIBLE` | Criteria fully met | Auto-approve path |
| `INELIGIBLE` | Criteria not met | Auto-reject path |
| `REQUIRES_REVIEW` | Borderline / needs human | Escalate to caseworker |
| `INFORMATION_REQUIRED` | Missing facts | Request more documents |
| `FLAG` | Risk indicator | Fraud check, priority flag |
| `SCORE` | Numeric score output | Priority ranking |
| `ERROR` | Evaluation failure | Missing fact, type mismatch |

**Default outcome** is configurable per Rule Set (applies when no rule matches).

---

## Missing Data Handling

| Situation | Result |
|-----------|--------|
| Fact path not found in snapshot | `UNKNOWN` → rule doesn't match → next rule or default outcome |
| Fact value is `null` | `exists` → FALSE, `not_exists` → TRUE, typed operators → `ERROR` |
| Type mismatch (string vs number) | `ERROR` — never coerced |
| All rules `UNKNOWN` | Default outcome = `INFORMATION_REQUIRED` |

The trace explicitly records every `UNKNOWN` with `reason: "fact path not found"`.

---

## Explanation Format

Every evaluation returns a **trace tree**:

```json
{
  "trace": [
    {
      "id": "1",
      "node_type": "root",
      "description": "evaluate rule set income_eligibility",
      "children": [
        {
          "id": "2",
          "node_type": "rule",
          "description": "rule priority 0",
          "result": "TRUE",
          "children": [
            {
              "id": "3",
              "node_type": "group",
              "description": "AND",
              "result": "TRUE",
              "children": [
                {
                  "id": "4",
                  "node_type": "leaf",
                  "field": "form.income.amount",
                  "operator": "lte",
                  "expected": "500",
                  "actual": "300",
                  "actual_type": "number",
                  "result": "TRUE",
                  "reason": "300 <= 500"
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

**Non-technical readable fields:**
- `description` — human summary at each level
- `field` — fact path (e.g., `form.income.amount`)
- `operator` — what comparison (`lte`, `eq`, etc.)
- `expected` / `actual` — values used
- `result` — `TRUE` / `FALSE` / `UNKNOWN` / `ERROR`
- `reason` — plain English why

---

## Human Decision Boundary

**Rules never make final decisions.**

| Rules Engine Output | Human Decision (via `/decisions`) |
|---------------------|-----------------------------------|
| `ELIGIBLE` | `APPROVED` / `REJECTED` / `NEEDS_MORE_INFORMATION` |
| `INELIGIBLE` | `REJECTED` (or override to `APPROVED`) |
| `REQUIRES_REVIEW` | Caseworker reviews and decides |
| `INFORMATION_REQUIRED` | Request docs, re-evaluate |
| `FLAG` | Review flagged case |
| `SCORE` | Use for prioritization |

The `decisions` module records: `decision_maker`, `reason`, `decided_at` — full accountability.

---

## Workflow Integration

Rules integrate with the Workflow Engine via **observers**:

1. **Automatic evaluation** — Rule Set declares `triggers: ["form_submitted"]`. Workflow Engine's `TransitionObserver` calls `EvaluateRuleSet` on form submission.
2. **Manual evaluation** — Staff POST `/rule-sets/{id}/evaluate` with custom facts.
3. **Results** — Evaluation outcome stored in `rules.evaluations`. Workflow Engine can read evaluations to gate transitions (e.g., "only transition to APPROVED if latest evaluation is ELIGIBLE").
4. **No writes** — Rules Engine never mutates case state, workflow state, or form submissions.

---

## Rule Templates (Reuse Across Organizations)

**Rule Templates** are reusable rule snippets at two scopes:

| Scope | Owner | Use Case |
|-------|-------|----------|
| `GLOBAL` | Platform admin | "Minimum Age 18", "Income ≤ Threshold" — common patterns |
| `ORG` | Organization admin | Tenant-specific reusable patterns |

**Workflow:**
1. Admin creates template: `POST /rules/templates` with a single `Rule`
2. Staff lists templates: `GET /rules/templates?scope=GLOBAL&category=eligibility`
3. When building a Rule Set, staff instantiates: `POST /rules/templates/{id}/instantiate` → returns a `Rule` ready to insert

Templates are **copies**, not references — modifying an instantiated rule doesn't affect the template.

---

## API Summary

| Method | Path | Role | Description |
|--------|------|------|-------------|
| `POST` | `/rules/rule-sets` | admin | Create Rule Set (DRAFT) |
| `GET` | `/rules/rule-sets` | staff | List (filter by case, key, status) |
| `GET` | `/rules/rule-sets/{id}` | staff | Get Rule Set + rules |
| `PATCH` | `/rules/rule-sets/{id}` | admin | Update DRAFT |
| `POST` | `/rules/rule-sets/{id}/version` | admin | Clone to new DRAFT version |
| `POST` | `/rules/rule-sets/{id}/publish` | admin | Validate + activate |
| `POST` | `/rules/rule-sets/{id}/archive` | admin | Archive active version |
| `DELETE` | `/rules/rule-sets/{id}` | admin | Delete DRAFT only |
| `GET` | `/rules/rule-sets/{id}/versions` | staff | Version history |
| `POST` | `/rules/rule-sets/{id}/evaluate` | staff | Run evaluation (persists) |
| `GET` | `/rules/evaluations/{id}` | staff | Get evaluation + trace |
| `GET` | `/rules/cases/{caseId}/evaluations` | staff | Case evaluation history |
| `GET` | `/rules/fields` | staff | Discoverable form fields |
| `POST` | `/rules/templates` | admin | Create template |
| `GET` | `/rules/templates` | staff | List templates (filter scope, category) |
| `GET` | `/rules/templates/{id}` | staff | Get template |
| `POST` | `/rules/templates/{id}/instantiate` | staff | Get rule copy for Rule Set |
| `DELETE` | `/rules/templates/{id}` | admin | Delete template |

All endpoints require `BearerAuth` + matching `organization_id` claim.

---

## Configuration Test: Two Organizations, Zero Code Changes

**Organization A — Emergency Housing**

```json
{
  "key": "emergency_housing",
  "name": "Emergency Housing Eligibility",
  "default_outcome": "REQUIRES_REVIEW",
  "rules": [
    {
      "priority": 0,
      "outcome": "ELIGIBLE",
      "conditions": {
        "all": [
          { "field": "form.housing_assessment.status", "operator": "eq", "value": "HOMELESS" },
          { "field": "form.income_verification.amount", "operator": "lte", "value": 500 }
        ]
      }
    },
    {
      "priority": 1,
      "outcome": "INELIGIBLE",
      "conditions": {
        "field": "form.housing_assessment.status", "operator": "eq", "value": "HOUSED"
      }
    }
  ]
}
```

**Organization B — Education Grant**

```json
{
  "key": "education_grant",
  "name": "Education Grant Eligibility",
  "default_outcome": "INELIGIBLE",
  "rules": [
    {
      "priority": 0,
      "outcome": "ELIGIBLE",
      "conditions": {
        "all": [
          { "field": "form.student_assessment.enrollment_status", "operator": "eq", "value": "ENROLLED" },
          { "field": "form.student_assessment.gpa", "operator": "gte", "value": 2.5 },
          { "field": "form.income_verification.amount", "operator": "lte", "value": 30000 }
        ]
      }
    },
    {
      "priority": 1,
      "outcome": "INFORMATION_REQUIRED",
      "conditions": {
        "field": "form.student_assessment.enrollment_status", "operator": "not_exists"
      }
    }
  ]
}
```

**No source code changes.** Both configured entirely via API.

---

## Rule Reuse Example: Minimum Age

**Global Template** (platform-provided):
```json
POST /rules/templates
{
  "scope": "GLOBAL",
  "key": "minimum_age_18",
  "name": "Minimum Age 18",
  "category": "eligibility",
  "rule": {
    "priority": 0,
    "outcome": "INELIGIBLE",
    "conditions": {
      "field": "person.age",
      "operator": "lt",
      "value": 18
    }
  }
}
```

**Org A instantiates** for Emergency Housing rule set:
```bash
POST /rules/templates/{template-id}/instantiate
# → returns Rule { priority: 0, outcome: INELIGIBLE, conditions: { field: "person.age", operator: "lt", value: 18 } }
# Insert into Org A's emergency_housing rule set
```

**Org B instantiates** for Education Grant rule set:
```bash
POST /rules/templates/{template-id}/instantiate
# → same Rule, different priority if needed
# Insert into Org B's education_grant rule set
```

**Semantic safety:** Template only defines the *condition logic*. Each org sets its own `outcome` and `priority` when instantiating — preventing unsafe reuse where domain meaning differs.

---

## Rule Composition: Multiple Outcomes in One Rule Set

A single Rule Set can produce different outcomes via priority ordering:

```json
{
  "rules": [
    { "priority": 0, "outcome": "FLAG",       "conditions": { "field": "form.fraud_indicator", "operator": "is", "value": true } },
    { "priority": 1, "outcome": "INFORMATION_REQUIRED", "conditions": { "field": "form.income_verification.amount", "operator": "not_exists" } },
    { "priority": 2, "outcome": "SCORE",      "conditions": { "field": "form.risk_score", "operator": "gte", "value": 80 } },
    { "priority": 3, "outcome": "ELIGIBLE",   "conditions": { "field": "form.income_verification.amount", "operator": "lte", "value": 1000 } },
    { "priority": 4, "outcome": "REQUIRES_REVIEW", "conditions": { "field": "form.income_verification.amount", "operator": "lte", "value": 5000 } }
  ],
  "default_outcome": "INELIGIBLE"
}
```

**Evaluation order:**
1. Fraud flag → immediate `FLAG` (stops)
2. Missing income → `INFORMATION_REQUIRED` (stops)
3. High risk score → `SCORE` (stops)
4. Low income → `ELIGIBLE` (stops)
5. Medium income → `REQUIRES_REVIEW` (stops)
6. Nothing matched → default `INELIGIBLE`

No domain-specific code needed. Pure configuration.

---

## Rule Priority: Independent by Design

- Rules evaluate in **ascending priority order** (0, 1, 2...)
- **First match wins** — short-circuit
- **No dependencies** between rules — each rule is self-contained
- If you need "Rule B only if Rule A didn't match", just give B a higher priority number
- **No chaining, no forward references, no cycles** — keeps evaluation deterministic and explainable

---

## Testing & Simulation (Safe)

```bash
POST /api/v1/organizations/{orgId}/rules/rule-sets/{ruleSetId}/evaluate
{
  "facts": {
    "form": {
      "income_verification": { "amount": 300 },
      "housing_assessment": { "status": "HOMELESS" }
    },
    "person": { "age": 25 }
  },
  "trigger": "MANUAL"
}
```

**Returns:**
```json
{
  "outcome": "ELIGIBLE",
  "matched_rule_id": "...",
  "trace": [...],
  "facts_snapshot": {...},
  "evaluated_at": "2026-09-14T10:00:00Z"
}
```

**Safety guarantees:**
- Does NOT alter the case
- Does NOT create an evaluation record unless `case_id` provided
- Requires authentication + org-scoped authorization
- Read-only operation

---

## Case Evaluation View

In the Case Workspace, staff see:

| Rule Set | Version | Outcome | Explanation | Timestamp |
|----------|---------|---------|-------------|-----------|
| emergency_housing | v3 | ELIGIBLE | Income ≤ 500 AND housing = HOMELESS | 2026-09-14 10:00 |
| emergency_housing | v2 | REQUIRES_REVIEW | Missing income verification | 2026-09-13 14:30 |

Full trace expandable per row.

---

## Getting Started (Admin Checklist)

1. **Create forms** for data collection (e.g., `income_verification`, `housing_assessment`)
2. **Publish forms** and assign to workflow states
3. **Create Rule Set** via `POST /rules/rule-sets` (DRAFT)
4. **Add rules** using field keys from `GET /rules/fields`
5. **Validate** via `GET /rules/rule-sets/{id}` (checks condition syntax)
6. **Test** via `POST /rules/rule-sets/{id}/evaluate` with sample facts
7. **Publish** via `POST /rules/rule-sets/{id}/publish`
8. **Configure triggers** in Rule Set (`triggers: ["form_submitted"]`)
9. **Monitor** evaluations in `GET /rules/cases/{caseId}/evaluations`

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Evaluation returns `ERROR` | Type mismatch (string vs number) | Ensure form field types match operator expectations |
| Evaluation returns `INFORMATION_REQUIRED` | Fact path missing | Check form submitted; verify fact path spelling |
| Rule not matching | Priority too low / condition too strict | Test with `/evaluate`; adjust priority or conditions |
| "Rule set key already exists" | Duplicate key in same org+case scope | Use different key or archive old version first |
| Template not visible | Wrong scope filter | Use `?scope=GLOBAL` or `?scope=ORG` |

---

## Related Documentation

- [Architecture Spec](architecture/rules-engine.md) — Technical design
- [OpenAPI Spec](../api/openapi/openapi.yaml) — Complete API reference
- [Workflow Engine](architecture/workflow-engine.md) — Workflow integration
- [Dynamic Forms](architecture/forms.md) — Form definitions as fact sources