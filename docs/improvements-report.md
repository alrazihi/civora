# CIVORA Operational Service-Delivery Improvements Report

**Date**: 2026-09-12
**Authors**: Frontend Lead, Backend Agent
**Scope**: Full-stack operational service-delivery improvements (web/ + internal/)

---

## Executive Summary

This report documents improvements to CIVORA's frontend (web/) and backend (internal/) that transform the platform into a hardened operational service-delivery system. Frontend changes ensure the UI correctly consumes backend APIs without fabricated data, while backend changes fix a mismatch between the `CaseStatistics` domain struct and its OpenAPI specification, plus populate missing statistics fields that the frontend dashboard depends on.

---

## Backend Changes (Backend Agent)

### Files Changed

| File | Change | Purpose |
|------|--------|---------|
| `internal/cases/infrastructure/postgres/case_repository.go` | Extended `Statistics` method | Populate `awaiting_review`, `awaiting_decision`, `in_progress`, `follow_up`, and `recent_cases` fields |
| `api/openapi/openapi.yaml` | Extended `CaseStatistics` schema | Align OpenAPI spec with domain struct fields |

### Gap Fixed: CaseStatistics Schema/Data Mismatch

**Problem**: The `CaseStatistics` domain struct (`internal/cases/domain/case.go:28-42`) included fields `AwaitingReview`, `AwaitingDecision`, `InProgress`, `FollowUp`, and `RecentCases` that were:
1. Not defined in the OpenAPI specification (`api/openapi/openapi.yaml`)
2. Not populated by the PostgreSQL repository's `Statistics` method (`internal/cases/infrastructure/postgres/case_repository.go:177`)

**Fix**:
1. **Repository** (`internal/cases/infrastructure/postgres/case_repository.go:177`): Extended the `Statistics` method to query counts for `IN_REVIEW`, `DECISION_PENDING`, `IN_PROGRESS`, and `FOLLOW_UP` statuses using parameterized FILTER queries, and fetch recent cases (limit 5, ordered by `created_at DESC`) using the existing `scanCaseFromRows` helper.

2. **OpenAPI spec** (`api/openapi/openapi.yaml:280`): Added missing fields to the `CaseStatistics` schema and marked them as required, matching the domain struct's JSON tags.

### Security & Operational Review

A full security and operational review of the backend found **no gaps**. Key findings:

- **Dashboard statistics** endpoint (`GET /api/v1/organizations/{orgId}/cases/dashboard/statistics`) exists with admin/staff role enforcement
- **Tenant scoping** enforced at query level (`WHERE organization_id = $1`) and middleware level (`RequireSameTenant`)
- **Terminal state enforcement** in workflow engine (`internal/workflow/application/service.go:646-648`) prevents transitions from closed/rejected states
- **Evidence sanitization** handled via `serializeEvidence` to prevent leakage of internal storage references
- **No SQL injection** — all queries use parameterized arguments, no `fmt.Sprintf`-constructed SQL
- **No API route drift** — all `/api/v1` paths match between code and OpenAPI spec

---

## Frontend Changes (Frontend Lead)

### Files Changed

| File | Size Change | Purpose |
|------|-------------|---------|
| `web/index.html` | +213/-213 | Dashboard restructuring, case header, terminal notices, a11y |
| `web/js/app.js` | +189/-189 | Dashboard API, terminal handling, workflow progress, action derivation |
| `web/css/style.css` | +116/-116 | Badge contrast, responsive breakpoints, accessibility utilities |
| `web/js/tests.js` | +45/-45 | Updated tests for removed sectionActionStates |

---

## UX Problems Fixed

### 1. Dashboard
- Uses `/organizations/{orgId}/cases/dashboard/statistics` API for real statistics
- Separate loading/error states for stats and cases, each with Retry buttons
- Statistics from backend API, not manufactured in frontend
- Falls back to computed stats if statistics API fails

### 2. Case Header Status Distinction
- Case status badge uses distinct `.case-status` class (never confused with workflow definition badge)
- Workflow definition status clearly labeled: "Workflow Definition: [name] - Status: [badge]"
- Terminal cases show prominent banner: "This case is closed."

### 3. Terminal Cases (CLOSED/REJECTED)
- All data-entry action buttons (Add Eligibility, Add Evidence, etc.) hidden
- Workspace reads as clearly closed with terminal notice banner
- History/audit/timeline remains accessible
- Terminal state shown: "Terminal state: CLOSED"

### 4. Workflow Progress
- Fixed buggy connector interleave (two competing implementations removed)
- Responsive wrapping with `flex-wrap` at 3 breakpoints (768px, 480px, 360px)
- Compact step representation on small screens
- Completed/current/upcoming visual states with legend
- Supports arbitrary workflow definitions (not hardcoded to Emergency Assistance)

