# CIVORA Pre-1.0 Master Architecture Audit

**Stage:** 0.1–0.9 Master Architecture Reconstruction  
**Status:** HEAD `9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223` (2026-09-18)  
**Audit Type:** Comprehensive reconstruction against planned architecture  

---

## 1. Exact HEAD

| Field | Value |
|-------|-------|
| **Commit SHA** | `9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223` |
| **Commit message** | `build(api): modularize openapi specification and update milestone docs` |
| **Parent** | `ad92a70` |
| **Branch** | `main` |
| **Working tree** | Clean (0 changes) |
| **Go version** | `go1.23.4` (windows/amd64) |
| **Date** | 2026-09-18 |

---

## 2. Repository State

### 2.1 Build Verification

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l .` | CLEAN |
| `go test -short ./...` | PASS (70+ packages) |

### 2.2 Module Layout (Internal Packages)

The repository follows the modular monolith pattern defined in ADR-0001:

| Module | Responsibility | Version |
|--------|---------------|---------|
| `identity` | Users, credentials, sessions, tokens | ✓ Implemented |
| `organizations` | Tenants, settings | ✓ Implemented |
| `cases` | Cases, status, assignments | ✓ Implemented |
| `workflow` | Workflow definitions, instances, state machines, transitions | ✓ Implemented |
| `rules` | Rule sets, rules, evaluations | ✓ Implemented |
| `forms` | Form definitions, field types | ✓ Implemented |
| `form_submission` | Completed form submissions | ✓ Implemented |
| `evidence` | Evidence items, document metadata | ✓ Implemented |
| `assessment` | Needs assessments, recommendations | ✓ Implemented |
| `decisions` | Human decisions, rationale | ✓ Implemented |
| `assistance` | Assistance actions, service delivery | ✓ Implemented |
| `followup` | Follow-up scheduling, completion | ✓ Implemented |
| `people` | Person records, contact details | ✓ Implemented |
| `review_queue` | Human review work queue | ✓ Implemented |
| `audit` | Tamper-evident hash-chained audit log | ✓ Implemented |
| `ai` | AI observations, verified facts, provider abstraction | ✓ Implemented (disabled by default) |
| `casesummary` | AI-generated case summaries | ✓ Implemented |
| `documentintelligence` | AI-generated document analyses | ✓ Implemented |
| `casecontext` | Aggregated case context | ✓ Implemented |
| `operations` | Operations metrics, analysis, impact, exports | ✓ Implemented |
| `eligibility` | Legacy eligibility (superseded by rules) | ⚠️ PRE-BODY (not API-exposed) |

---

## 3. Capability Matrix

| CAPABILITY | PLANNED (0.1–0.9) | IMPLEMENTED | STATUS | NOTES |
|------------|------------------|-------------|--------|-------|
| **Workflow Engine** | Configurable, versioned state machine with definitions, instances, transitions, history | Configurable workflow definitions with states, transitions, terminal states, role-based authorization | IMPLEMENTED | Full workflow API, transitions require human decisions |
| **Workflow History** | Immutable record of every transition | `workflow_transition_history` table with actor, reason, metadata, `decision_id` linkage | IMPLEMENTED | Complete transition history |
| **Dynamic Forms** | 13 field types, versioning, state assignment | 13 field types, form versioning, workflow state assignment (required/optional) | IMPLEMENTED | Forms assigned to workflow states |
| **Form Submission** | Immutable data becoming facts for rules | Immutable `form_submissions` table, populates facts for rules engine | IMPLEMENTED | Submissions feed rules evaluation |
| **Rules Engine** | Deterministic, auditable eligibility evaluation as data (JSON) | Deterministic rule engine with typed operators, trace trees, DRAFT/PUBLISHED/ARCHIVED lifecycle | IMPLEMENTED | Rules never make decisions; human-in-the-loop |
| **Rule Evaluation** | Outcomes: ELIGIBLE/INELIGIBLE/REQUIRES_REVIEW/INFORMATION_REQUIRED/FLAG/SCORE/ERROR | Same outcomes; first-match-wins priority; full trace trees | IMPLEMENTED | Comprehensive trace for every evaluation |
| **Rule Set Triggers** | `form_submitted`, `workflow_transition`, `manual` | Same triggers; rule sets assigned to workflow states | IMPLEMENTED | Automatic evaluation on state transitions |
| **Evidence Management** | Document with provenance, verification state machine | Evidence with types, verification state (UNVERIFIED→VERIFIED/REJECTED/NEEDS_REVIEW), append-only history | IMPLEMENTED | Document `StorageKey` private via `json:"-"` |
| **Document Storage** | File upload, metadata, checksum | Local filesystem provider with path-traversal prevention, 10MB limit, SHA-256 checksum | IMPLEMENTED | Storage provider interface with local implementation |
| **Human Review Queue** | State machine for review, SKIP LOCKED | PENDING→ASSIGNED→IN_REVIEW→COMPLETED/ESCALATED/WAITING_INFORMATION | IMPLEMENTED | Uses PostgreSQL SKIP LOCKED for concurrent assignment |
| **Human Decisions** | Immutable with rationale, actor, timestamp | Immutable `decisions` table, versioned with supersession | IMPLEMENTED | Decoupled from rules (rules are advisory) |
| **Assistance Records** | Service delivery tracking | Assistance with type, description, status, responsible staff | IMPLEMENTED | Planned→In Progress→Completed states |
| **Follow-ups** | Scheduled follow-up and completion | Follow-up with scheduled date, outcome, notes | IMPLEMENTED | Linked to cases |
| **Audit Trail** | Hash-chained, tamper-evident, schema isolated | `audit.audit_events` schema-isolated, SHA-256 chain per org | IMPLEMENTED | Transactional consistency maintained |
| **Audit Vocabulary** | 100+ action types CHECK constraint | 100+ action types, includes `ruleset.*`, `evaluation.*`, `ai.*` | IMPLEMENTED | Extensive action vocabulary |
| **Multi-tenancy** | All data scoped by organization_id | All tables have `organization_id`, middleware enforces tenant isolation | IMPLEMENTED | No cross-tenant access possible |
| **Tenant Isolation** | Middleware, service, repository, database layers | `RequireSameTenant`, org-scoped queries, FK cascades | IMPLEMENTED | Verified by tests |
| **Authorization** | Role-based (admin/staff) | Same roles, enforced in handlers and middleware | IMPLEMENTED | Consistent role enforcement |
| **Operations Metrics** | Case volume, decision rates, evidence verification, workflow throughput | 11 metric types: case volume, workflow throughput, state duration, cycle time, aging cases, pending reviews, decisions, assistance, evidence verification, information required | IMPLEMENTED | Tenant-scoped, read-only |
| **Workflow Analysis** | State accumulation, duration anomalies, bottleneck identification | 6 analysis types: state accumulation, duration anomalies, state cycles, case aging, review backlog, workflow variants | IMPLEMENTED | Analyzes workflow efficiency |
| **Impact Intelligence** | Outcomes, trends, long-term measurements | Case outcomes tracking, trend analysis with `case_outcomes` table | PARTIAL | Outcomes tracked but limited impact analysis |
| **Exports** | JSON, CSV, report generation with PII sanitization | JSON/CSV exports with `SanitizeExportData` excluding PII | IMPLEMENTED | PII removed from exports |
| **AI Observations** | Evidence-level and case-level observations (optional, off by default) | Evidence/case-level observations with types: SUMMARY, ENTITY_EXTRACTION, CLASSIFICATION, INCONSISTENCY, MISSING_INFORMATION, RELEVANT_EVIDENCE, INCOMPLETE_DOCUMENTATION | IMPLEMENTED | **CRITICAL GAP**: Accepted observations NOT wired to rules/decisions |
| **AI Verified Facts** | When observations accepted, create verified facts with provenance | `ai_verified_facts` table with provenance on human accept/correct | IMPLEMENTED | Never fed into rules engine |
| **Case Summaries** | AI-generated summaries with types | FULL_SUMMARY, SITUATION, RELEVANT_INFORMATION, IMPORTANT_EVIDENCE, MISSING_INFORMATION, POTENTIAL_INCONSISTENCIES, RULE_EVALUATION_RESULTS, WORKFLOW_HISTORY, PREVIOUS_ACTIONS, AI_OBSERVATIONS | PARTIAL | Not utilized in rules/decision workflow |
| **Document Intelligence** | AI document analyses with types | CLASSIFICATION, TEXT_EXTRACTION, STRUCTURED_EXTRACTION, SUMMARIZATION, RELEVANCE_DETECTION | PARTLEMENTED | Not utilized in downstream workflows |
| **AI Provider Abstraction** | OpenAI, Local/Ollama, Noop (default off) | Same three providers, env-driven, off by default | IMPLEMENTED | Clean adapter pattern |
| **PII Sanitization** | Document content passes through redactor | `NewRedactingSanitizer()` applied to AI inputs | PARTIAL | Document content NOT actually retrieved (see Bug #1) |
| **AI Audit Trail** | All AI actions produce hash-chained audit events | `ai.observation.created`, `ai.observation.accepted`, `ai.observation.rejected`, `ai.observations_generated`, `ai.observation.failed` | PARTIAL | Audit events NOT transactional with data writes |
| **E2E Tests** | Playwright suite covering all workflows | 17 spec files: cases, dashboard, forms, workspace, form-management, operations-dashboard, workflow-analysis, impact-intelligence, export, documents, ai-observations | PARTIAL | Tests defined but require manual setup |
| **Integration Tests** | Platform config, tenant isolation, security, concurrency | `platform_configurability_test.go`, `tenant_isolation_test.go`, `security_regression_test.go`, `concurrency_test.go` | PARTIAL | Require PostgreSQL for full verification |
| **OpenAPI Spec** | Source of truth for all endpoints | 200+ component files, modular build system, valid spec (validated with redocly) | PARTIAL | AI endpoints NOT documented; spec version claims different than current |
| **Configuration** | Environment variables with examples | Full CIVORA_* variable set, `.env.example`, comprehensive defaults | IMPLEMENTED | AI configuration present and documented |

---

## 4. Architecture Diagram (Actual Implementation)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           INTERNAL MODULES                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌────────────┐  ┌─────────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │  API Layer │──│ Application     │──│ Domain       │──│ Infrastructure│    │
│  │ (chi)      │  │ Services        │  │ (Entities)   │  │ (Postgres,    │    │
│  │            │  │                 │  │              │  │  Storage)    │    │
│  └────────────┘  └─────────────────┘  └──────────────┘  └──────────────┘    │
│       │                    │                   │                │           │
│       ▼                    ▼                   ▼                ▼           │
│  ┌─────────┐  ┌──────────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │identity │  │workflow          │  │ai            │  │audit         │    │
│  │organizations│              │  │cases         │  │              │    │
│  │cases   │  │rules             │  │forms         │  │casesummary   │    │
│  │people  │  │form assignments  │  │evidence      │  │documentint.  │    │
│  │eligibility│               │  │assessment    │  │casecontext   │    │
│  │review_queue│              │  │decisions     │  │operations    │    │
│  │assistance│  │                │  │followup      │  │              │    │
│  └─────────┘  └──────────────────┘  └──────────────┘  └──────────────┘    │
│                             │                                               │
│                             ▼                                               │
│                    ┌────────────────┐                                       │
│                    │ aiapplication  │                                       │
│                    │ (AI provider   │                                       │
│                    │  abstraction)  │                                       │
│                    └────────────────┘                                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
                    ┌────────────────────────────────┐
                    │         DATABASE               │
                    │  PostgreSQL (schema-per-module)│
                    │  - public (cases, forms, etc.) │
                    │  - audit   (audit_events)       │
                    │  - rules   (rule_sets,         │
                    │             evaluations)       │
                    │  - ai      (ai_observations,   │
                    │             ai_verified_facts)  │
                    │  - ... (other schemas)         │
                    └────────────────────────────────┘
```

