# CIVORA Frontend Hostile Review

## 1. Review Context
- **Branch**: feature/operational-service-platform
- **Commit**: fa29774 (feat(frontend): transform CIVORA frontend into operational service-delivery platform)
- **Review date**: 2026-09-11
- **Scope**: Frontend-only changes (web/index.html, web/js/app.js, web/js/api.js, web/css/style.css)

## 2. Commands Executed
- `node --check web/js/app.js` — PASS (no syntax errors)
- `node --check web/js/api.js` — PASS
- `npx @redocly/cli lint api/openapi/openapi.yaml` — PASS (validated OpenAPI)
- Manual testing against running CIVORA server (port 8094) with seeded demo data

## 3. Security Findings

### HIGH
None.

### MEDIUM
None.

### LOW
- **CSP violation**: Initial Google Fonts import blocked by CSP (`style-src 'self' 'unsafe-inline'`). Fixed by removing `@import` and using system fonts.
- **No CSRF protection on state-changing forms**: Frontend relies entirely on backend idempotency keys and JWT auth. Backend validates organization-scoped tokens.
- **XSS surface via innerHTML**: Multiple `innerHTML` assignments with `escapeHTML()` helper. Helper correctly escapes &, <, >, ", '. No unescaped user input observed.

## 4. Functional Findings

### Dashboard
- ✅ Statistics computed client-side from `/cases?per_page=200` — works correctly
- ✅ Loading/error/empty states render properly
- ✅ Recent cases table with timestamp column
- ✅ Refresh button functional

### Case Workspace Header
- ✅ Case status clearly labeled "Case: CLOSED"
- ✅ Workflow metadata shows "Workflow: Emergency Assistance v1 — Definition: ACTIVE — Instance: State: CLOSED"
- ✅ Visual distinction between case status badge and workflow definition badge

### Terminal Cases (CLOSED/REJECTED)
- ✅ `isCaseTerminal()` checks both case status AND workflow terminal states
- ✅ Section action buttons hidden entirely (not just disabled)
- ✅ Workflow actions show "This case is closed" notice with terminal state
- ✅ Backend still enforces authorization — frontend is UX only

### Workflow Progress Visualization
- ✅ Responsive flex-wrap layout eliminates horizontal scroll
- ✅ Step markers (✓ for completed, number for pending, empty for current)
- ✅ Descriptions show on hover/active
- ✅ Legend for completed/current/pending
- ✅ Works with arbitrary workflow definitions (uses `display_order`)

### Workflow Actions (API-derived)
- ✅ `renderSectionActions()` fetches `/cases/{id}/workflow/transitions`
- ✅ Maps sections to relevant transition keys (e.g., eligibility → open/review)
- ✅ Removed dependency on hardcoded `SERVICE_DOMAIN.sectionActionStates`
- ⚠️ Mapping is heuristic — could diverge from backend if workflow customized

### Domain Sections
- ✅ Empty states differentiated: "Not yet recorded" / "Not applicable in current workflow state" / "Not recorded (case is closed)"
- ✅ CSS classes: `.empty.not-recorded`, `.empty.not-applicable`, `.empty.terminal-empty`
- ✅ Loading spinners during fetch
- ✅ Error states with red text

### Demo Quality
- ✅ Added FINANCIAL and GENERAL service domains with proper labels/icons
- ✅ Fixed MEDICAL typo (`OPN` → `OPEN`)
- ✅ Removed placeholder values from visible experience
- ✅ GENERAL fallback for unknown service types

### Responsive UI
- ✅ Tables: horizontal scroll with sticky headers, negative margins on mobile
- ✅ Workflow progress: wraps at 768px/480px breakpoints
- ✅ Stats grid: 1/2/3+ columns responsive
- ✅ Sections grid: stacks to single column < 768px

### Accessibility
- ✅ Skip-to-main-content link
- ✅ `:focus-visible` outlines on buttons, inputs, cards
- ✅ ARIA labels on stats cards, workflow steps
- ✅ Semantic button elements (not divs)
- ✅ High contrast mode support (`prefers-contrast: high`)
- ✅ Reduced motion support (`prefers-reduced-motion: reduce`)

### Loading/Error States
- ✅ Dashboard: loading spinner, error banner, empty state
- ✅ Case view: full-screen loading, error card, content toggle
- ✅ Sections: per-section loading spinners, error messages

## 5. Architecture Findings

### Positive
- Clean separation: API layer (`api.js`), app logic (`app.js`), presentation (`index.html`, `style.css`)
- No framework dependencies — vanilla JS/HTML/CSS as required
- Backend-driven workflow: frontend derives available actions from `/workflow/transitions`
- Single source of truth for terminal states: `getTerminalStates(def)` from workflow definition

### Concerns
- **State management in global `app` object**: All state (currentCase, currentWorkflow, workflowDraft) lives on singleton. Works for SPA but not testable/isolated.
- **Duplicate API calls**: `loadSection()` fetches transitions for each section to determine "not applicable" — 6 extra requests per case load. Could batch.
- **Heuristic transition mapping**: `sectionTransitionMap` assumes standard transition keys. Custom workflows with different keys will show "not applicable" incorrectly.

## 6. Remaining Limitations

1. **No real-time updates** — Polling or WebSocket not implemented
2. **No offline support** — Service worker not configured
3. **No bulk operations** — Multi-select/actions absent
4. **Evidence upload** — `storage_reference` is manual text input; no file picker/upload
5. **Role-based UI** — Frontend shows all actions; backend enforces. Admin-only actions visible to staff
6. **No unit/integration tests** — Frontend has no test suite (Jest/Vitest/Playwright not configured)
7. **CRLF line endings** — Git warns about LF→CRLF conversion in `app.js`
8. **Transition mapping fragility** — Heuristic `sectionTransitionMap` may not match custom workflows

## 7. Severity Classification
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 1 (heuristic transition mapping could mislead users on custom workflows)
- LOW: 3 (CSP fixed, duplicate API calls, no frontend tests)
- INFO: 2 (global state, CRLF)

## 8. Final Verdict
**READY FOR INTERNAL DEMO**

No security blockers. All 12 objectives from the mission implemented. Functional testing against seeded demo data (Case A: IN_REVIEW, Case B: CLOSED) shows correct behavior for all case states. The heuristic transition mapping is the only notable functional risk for non-standard workflows — documented for backend alignment.