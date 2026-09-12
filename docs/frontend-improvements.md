# CIVORA Frontend Improvements Report

**Date**: 2026-09-12
**Author**: Frontend Lead
**Scope**: Frontend operational service-delivery improvements (web/)

---

## Files Changed

| File | Size Change | Purpose |
|------|-------------|---------|
| `web/index.html` | +213/-213 | Dashboard restructuring, case header, terminal notices, a11y |
| `web/js/app.js` | +189/-189 | Dashboard API, terminal handling, workflow progress, action derivation |
| `web/css/style.css` | +116/-116 | Badge contrast, responsive breakpoints, accessibility utilities |
| `web/js/tests.js` | +45/-45 | Updated tests for removed sectionActionStates |

**Backend changes**: None.

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
| `go test -short ./...` | All packages pass |
| `go vet ./...` | Clean |
| `gofmt -l .` | No unformatted files |
| `node --check app.js` | Pass |
| `node --check api.js` | Pass |
| `node --check tests.js` | Pass |
| 46-point verification script | 46/46 PASS |

---

## Remaining Frontend Limitations

1. **Section-action availability**: Uses heuristic key/name matching instead of dedicated API endpoint
2. **Statistics access**: Requires admin/staff role; regular users trigger fallback computation
3. **Real-time updates**: No live updates after state changes; manual refresh required
4. **Session management**: Token refresh not handled; 401 redirects to login without warning
5. **Offline support**: All features require live backend connection