---

## 5. Architectural Drift Analysis

### 5.1 Major Drifts and Contradictions

| Area | Planned | Implemented | Status | Analysis |
|------|---------|-------------|--------|----------|
| **Legacy Eligibility** | Replaced by rules engine | `internal/eligibility/` still exists but not API-exposed | **STALE** | Legacy module kept for data compatibility; superseded by ADR-007 |
| **AI Integration** | Observation assistance, off by default, human-in-the-loop | Implemented but **NOT INTEGRATED** into rules/decisions pipeline | **PARTIAL** | Observations stored but never affect case processing |
| **OpenAPI Spec Versioning** | Spec is source of truth | Version 0.8.0, but AI endpoints missing | **STALE** | 4 AI routes not in spec despite being routable |
| **Documentation** | README claims 0.7 focus, AI "planned" | AI implemented in 0.8 | **STALE** | README/CHANGELOG not updated for milestones |
| **Audit Atomicity** | All mutations + audit in single transaction | AI module saves observations OUTSIDE transaction | **CONTRADICTORY** | Race condition: audit event may exist without data |
| **PII Sanitization** | Document content sanitized before AI | Document content NOT retrieved at all | **PARTIAL** | Accidental privacy protection via bug |
| **Outcome Tracking** | Long-term impact measurements | Only `case_outcomes` table, limited analysis | **MISSING** | No integration with decisions or assistance completion |

