# Agent Test Assignments — Frontend Generic Workflow Cases

Source: `.kilo/GAPS-front-generic-workflow-cases.md`

Each agent must read this document and `.kilo/GAPS-front-generic-workflow-cases.md` before running any tests.

---

## Agent: Maya Chen — Playwright E2E Suite

**Scope**: `web/e2e/` — Playwright tests only
**Command**: `cd web/e2e && npm test`
**Pre-read**: `web/e2e/tests/helpers.ts` (understand mocked routes and shared utilities), `web/e2e/tests/cases.spec.ts` (New case workflow selection tests)

Test groups to execute:
1. **New case workflow selection** (3 tests): workflow selector rendering, case creation navigation, custom workflow support — run with `npx playwright test cases`
2. **Dashboard** (2 tests): terminal-state status breakdown, recent cases listing — run with `npx playwright test dashboard`
3. **Dynamic Form Renderer** (6 tests): form rendering, invalid submission blocking, supported field types, 404 handling, terminal state hiding, server validation errors, successful submission — run with `npx playwright test forms`

Agents must target the specific category relevant to their changes rather than running the full scan. Only run the full suite (`npm test`) when explicitly asked or when changes may affect multiple areas.

Do NOT run Go tests or backend tests. Only Playwright E2E.

---

## Agent: Rafael Ortiz — Frontend JS Unit & Syntax Checks

**Scope**: `web/js/` — frontend JavaScript tests and syntax validation
**Commands**:
- `node --check web/js/app.js`
- `node --check web/js/tests.js`
- Inspect `web/js/tests.js` for any runnable test cases and execute them

Pre-read: `web/js/app.js`, `web/js/tests.js`

Focus areas from the gap report:
- Workflow selector logic
- `computeTerminalStateSet` / `isCaseTerminal`
- `createCase` with workflow selection
- Dashboard terminal-state usage
- `renderWorkflowSelector` bug fix (preselectId parameter)
- `escapeHTML` XSS fix

Do NOT run Playwright or Go tests.

---

## Agent: Priya Nair — Backend Cases API Tests (Gaps #1 & #3)

**Scope**: `internal/cases/...` — Go tests for cases creation and transitions
**Commands**:
- `go test -short -count=1 ./internal/cases/...`
- Focus especially on: `internal/cases/api/handler_test.go`, `internal/cases/application/service_test.go`, `internal/cases/domain/` tests

Pre-read: `.kilo/GAPS-front-generic-workflow-cases.md` (gaps #1 and #3), relevant source files

Gap #1 — CreateCase accepts `workflow_definition_id`:
- Tests should verify that POST /organizations/{orgId}/cases accepts `workflow_definition_id` in the request body
- Validate the definition belongs to the org and is ACTIVE
- When `workflow_definition_id` is provided, skip key/service_type fallback

Gap #3 — Transition payload includes metadata:
- Tests should verify transition responses include `allowed_roles`, `conditions`, `description`

Do NOT run Playwright tests or web/e2e tests.

---

## Agent: Jonas Berg — Backend Workflow Tests (Gaps #2 & #5)

**Scope**: `internal/workflow/...` — Go tests for workflow list and history
**Commands**:
- `go test -short -count=1 ./internal/workflow/...`
- Focus especially on: `internal/workflow/api/handler_test.go`, `internal/workflow/application/`, `internal/workflow/domain/` tests

Pre-read: `.kilo/GAPS-front-generic-workflow-cases.md` (gaps #2 and #5), relevant source files

Gap #2 — Workflows list endpoint supports `?status=` filter:
- Tests should verify `GET /organizations/{orgId}/workflows?status=active` returns only active definitions

Gap #5 — Workflow history pagination:
- Tests should verify `GET /organizations/{orgId}/cases/{caseId}/workflow/history` supports `limit`/`offset` and returns `total` in meta

Do NOT run Playwright tests or web/e2e tests.

---

## Agent: Aiko Tanaka — Build & Static Verification

**Scope**: Full project build, lint, and static checks
**Commands**:
- `go build ./...`
- `go vet ./...`
- `gofmt -l .` (check only, expect empty output)
- `node --check web/js/app.js web/js/tests.js`

Pre-read: `.kilo/GAPS-front-generic-workflow-cases.md` (Verification done section)

Also verify:
- No Go test compilation errors: `go test -short -count=1 -run=NONE ./...` (compiles all tests without running)
- OpenAPI lint: `npx @redocly/cli lint api/openapi/openapi.yaml` (from project root)

Do NOT run Playwright or Go test execution — only build/vet/format/static checks.
