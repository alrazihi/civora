# Frontend: Generic Workflow-Driven Cases — Gap Report

## Objective
Transform `web/` from Emergency-Assistance-specific logic into a generic,
workflow-definition-driven case operation experience: select a workflow,
create a case bound to it, and operate through actual workflow states/transitions.

## Completed work (frontend)
- Workflow selector in new-case flow; service type derived from workflow key.
- `createCase` selects workflow → derives service_type → POST → verifies
  returned instance `workflow_definition_id` matches selection.
- Generic terminal-state computation (`computeTerminalStateSet` /
  `isCaseTerminal`); section status labels (Completed/Available/Not applicable).
- Cached transitions reused; `loadCaseSections(id, transitions)` race fixed.
- Security: `escapeHTML` XSS fix; duplicate `workflow-state-name` id fixed.
- Dashboard uses computed terminal-state set (no hardcoded `openStatuses`).
- Bug fix: `renderWorkflowSelector(defs, preselectId)` now declares the
  parameter it already referenced (previously threw
  "preselectId is not defined", breaking the new-case selector).
- Added a Playwright E2E harness under `web/e2e/` covering the workflow
  selector, case creation with workflow verification, the custom-workflow
  creation guard, and dashboard stats/case listing. Runs fully offline
  (static server + API mocking).

## API contract gaps (backend required)
These block the fully-dynamic journey for service-type AND custom workflows.

### 1. Create case does not accept explicit workflow selection
- **Endpoint**: `POST /organizations/{orgId}/cases`
- **Problem**: Only binds a workflow via `service_type` through
  `WorkflowKeyForServiceType` (EMERGENCY→emergency_assistance, etc.).
  Ignores any `workflow_definition_id` in the payload.
- **Frontend behavior**: Derive `service_type` from selected workflow key;
  for keys with no mapping (custom workflows), creation is blocked with an
  explicit error. Verification compares instance definition id to selection.
- **Required backend change**: Add `workflow_definition_id` (uuid) to the
  create-case request body. When provided, instantiate that exact definition
  (skip key/service_type fallback). Validate the definition belongs to the org
  and is ACTIVE. Update OpenAPI spec + request decoder.
- **Acceptance**: Frontend can send `workflow_definition_id` directly instead
  of deriving `service_type`; selection works for any custom workflow.

### 2. Workflows list endpoint returns all statuses
- **Endpoint**: `GET /organizations/{orgId}/workflows`
- **Problem**: No `?status=` filter; returns every definition including
  non-active versions. Frontend filters `ACTIVE` locally.
- **Desired backend enhancement**: Accept `?status=active|all` query param
  and `GET /workflows?status=active` to return only active definitions by key.

### 3. Transition payload lacks metadata
- **Endpoint**: `POST /organizations/{orgId}/cases/{caseId}/workflow/transitions`
- **Problem**: Response only carries `{success, data:{status}}`. Missing:
  - `allowed_roles` (frontend cannot role-gate transitions).
  - `conditions` (why a transition is forbidden).
  - `description` (human-readable reason for restriction).
- **Required backend change**: Return transition metadata in the response
  envelope: `allowed_roles []string`, `conditions []string`,
  `description string`.

### 4. Statistics endpoint lacks workflow breakdown
- **Endpoint**: `GET /organizations/{orgId}/cases/dashboard/statistics`
- **Problem**: Returns `by_status`/`by_service_type` but no `by_workflow`.
- **Desired backend enhancement**: Add `by_workflow []WorkflowCount` to the
  statistics response so the dashboard can render a generic workflow
  breakdown without deriving it from `by_service_type`.

### 5. Workflow history lacks pagination
- **Endpoint**: `GET /organizations/{orgId}/cases/{caseId}/workflow/history`
- **Problem**: Returns full history; no pagination for long-running cases.
- **Desired backend enhancement**: Add `limit`/`offset` query params and
  `total` in meta for pagination.

## Files
- `web/js/app.js` — core logic
- `web/js/tests.js` — frontend test runner
- `web/index.html` — views (new-case workflow selector, dashboard)
- `web/css/style.css` — selectors/breakdown styles
- `web/js/api.js` — unchanged client wrapper