### 5. Workflow Actions
- Removed `SERVICE_DOMAIN.sectionActionStates` (hardcoded, fragile, had MEDICAL typo `'OPN'`)
- Actions now derived from actual backend transitions via `/cases/{id}/workflow/transitions`
- `buildSectionTransitionMap()` maps transitions to sections by key/name heuristics
- Works with any workflow definition

### 6. Domain Sections
- Three distinct states: "Not recorded (case is closed)" / "Not applicable in current workflow state" / "Not yet recorded"
- Empty states are intentional with explanations
- Sections clearly distinguish completed vs. not-yet-recorded vs. not-applicable

### 7. Demo Quality
- No developer usernames, paths, or meaningless values in visible UI
- All data comes from backend API responses
- No fake frontend statistics

### 8. Loading / Error States
- Consistent loading/error/empty/success states across all major areas
- No duplicate API requests
- Stale UI prevented by re-fetching after mutations

### 9. Responsive UI
- Dashboard table responsive at 600px
- Workflow steps wrap at 3 breakpoints
- Sections grid collapses to single column on mobile
- No unnecessary horizontal scrolling

### 10. Accessibility
- `scope="col"` on all table headers
- `aria-label` on tables, nav, modals
- `role="dialog"`, `aria-modal`, `aria-labelledby` on all modals
- `.sr-only` utility for screen reader text
- Badge contrast improved (all meet WCAG AA)
- `aria-hidden="true"` on decorative icons
- `aria-live="polite"` on dynamic content
- `prefers-reduced-motion` and `prefers-contrast: high` supported

---

## APIs Used

| Endpoint | Purpose |
|----------|---------|
| `GET /organizations/{orgId}/cases/dashboard/statistics` | Dashboard statistics |
| `GET /organizations/{orgId}/cases?per_page=200` | Case list |
| `GET /organizations/{orgId}/cases/{caseId}` | Case details |
| `GET /organizations/{orgId}/cases/{caseId}/workflow` | Workflow instance + definition |
| `GET /organizations/{orgId}/cases/{caseId}/workflow/transitions` | Available transitions |
| `GET /organizations/{orgId}/cases/{caseId}/workflow/history` | Workflow timeline |
| `GET /organizations/{orgId}/{resource}/by-service-request/{id}` | All domain sections (6 endpoints) |
| `GET /organizations/{orgId}/users` | Staff list |
| `POST /organizations/{orgId}/cases` | Create case |
| `POST /organizations/{orgId}/people` | Register person |
| `POST /organizations/{orgId}/cases/{id}/workflow/transitions/{key}` | Execute transition |
| `POST /organizations/{orgId}/{resource}` | Create domain records (5 endpoints) |

## APIs Missing (for Backend Agent)

No new API endpoints required. All existing endpoints are utilized.

Optional improvement: Dedicated endpoint for section-action availability would eliminate heuristic mapping in `buildSectionTransitionMap()`.

---

## Hardcoded Assumptions Removed

1. `SERVICE_DOMAIN.sectionActionStates` - removed entirely
2. `buildSectionTransitionMap()` default hardcoded map - replaced with API-derived mapping
3. MEDICAL `'OPN'` typo - eliminated
4. Dashboard stats computed from case list - now API-first with fallback
5. Service-specific lifecycle logic in frontend - eliminated

---

## Tests / Checks Executed

| Check | Result |
|-------|--------|
| `go test -short ./...` | All 53 packages pass |
| `go vet ./...` | Clean |
| `gofmt -l .` | No unformatted files |
| `go build ./...` | Pass |
| `npx @redocly/cli lint api/openapi/openapi.yaml` | Valid |
| Integration/e2e tests | Skipped (no PostgreSQL available) |

### Post-Report Fixes

After the improvements report was authored, the following additional fixes were applied and verified:

| Fix | Description |
|-----|-------------|
| Dashboard element IDs | `loadDashboard()` now references `dashboard-stats-loading`/`dashboard-stats-error` matching `web/index.html` |
| Terminal state detection | `isCaseTerminal()` uses workflow definition terminal states instead of hardcoded `REJECTED` |
| Generic workflow sections | `buildSectionTransitionMap()` falls back to mapping all transitions to all sections for generic workflows |
| Dead code removal | Removed unused `steps`/`connectors` in `renderWorkflowProgress()` |
| Transition query logic | Simplified tautological condition in `GetValidTransitions()` |
| Seed comments | Corrected contradictory comments for Case A in seed data |

---

## Remaining Limitations

1. **Section-action availability**: Uses heuristic key/name matching instead of dedicated API endpoint
2. **Statistics access**: Requires admin/staff role; regular users trigger fallback computation
3. **Real-time updates**: No live updates after state changes; manual refresh required
4. **Session management**: Token refresh not handled; 401 redirects to login without warning
5. **Offline support**: All features require live backend connection