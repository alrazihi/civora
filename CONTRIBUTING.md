# Contributing to CIVORA

Thank you for your interest in contributing to CIVORA. This document
describes how to get involved and the expectations for contributions.

---

## Table of contents

1. [How to contribute](#how-to-contribute)
2. [Getting started](#getting-started)
3. [Development workflow](#development-workflow)
4. [Coding standards](#coding-standards)
5. [Testing](#testing)
6. [Documentation](#documentation)
7. [Architecture decisions](#architecture-decisions)
8. [Issue triage](#issue-triage)
9. [Community](#community)
10. [Reporting security vulnerabilities](#reporting-security-vulnerabilities)

---

## How to contribute

You can contribute in many ways:

- **Report bugs** by opening an issue.
- **Propose features** by opening a discussion or issue.
- **Improve documentation**.
- **Submit code** via pull requests.
- **Review pull requests** (once you have contributor status).
- **Translate** the UI and documentation (when i18n infrastructure exists).
- **Test** pre-release builds and report issues.

All contributions are welcome, large or small. See
[GOVERNANCE.md](GOVERNANCE.md) for how decisions are made.

---

## Getting started

1. Fork the repository on GitHub.
2. Clone your fork locally:

    ```bash
    git clone https://github.com/<your-username>/civora.git
    cd civora
    ```

3. (Optional) Add the upstream repository as a remote:

    ```bash
    git remote add upstream https://github.com/alrazihi/civora.git
    ```

4. Build and run the project (see [AGENTS.md](AGENTS.md)).

---

## Development workflow

1. Create a branch for your work:

    ```bash
    git checkout -b feature/my-contribution
    ```

    Use a clear, descriptive branch name. Prefix with `feature/`, `fix/`,
    `docs/`, `refactor/`, or `test/`.

2. Make your changes.

3. Write or update tests as needed.

4. Ensure linting and tests pass (see [AGENTS.md](AGENTS.md)):

    ```bash
    go build ./...
    gofmt -l .
    go vet ./...
    go test -short ./...
    go test -p 1 -count=1 ./test/integration/...
    ```

5. Commit your changes with a clear commit message following
    [Conventional Commits](https://www.conventionalcommits.org/) format:

    ```
    feat: add user profile endpoint

    Add GET /api/v1/users/me to retrieve the authenticated user's
    profile information.

    Fixes #123
    ```

6. Push to your fork:

    ```bash
    git push origin feature/my-contribution
    ```

7. Open a pull request against the `main` branch of the upstream
    repository.

8. A maintainer will review your PR. Be prepared to iterate on feedback.

---

## Coding standards

- Follow the style conventions of the primary implementation language.
- Write clear, self-documenting code; minimize comments but use them
  to explain *why*, not *what*.
- Avoid duplicating existing functionality; refactor or propose
  improvements first.
- Keep functions small and focused.
- Write tests for new functionality.
- Do not commit generated files unless explicitly required.

---

## Security-sensitive contributions

Contributions that touch authentication, authorization, session handling,
tenant isolation, evidence management, audit recording, or input validation
carry additional responsibilities:

- **Preserve tenant isolation.** Do not introduce code paths that bypass
  `organization_id` scoping in repositories or services.
- **Preserve authorization boundaries.** Do not weaken role checks or add
  unauthenticated routes.
- **Add regression tests for security fixes.** Every security fix must include
  a test that demonstrates the vulnerability and verifies the fix.
- **Test concurrency-sensitive changes.** Changes to workflow transitions,
  evidence verification, decisions, or session operations must include
  concurrency tests.
- **Avoid bypassing domain/service boundaries.** Cross-module calls must go
  through service interfaces. Do not access another module's database tables
  directly.
- **Document security implications.** If a change affects the threat model,
  update `docs/threat-model.md` and `SECURITY.md`.

### Development philosophy

Build → Test → Attack → Harden → Document → Prove

1. **Build** — implement the feature.
2. **Test** — write unit, integration, and E2E tests.
3. **Attack** — attempt to break your own change (cross-tenant access,
   authorization bypass, concurrency races, input validation bypass).
4. **Harden** — fix the weaknesses you found.
5. **Document** — update architecture docs, OpenAPI, and CHANGELOG.
6. **Prove** — run the full verification suite and report results.

---

## Testing

- Unit tests must accompany new logic.
- Integration tests must cover cross-module interactions.
- End-to-end tests must cover API-level workflows.
- Test coverage should not drop below 80% for new code.
- Security-sensitive changes require concurrency and cross-tenant tests.
- See [AGENTS.md](AGENTS.md) for test commands.

### Test categories

| Category | Command | When to run |
|----------|---------|-------------|
| Unit + domain | `go test -short ./...` | Always, before committing |
| Integration | `go test -p 1 -count=1 ./test/integration/...` | Always, before opening PR |
| All Go tests | `go test -p 1 -count=1 ./...` | Before merging |
| Playwright E2E | `cd web/e2e && npm test` | When frontend changes |
| OpenAPI lint | `npx @redocly/cli lint api/openapi/openapi.yaml` | When API changes |
| Vet | `go vet ./...` | Always, before committing |
| Format | `gofmt -l .` | Always, before committing |
| Build | `go build ./...` | Always, before committing |

---

## Documentation

- Update relevant documentation when you change behavior.
- API changes must be reflected in the OpenAPI specification.
- Architecture decisions must be recorded as ADRs (see below).
- User-facing changes must be noted in the CHANGELOG.
- Security implications must be documented in `SECURITY.md` and
  `docs/threat-model.md` if they affect the threat model.

---

## Architecture decisions

All substantial technical decisions are recorded as **Architecture
Decision Records (ADRs)** in `docs/decisions/`.

To propose a decision:

1. Create a new file: `docs/decisions/NNNN-short-title.md` (where
   `NNNN` is the next number).
2. Use the [ADR template](docs/decisions/0000-template.md). See
   [AGENTS.md](AGENTS.md) for project conventions.
3. Include: context, decision, alternatives considered, trade-offs,
   assumptions, how to reverse.
4. Submit as a pull request for review.

Major decisions (anything affecting module boundaries, data model,
deployment, or API surface) require ADR review before implementation.

---

## Issue triage

- Issues are labeled by type: `bug`, `feature`, `documentation`,
  `question`, `security`, `wontfix`, `duplicate`.
- Issues are labeled by priority: `critical`, `high`, `medium`, `low`.
- Maintainers triage issues regularly.
- Contributors are encouraged to claim issues before starting work
  to avoid duplication.

---

## Community

- Be respectful and constructive in all interactions.
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md).
- Use GitHub Discussions for conversations that are not bug reports
  or feature proposals.
- All work happens in the open on GitHub.

---

## Reporting security vulnerabilities

See [SECURITY.md](SECURITY.md) for instructions on reporting security
vulnerabilities. Do not open a public issue for security problems.