### 5.2 Duplicate Implementations

1. **Eligibility Module**: `internal/eligibility/` contains full CRUD service but is not registered in `main.go` routes. Data exists in migrations but superseded by rules engine.

2. **Multiple Rule Systems**:
   - Old `internal/eligibility/` - deprecated
   - New `internal/rules/` - active
   - `internal/rules/domain/engine.go` - pure evaluation
   - `internal/rules/application/service.go` - CRUD + evaluation

### 5.3 Dead Code / Unused Exports

| File | Issue |
|------|-------|
| `api/openapi/modular/paths/organizations-{orgId}-eligibilities.yaml` | Routes exist but underlying service in `eligibility/` not wired to main |
| `internal/rules/api/handler.go` | Rules handler exists but rules engine not integrated with workflow transitions |

### 5.4 Inconsistent Sources of Truth

| Component | API Source | Database Source | Discrepancy |
|-----------|------------|-----------------|-------------|
| Rules Engine | `/api/v1/organizations/{orgId}/rules/*` | `rules.rule_sets`, `rules.evaluations` | ✅ Consistent |
| Workflows | `/api/v1/organizations/{orgId}/workflows/*` | `workflow_definitions`, `workflow_instances` | ✅ Consistent |
| AI Observations | `/api/v1/.../ai/observations/*` | `ai_observations` | API exists but spec not updated |