## Verification done
- `go build ./...`
- `go vet ./...`
- `gofmt -l .` (empty)
- `go test -short ./...` (all ok)
- `go test -v ./internal/forms/domain/ ./internal/workflow_form_assignment/application/ ./internal/cases/application/` — 36 domain tests pass
- `node --check web/js/app.js web/js/tests.js` (pass)
- OpenAPI lint: `npx @redocly/cli lint api/openapi/openapi.yaml` — valid
- Playwright E2E (`web/e2e`): 34/35 pass; 1 flaky (`shows Forms nav button for admin users`, `#btn-forms-nav` visibility timing issue)
- Integration tests (`test/integration/`): cannot run — no PostgreSQL/Docker available in this environment

## New findings from audit (dynamic forms & data collection)

### F1. Dead code: duplicate CaseFormService (CRITICAL)
- **File**: `internal/form_submission/application/service.go:37`
- **Problem**: 730-line `CaseFormService` is a near-verbatim duplicate of
  `CaseService` form methods (`GetCaseForms`, `SubmitForm`,
  `GetWorkflowRequirements`, validation, etc.). Zero production usage —
  not registered in `main.go`, not exposed via any API handler.
- Only referenced in `test/integration/form_submission_test.go` which tests
  the duplicate instead of the production `CaseService`.
- **Impact**: Maintenance hazard; tests validate dead code, not the real path.
- **Fix**: Delete `internal/form_submission/application/` entirely. Point
  integration tests at `caseSvc.GetCaseForms` / `caseSvc.SubmitForm` etc.

### F2. Frontend/backend API path mismatch for form assignments (CRITICAL)
- **Files**: `web/js/api.js:248-259` vs
  `internal/workflow_form_assignment/api/handler.go:25`
- **Problem**: Frontend calls `/organizations/{orgId}/forms/{formId}/assignments`
  for assignment CRUD. Backend registers at
  `/organizations/{orgId}/workflows/{workflowId}/form-assignments`.
- Every frontend assignment API call returns 404 against the real backend.
- The Playwright mocks (`web/e2e/tests/helpers.ts:340`) mirror the frontend's
  wrong path, so E2E passes but real integration fails.
- **Fix**: Update `api.js` to use `/workflows/{workflowId}/form-assignments`
  and update E2E mocks accordingly.

### F3. Frontend/backend field type mismatch (HIGH)
- **Files**: `web/js/form-renderer.js:28-32` vs
  `internal/forms/domain/form.go:81-94`
- **Problem**: Frontend uses lowercase types (`text`, `email`, `phone`,
  `multiselect`). Backend stores uppercase (`TEXT`, `EMAIL`, `PHONE`,
  `MULTISELECT`). Without normalization, `EMAIL`/`PHONE` fall through to the
  `text` default, losing email/phone regex validation and correct input types.
- **Fix**: Normalize field types in `form-renderer.js` (lowercase the type
  before the switch) or emit uppercase from backend (breaking; avoid).

### F4. Frontend/backend validation key mismatch (HIGH)
- **Files**: `web/js/form-renderer.js:56-138` vs
  `internal/cases/application/service.go:1290-1324`
- **Problem**: Frontend `form-renderer.js` reads `min`, `max`, `min_length`,
  `max_length`, `pattern`. Backend validation domains accept both
  snake_case and camelCase (`validateCommonFieldValidation`), BUT the
  case service `validateFieldValue` only reads `minLength`, `maxLength`,
  `minValue`, `maxValue` — NOT the snake_case variants the frontend sends.
- A field submitted with `{min_length: 2}` passes frontend validation but
  server-side length validation is silently skipped.
- **Fix**: Align validation key names across frontend and backend. Use
  camelCase consistently: `minLength`, `maxLength`, `minValue`, `maxValue`.

### F5. No RBAC on form management and assignment routes (HIGH)
- **Files**: `internal/forms/api/handler.go:40-57`,
  `internal/workflow_form_assignment/api/handler.js:24-42`
