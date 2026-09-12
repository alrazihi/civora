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
- `node --check web/js/app.js web/js/tests.js` (pass)
- OpenAPI lint not run (no spec changes in this task).

## Next steps for backend agent
1. Implement gap #1 (CreateCase `workflow_definition_id`) — highest priority;
   unblocks arbitrary workflow selection.
2. Optional: gaps #2–#5 for richer UX and cleaner API surface.
3. After backend lands #1, update frontend `createCase` to send
   `workflow_definition_id` directly instead of deriving `service_type`.