### 5.5 Frontend/Backend Duplication

The frontend E2E tests in `web/e2e/tests/` mock the API. The frontend:
- Uses real API paths via `workflow_key` (not hardcoded demo data)
- Contains Playwright specs but requires manual `npm install`
- No production frontend shipped; CIVORA is API-first by design

---

## 6. Capability Status Legend

| Status | Definition |
|--------|------------|
| **IMPLEMENTED** | Fully working, tested, documented, API-exposed |
| **PARTIAL** | Core functionality exists but has notable gaps |
| **MISSING** | Planned but not yet implemented |
| **STALE** | Previously implemented but now obsolete/undocumented |
| **CONTRADICTORY** | Implementation contradicts planned architecture |
| **UNTESTED** | Implemented but tests not run/verified |
| **UNSAFE** | Security or correctness concerns present |

---

## 7. Critical Findings

### 7.1 Critical Gap: AI Observations Are Siloed (HIGH)

**Finding**: AI observations, after human acceptance, have **no effect** on the case processing pipeline.

**Evidence**:
- `internal/rules/application/case_integration.go` - does not read from `ai_observations`
- `internal/casecontext/domain/context.go` - context builder does not include accepted AI observations
- `internal/decisions/` - does not reference `ai_observations`
- `internal/review_queue/` - does not link to observations

**Impact**: AI investment delivers observability but not utility. The "identify missing information" role is not fulfilled.

### 7.2 Audit Non-Atomicity in AI Module (HIGH)

**Finding**: `GenerateObservations` records audit events with `nil` transaction and saves observations individually.

**Evidence**:
- `internal/ai/application/service.go:163-233`
- `shared.RecordAuditEventInTx(ctx, nil, ...)` called outside transaction
- `repo.Save(ctx, obs)` called without transaction wrapper

**Contradiction**: This violates the CIVORA principle that "audit writes occur in the same database transaction as the state change they record."

### 7.3 Document Content Not Retrieved for AI Processing (CRITICAL)

**Finding**: AI observations are generated from metadata only because document content is never retrieved.

**Evidence**:
- `internal/ai/application/service.go:207` - `DocumentContent` initialized with nil `Content`
- `internal/ai/infrastructure/provider/openai.go:251-256` - prompt includes `doc.Content` which is nil

**Risk**: If bug fixed without PII sanitization, privacy violation. Current "protection" is accidental.

### 7.4 Missing OpenAPI Documentation for AI Endpoints (MEDIUM)

**Finding**: Four AI endpoints are routable but not documented in OpenAPI spec.

**Endpoints**:
```
POST /api/v1/organizations/{orgId}/evidence/{evidenceId}/ai/observations/generate
GET  /api/v1/organizations/{orgId}/evidence/{evidenceId}/ai/observations
POST /api/v1/organizations/{orgId}/evidence/{evidenceId}/ai/observations/{observationId}/accept
POST /api/v1/organizations/{orgId}/evidence/{evidenceId}/ai/observations/{observationId}/reject
```