- **Problem**: Form CRUD (create, version, publish, archive, add/update fields)
  and form-assignment CRUD have NO role-based access control. Any authenticated
  user — including staff — can reconfigure forms and workflow assignments.
  Compare with `internal/cases/api/handler.go` which gates admin endpoints
  with `RequireAnyRole("admin")`.
- **Fix**: Add `middleware.RequireAnyRole("admin")` to write routes on both
  form and assignment handlers.

### F6. E2E test server missing forms/assignments wiring (HIGH)
- **File**: `test/e2e/e2e_test.go:192-203`
- **Problem**: The E2E test server does not register the form handler,
  assignment handler, or call `caseService.SetFormRepos()`. Zero E2E coverage
  for the entire dynamic forms pipeline.
- **Fix**: Register `formHandler` and `assignmentHandler` in `SetupTestServer`
  and wire up `SetFormRepos` on the case service (as `main.go` does).

### F7. OpenAPI spec missing endpoints (MEDIUM)
- **File**: `api/openapi/openapi.yaml`
- **Problem**: Missing `/organizations/{orgId}/workflows/{workflowId}/form-assignments`
  and `/organizations/{orgId}/cases/{caseId}/workflow/forms` (used by frontend
  `app.js:1984`). These endpoints exist in code but are undocumented.
- **Fix**: Add schemas and path definitions to the OpenAPI spec.

### F8. GetActiveVersion returns draft as active fallback (MEDIUM)
- **File**: `internal/forms/application/service.go:646-659`
- **Problem**: If no published version exists, `GetActiveVersion` returns the
  latest version regardless of status (could be DRAFT). Exposes unfinished
  form definitions to end users.
- **Fix**: Return `ErrFormVersionNotFound` when no published version exists;
  do not fall back to draft.

### F9. Case service audit events lack RequestID (LOW)
- **File**: `internal/forms/application/service.go` (all audit calls)
- **Problem**: Case service includes `RequestID` in audit events; form service
  does not. Makes tracing form operations in multi-request scenarios impossible.
- **Fix**: Add `RequestID: shared.StrPtr(intmid.RequestIDFromContext(ctx))`
  to form service audit event params.

### F10. Form field type immutability not documented (LOW)
- **File**: `internal/forms/application/service.go:499-525`
- **Problem**: `UpdateFieldParams` has no `Type` field; field type is immutable
  after creation even in draft status. This is likely intentional (changing
  a field type could invalidate existing submissions) but is undocumented.
- **Fix**: Document this constraint or add a `ReplaceType` method gated
  behind additional validation.

### F11. Form key validation allows invalid keys (LOW)
- **File**: `internal/forms/domain/form.go:200-202`
- **Problem**: `ValidateFormKey` has an empty `if` block for case normalization.
  The frontend E2E test (`form-management.spec.ts:118`) expects `Test-Form`
  (uppercase + hyphen) to be rejected, but the backend accepts it.
- **Fix**: Enforce lowercase alphanumeric + underscore constraint in
  `ValidateFormKey`.

### F12. GetFloat missing json.Number support in cases service (LOW)
- **File**: `internal/cases/application/service.go:1375-1388`
- **Problem**: `getFloat` does not handle `json.Number`, unlike the duplicate
  in `form_submission/application/service.go:717-732`. If JSON is decoded
  with `UseNumber()`, numeric validation fails silently.
- **Fix**: Add `json.Number` case to `getFloat`.

## Next steps for backend agent
1. **High priority**: Fix F2 (API path mismatch), F5 (RBAC), F6 (E2E wiring)
2. **High priority**: Delete F1 (dead code) and repoint tests
3. **Medium priority**: Fix F3 (type normalization), F4 (validation keys),
   F8 (draft fallback), F7 (OpenAPI spec)
4. **Low priority**: F9 (RequestID), F10 (type immutability docs),
   F11 (key validation), F12 (json.Number)
5. Rerun: `go test -short ./...`, `go test -p 1 -count=1 ./...` (with Postgres),
   `npx @redocly/cli lint api/openapi/openapi.yaml`, `npm test` (from `web/e2e`)
