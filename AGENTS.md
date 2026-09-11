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

Integration and e2e tests require a PostgreSQL instance with a `civora_test` user
and database. They are skipped automatically when `-short` is passed.

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