### 7.5 Outdated Documentation (MEDIUM)

| Document | Claimed State | Actual State |
|----------|--------------|--------------|
| README.md line 28-30 | "Current focus: Milestone 0.7 — Evidence & Documents" | 0.8 complete, 0.9 in progress |
| README.md line 507-513 | "Does CIVORA use AI? No." | AI implemented (disabled by default) |
| CHANGELOG.md | No 0.8 entry | Missing changelog for AI |
| ARCHITECTURE.md | No AI module in diagrams | AI module exists |

---

## 8. Missing Capabilities

### 8.1 Operations & Impact Intelligence (0.9)

| Capability | Status | Gap |
|------------|--------|-----|
| Operations Metrics | IMPLEMENTED | ✅ |
| Workspace Analysis | IMPLEMENTED | ✅ |
| Bottleneck Identification | IMPLEMENTED | ✅ |
| Case Outcomes Tracking | IMPLEMENTED | ⚠️ Limited - no integration with decisions |
| Trend Analysis | IMPLEMENTED | ✅ |
| Export with Sanitization | IMPLEMENTED | ✅ |

**Missing**:
- No database-backed AI settings per organization
- Outcome metrics not tied to actual assistance completion
- Limited impact intelligence (no longitudinal analysis)

### 8.2 Integration Points

| Integration | Planned (0.7–0.9) | Status |
|-------------|------------------|--------|
| AI → Rules Facts | Future or 0.9+ | **MISSING** |
| AI → Review Queue | Future | **MISSING** |
| AI → Case Context | 0.8 ADR-007 | **MISSING** |
| AI → Decisions | NEVER (human-in-loop) | ✅ Correct |

---

## 9. Architectural Principles Assessment

| Principle | Status | Notes |
|-----------|--------|-------|
| **Open source** | ✅ | Apache 2.0, all development public |
| **API-first** | ✅ | All functionality exposed via API |
| **Modular architecture** | ✅ | Clean module boundaries |
| **Secure by default** | ✅ | Auth on all endpoints, tenant isolation |
| **Privacy-aware** | ✅ | PII removed from metrics, storage keys hidden |
| **Human-centered** | ✅ | Rules advisory, decisions human |
| **Auditable** | ✅ | Hash-chained audit log |
| **Observable** | ✅ | Metrics endpoints, structured logging |
| **Interoperable** | ✅ | OpenAPI spec, standard formats |
| **Deployable on-premises** | ✅ | Docker, single binary |
| **Deployable in cloud** | ✅ | Docker, Kubernetes compatible |
| **Suitable for limited-infrastructure** | ✅ | Low resource footprint |
| **Vendor-neutral** | ✅ | Adapter patterns for providers |
| **Model-provider-neutral** | ✅ | OpenAI/Local/Noop abstraction |
| **Database-provider-neutral** | ✅ | PostgreSQL primary, SQLite possible |
| **Auditable AI** | ⚠️ PARTIAL | Audit events not transactional |

---

## 10. Test Coverage Summary

| Module | Unit Tests | Integration | E2E | Status |
|--------|------------|-------------|-----|--------|
| `domain` | ✅ | N/A | N/A | HIGH |
| `application` | ✅ | Partial | N/A | HIGH |
| `ai/domain` | 6 tests | ❌ | ❌ | MEDIUM |
| `ai/application` | 7 tests | ❌ | ❌ | MEDIUM |
| `ai/provider` | 16 tests | N/A | N/A | HIGH |
| `rules/domain` | ✅ | ❌ | ❌ | HIGH |
| `workflow/domain` | ✅ | ❌ | ❌ | HIGH |
| `audit/domain` | ✅ | ❌ | ❌ | HIGH |
| `cases/domain` | ✅ | ❌ | ❌ | HIGH |
| `evidence/domain` | ✅ | ❌ | ❌ | HIGH |
| `web/e2e` | N/A | Mocked | 17 spec files | PARTIAL |

**Missing Tests**:
- Audit atomicity in AI service
- Observation content size validation
- Confidence value range validation
- Observation type validation
- Review observation concurrency

---

## 11. Database Migrations Status

