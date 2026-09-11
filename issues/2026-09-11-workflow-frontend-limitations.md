# Workflow Administration Frontend — Known Limitations & Issues

**Date:** 2026-09-11  
**Author:** Kilo (Frontend Lead)  
**Status:** Resolved (backend gaps closed) / Open (frontend v1 limitations remain)
**Related work:** Frontend foundation for configurable workflows (workflow list, detail, and create/editor views)

---

## 0. Summary

The backend API gaps originally documented below have all been closed. The
remaining items are intentional frontend v1 limitations (documented in
Section 2) and are not blocking. This document is retained for historical
reference and to track the frontend-only limitations.

## 1. Backend API Status (verified 2026-09-11)

All workflow lifecycle endpoints are implemented and documented. The items
originally listed in this section as gaps have been resolved.

### 1.1 Update endpoint — IMPLEMENTED
- `PUT /organizations/{orgId}/workflows/{workflowId}` updates a draft
  workflow definition (key, name, description, version, initial state,
  states, transitions, metadata). Only draft definitions can be updated.
- The frontend exposes it via `app.wfUpdate(id, body)` and the Edit button
  on the workflow list.

### 1.2 Delete endpoint — IMPLEMENTED
- `DELETE /organizations/{orgId}/workflows/{workflowId}` deletes a draft
  workflow definition and its states and transitions. Non-draft
  definitions are rejected.
- The frontend exposes it via `app.wfDelete(id)` and the Delete button.

### 1.3 Per-state/transition mutation — NOT AVAILABLE (intentional)
- Only full-definition replace is supported (PUT). Incremental
  single-state or single-transition endpoints do not exist. This is a
  deliberate scope decision for v1; the editor performs a full replace.

### 1.4 `CreatedAt` on states/transitions — FIXED
- `CreateWorkflowDefinition` now sets `state.CreatedAt` and
  `transition.CreatedAt` to `time.Now().UTC()` before the batch INSERT
  (`internal/workflow/application/service.go:148-150, 161-163`).

### 1.5 OpenAPI spec — COMPLETE
- All workflow endpoints are documented in `api/openapi/openapi.yaml`,
  including `GET /organizations/{orgId}/workflows/{workflowId}`.
- The spec passes `npx @redocly/cli lint`.

---

## 2. Frontend Limitations (v1 — intentional, documented for future iterations)

### 2.1 No drag-and-drop state/transition editor
- States are reordered with up/down buttons; transitions are added/removed via buttons.
- A DnD BPMN-style editor is explicitly out of scope for this foundation pass.

### 2.2 Conditions editor is a raw JSON textarea
- Users must manually type valid JSON for `conditions` on transitions.
- No schema-assisted picker, autocomplete, or validation feedback until submit-time.

### 2.3 Allowed roles is comma-separated text
- Users type role names separated by commas (e.g., `admin, staff`).
- No role picker or autocomplete. Role validity is not checked client-side.

### 2.4 No search/filter in the workflow list
- Pagination is available, but there is no keyword search, status filter, or key filter.
- Large organizations with many workflow versions will need to page through results.

### 2.5 Mobile tables rely on horizontal scroll
- State and transition tables use `.table-wrap { overflow-x: auto }` for small screens.
- Acceptable for v1 but not an optimized mobile data-entry experience.

### 2.6 Service-domain sections remain hardcoded per-service-type
- `SERVICE_DOMAIN.sectionActionStates` maps specific workflow states to case-workspace section action buttons (Eligibility, Evidence, Assessment, Decision, Assistance, Follow-up).
- This is the remaining legacy assumption that ties the case workspace to a specific workflow sequence.
- **Why it persists:** These sections are genuine product areas in CIVORA's service-delivery lifecycle (Evidence, Decisions, Assistance, etc.). Fully decoupling them requires a per-workflow-definition section configuration in the backend, which does not yet exist.
- **Future direction:** Move section definitions into the workflow definition's `metadata` (or a dedicated field) so each workflow declares which sections apply and at which states they are actionable.

---

## 3. Frontend Bugs Fixed in This Pass

### 3.1 `TERMINAL_STATES` ReferenceError (critical)
- **Location:** `web/js/app.js:460` (original) — `const isTerminal = TERMINAL_STATES.includes(state);`
- `TERMINAL_STATES` was never declared, causing a `ReferenceError` whenever a case had no valid outgoing transitions.
- **Fix:** Terminal states are now derived from `workflow.definition.states` where `terminal === true` via `getTerminalStates(def)`.

### 3.2 Hardcoded `DEFAULT_TERMINAL_STATES` removed
- The legacy `['REJECTED', 'CLOSED']` list was used by `renderWorkflowProgress`.
- **Fix:** Progress rendering now reads terminal flags from the workflow definition.

---

## 4. Security / Correctness Notes

### 4.1 JWT role decoding (client-side)
- The login response does not include the user's role. The frontend decodes the JWT `role` claim to gate admin-only UI (Create/Activate/Archive).
- This is safe: the JWT is signed (not encrypted); the role claim is intentionally readable client-side. The server still enforces admin role on every mutation endpoint.

### 4.2 Client-side validation is best-effort
- Draft validation mirrors backend rules but is not a security boundary. The backend re-validates all inputs.
- Invalid JSON in the conditions textarea is silently defaulted to `[]` during payload construction; the server will also reject or coerce as appropriate.

---

## 5. Testing Limitations

- **No frontend test framework** is configured in this repository (no `package.json`, no test runner).
- Frontend correctness has been verified through:
  - `node --check` syntax validation on both `web/js/api.js` and `web/js/app.js`.
  - Manual code review against the backend API contracts (verified against handler source and OpenAPI spec).
- **Recommendation:** Add a minimal frontend test setup (e.g., Vitest + happy-dom) with smoke tests for the router, workflow draft validation, and API helper functions.

---

## 6. Backend Changes Included in This Commit

The following backend files were already modified in the working tree before this session and are included in this commit as part of the complete workflow-engine feature:

- `internal/workflow/api/handler.go` — workflow HTTP handlers (list, get, create, update, delete, activate, archive, case-workflow endpoints)
- `internal/workflow/api/handler_test.go` — handler tests including tenant isolation
- `internal/workflow/application/service.go` — `CreateWorkflowDefinition`, `ActivateWorkflowDefinition`, `ArchiveWorkflowDefinition`, case-workflow helpers
- `internal/workflow/application/service_second_workflow_test.go` — service-level tests
- `internal/workflow/domain/repository.go` — repository interface extensions (`SaveBatchTx`, `DeleteBatchByDefinitionIDTx`, `FindByFromState`)
- `internal/workflow/infrastructure/postgres/workflow_definition_repository.go` — status update support
- `internal/workflow/infrastructure/postgres/workflow_state_repository.go` — batch insert/delete/find
- `internal/workflow/infrastructure/postgres/workflow_transition_repository.go` — batch insert/delete/find

Plus one frontend-focused spec fix:
- `api/openapi/openapi.yaml` — fixed pre-existing YAML indentation error that prevented the OpenAPI lint from parsing (workflows `post` responses block and `activate` endpoint over-indentation).

---

*End of issues log — 2026-09-11*
