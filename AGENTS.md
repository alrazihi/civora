# AGENTS.md

This file documents commands for working with the CIVORA codebase.

## Build

```bash
go build ./...
```

## Formatting

```bash
gofmt -l .                    # check for unformatted files
gofmt -w .                    # format all files
```

## Lint / Vet

```bash
go vet ./...
```

## Test

```bash
go test -short ./...                     # unit + domain tests only
go test -p 1 -count=1 ./...               # all tests (serial, for DB integration)
go test -short -c ./test/e2e/            # compile e2e tests without running
```

> **Note:** The Go race detector (`-race`) may fail on Windows due to
> memory limits. If you encounter `fatal error: runtime: out of memory`
> or `The paging file is too small`, either increase the Windows paging
> file size or omit `-race` on that platform.

Integration and e2e tests require a PostgreSQL instance with a `civora_test` user
and database. They are skipped automatically when `-short` is passed.

### Frontend E2E tests (Playwright)

The web frontend (`web/`) ships a Playwright suite under `web/e2e/`. It serves
`web/` as a static site (no Go backend or PostgreSQL required) and mocks the
API via request interception, so it runs fully offline.

```bash
cd web/e2e

# Install deps once (also installs the chromium browser)
npm install
npx playwright install --with-deps chromium

# Run the suite (default: 2 workers locally; 1 worker in CI)
npm test
# or: npx playwright test

# Interactively step through tests
npm run test:ui
```

The static server is started/stopped automatically by `globalSetup`/`globalTeardown`;
`npm test` (alias) takes care of this. Tests live in `web/e2e/tests/` and are
written in TypeScript. The test suite is split into category-specific spec files:

- `cases.spec.ts` — New case workflow selection
- `dashboard.spec.ts` — Dashboard rendering
- `forms.spec.ts` — Dynamic form renderer
- `workspace.spec.ts` — Case workspace integration
- `form-management.spec.ts` — Form management CRUD
- `helpers.ts` — Shared fixtures and utility functions

### Running tests by category

Run only a specific category instead of the full suite (faster iteration):

```bash
cd web/e2e

# Run a single category
npx playwright test cases
npx playwright test dashboard
npx playwright test forms
npx playwright test workspace
npx playwright test form-management

# Or combine multiple categories
npx playwright test cases dashboard

# Run the full suite (all categories)
npm test
```

For local development and debugging, agents should target the specific category
relevant to the changes rather than running the full scan. Only run the full
suite (`npm test`) when explicitly asked or when changes may affect multiple
areas.


To start the database locally with Docker Compose:

```bash
docker compose up -d db
```

Environment variables for the test database:

| Variable | Default |
|---|---|
| `CIVORA_TEST_DB_HOST` | `localhost` |
| `CIVORA_TEST_DB_PORT` | `5432` |
| `CIVORA_TEST_DB_USER` | `civora_test` |
| `CIVORA_TEST_DB_PASSWORD` | `civora_test` |
| `CIVORA_TEST_DB_NAME` | `civora_test` |
| `CIVORA_TEST_DB_SSLMODE` | `disable` |

## OpenAPI Validation

```bash
# Validate the OpenAPI specification
npx @redocly/cli lint api/openapi/openapi.yaml
```

## Docker

```bash
docker build -t civora .
docker compose up -d
```

## Local Development

```bash
# Start the database
docker compose up -d db

# Run the server
go run ./cmd/civora

# Run all tests (serial)
go test -p 1 -count=1 ./...

# Run only unit tests (fast, no database)
go test -short ./...
```

## Global Agent Instructions

For large tasks, agents should follow this workflow:

1. **Create a feature branch** - Before starting significant work, create a new feature branch from main:
   ```bash
   git checkout -b feature/<descriptive-name>
   ```

2. **Implement the feature** - Work on the task in the feature branch

3. **Double-check work** - Before merging:
    - Run all tests: `go test -p 1 -count=1 ./...`
    - Run linter: `go vet ./...`
    - Check formatting: `gofmt -l .`
    - Verify the build: `go build ./...`

4. **Merge when confident** - Only merge to main when all checks pass:
    ```bash
    git checkout main
    git merge feature/<descriptive-name>
    git branch -d feature/<descriptive-name>
    ```

5. **Self-score the task** - After completion, provide a self-assessment score (1-10) based on:
    - Code quality and adherence to project standards
    - Test coverage and correctness
    - Completeness of the implementation
    - Documentation updates if needed

## Hard Rules — Do Not Violate

The following rules are mandatory for all agents working on CIVORA.
Violating any of these rules may introduce security vulnerabilities,
data-leakage paths, or architectural regressions.

### Tenant Isolation

- Never introduce code paths that bypass `organization_id` scoping in
  repositories or services.
- Never weaken cross-tenant access checks. All data-access methods must
  filter by `organization_id`.
- Never expose one tenant's data to another tenant, even in error messages
  or logs.

### Session Architecture (Stage 6)

- `FindByRefreshTokenHash(...)` does not require `organization_id` because
  it is used in the pre-authentication refresh-token flow. Do not add an
  `organization_id` parameter to this method.
- All authenticated session operations must use org-aware methods
  (`FindByIDForOrganization`, `RevokeForOrganization`, `MarkUsedForOrganization`,
  etc.). Do not use base methods in authenticated contexts.
- Do not bypass JTI/session binding. The `AuthRequired` middleware must
  continue to validate `jti` against `auth_sessions.id` with org scoping.

### Testing

- Do not weaken, skip, or mark-as-expected any existing test without
  explicit evidence that the test is incorrect.
- Do not hide test failures. If a test fails, report it with the full
  error output and the command that was run.
- Security-sensitive changes require concurrency tests and cross-tenant
  tests. Do not merge security-related changes without them.
- When running tests, report actual results — do not fabricate pass/fail
  counts.

### Security Architecture

- Do not introduce Docker-only assumptions or infrastructure dependencies
  when the supported environment does not require Docker. CIVORA runs as
  a single static Go binary against PostgreSQL.
- Do not invent new infrastructure dependencies (message queues, caches,
  external services) without an ADR and implementation evidence.
- Do not change the audit recording path. Audit events must continue to be
  written in the same transaction as the state change they record.
- Do not make AI outputs capable of triggering consequential actions
  (decisions, workflow transitions, status changes). The AI module must
  remain architecturally isolated from consequential functions.

### Migrations

- Do not modify existing migrations. Add new migrations for schema changes.
- Do not write destructive migrations (DROP TABLE, DROP COLUMN) without
  explicit review and a rollback path.
- Migrations must be reversible. Every `up` migration must have a
  corresponding `down` migration.

### Evidence and Concurrency

- Do not remove row-level locking (`FOR UPDATE`) from evidence verification
  updates.
- Do not make published forms or rule sets mutable. Published versions must
  remain immutable.
- Do not expose `storage_reference` or `StorageKey` in API responses.

### Documentation and Verification

- When claiming a test passes, run the test and report the actual output.
- Do not declare environmental failures (e.g., "Windows OOM") without
  evidence from the actual command output.
- Do not modify production code or tests when the task is documentation-only.
- When in doubt, read the existing code before changing it. Do not assume
  behavior from documentation alone.