| Migration | Purpose | Status |
|-----------|---------|--------|
| 0001-0014 | Core schema (cases, identity, organizations) | ✅ Implemented |
| 0015 | Forms | ✅ Implemented |
| 0016-0018 | Workflow engine, audit vocabulary | ✅ Implemented |
| 0019-0020 | Rules engine tables and audit actions | ✅ Implemented |
| 0021 | Workflow state rule assignments | ✅ Implemented |
| 0022-0023 | Evaluations, rule templates | ✅ Implemented |
| 0024-0025 | Rule set triggers, schema fixes | ✅ Implemented |
| 0026-0030 | Decisions, review queue, evidence | ✅ Implemented |
| 0031-0034 | Evidence verification, documents | ✅ Implemented |
| 0035-0042 | AI observations, verified facts | ✅ Implemented |
| 0043 | Operations reporting indexes | ⚠️ **UNTESTED** |

**Missing Indexes** (identified in Stage 0.9 report):
- `cases(organization_id, workflow_state, created_at DESC)`
- `cases(organization_id, service_type, created_at DESC)`
- `cases(organization_id, priority, created_at DESC)`
- `workflow_transition_history(organization_id, occurred_at DESC)`

---

## 12. Recommended Remediation

### 12.1 Immediate (Blocker for 1.0)

1. **Fix AI Audit Atomicity**: Wrap observation save and audit recording in single transaction
2. **Wire AI Observations**: Integrate accepted observations into rules fact assembly and case context
3. **Update OpenAPI Spec**: Document AI endpoints, bump version

### 12.2 Short-term (High Priority)

4. **Document AI Integration Gap**: Update README and ADR-007 to clarify AI is advisory-only
5. **Implement Document Content Retrieval**: With PII redaction before AI processing
6. **Add Confidence Validation**: Range 0.0–1.0 for observation confidence
7. **Implement Observation Content Size Check**: Enforce `MaxObservationContentSize`

### 12.3 Medium-term (Before 1.0)

8. **Add Missing Database Indexes**: For reporting query performance
9. **Add Per-Organization AI Settings**: Allow tenant-level AI toggle
10. **Implement AI Observation Concurrency Protection**: SKIP LOCKED or versioning
11. **Add Retention Policies**: For `ai_observations`, `case_summaries`, `document_analyses`
12. **Update CHANGELOG.md**: Document all 0.8 and 0.9 features

### 12.4 Documentation Updates

13. **ARCHITECTURE.md**: Add AI module and operations module diagrams
14. **README.md**: Update milestone status and AI section
15. **ADR-007**: Add evolution plan for AI observations integration

---

## 13. Verdict

CIVORA at HEAD demonstrates substantial implementation of the 0.1–0.9 architecture plan with **modular monolith patterns** well-executed. The core platform capabilities (workflow engine, dynamic forms, rules engine, evidence management, audit trail, multi-tenancy) are **fully implemented and operational**.

However, **critical integration gaps prevent readiness for 1.0**:

1. **AI observations are siloed** — the hallmark 0.8 feature delivers observability but not influence on the deterministic pipeline
2. **Audit non-atomicity** in AI module violates the tamper-evident principle
3. **Documentation drift** across README, CHANGELOG, and OpenAPI spec

The platform is **operationally viable** for organizations that do not require AI features, but the AI module should be considered **preview functionality** until integrated.

**NOT READY FOR 1.0** until remediation items 1–3 are addressed.

---

## 14. References

- `README.md` — Project overview, demo scenario
- `docs/vision.md` — Vision and capabilities
- `docs/principles.md` — 20 architectural principles
- `docs/decisions/0001-initial-architecture.md` — ADR-001: Modular monolith
- `docs/decisions/0006-configurable-workflow-engine.md` — ADR-006: Workflow engine
- `docs/decisions/0007-ai-observation-integration-boundaries.md` — ADR-007: AI boundaries
- `docs/0.9-stage-01-architecture-readiness.md` — Stage 0.9 readiness
- `docs/0.9-final-release-gate.md` — 0.9 final gate verdict
- `docs/0.8-stage-01-ai-architecture-readiness.md` — AI audit
- `api/openapi/openapi.yaml` — OpenAPI specification
- `migrations/` — Database migrations (43 total)
- `internal/` — All internal modules
- `web/` — Frontend (static site SPA, E2E tests)
- `cmd/seed/main.go` — Demo seeding script

---

*Document generated by CIVORA Master Architecture Auditor - Stage 0.1*
*HEAD: 9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223*