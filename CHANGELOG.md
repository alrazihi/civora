# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `eligibility` module: eligibility checks for service requests
- `assistance` module: assistance provisioning and tracking
- `followup` module: follow-up scheduling and management
- New OpenAPI schemas: `Person`, `Eligibility`, `Evidence`, `Assessment`, `Decision`, `Assistance`, `FollowUp`
- New API endpoints for eligibility, assistance, and follow-up resources
- Configurable rate limiter (`ServerConfig.RateLimit`, `ServerConfig.RateLimitBurst`)

### Changed

- `eligibility_repository.go`: fixed JSONB scanning for `Criteria` field using `[]byte` + `json.Unmarshal`
- `eligibility_repository.go`, `assistance_repository.go`, `followup_repository.go`: converted `SaveTx` operations to UPSERT (`INSERT ... ON CONFLICT DO UPDATE`)
- E2E tests: `registerUser` now returns created user ID; `TestServiceRequestFullLifecycle` uses valid `responsible_staff` UUID
- OpenAPI spec: extended with all new endpoint paths and schemas; corrected `Case.status` enum to match actual code values

### Fixed

- JSONB scan panic in eligibility repository
- Foreign key violation in E2E test due to nil UUID for `responsible_staff`

---

## [0.1.0] - 2026-09-06

### Added

- Initial project structure with modular monolith architecture
- Identity module: authentication, authorization, user management
- Organizations module: tenant scoping
- Audit module: immutable event log with hash chain
- Cases module: case creation, status tracking, assignment
- OpenAPI specification with all core endpoints
- CI pipeline: `gofmt`, `go vet`, `go test`, `golangci-lint`, `gosec`
- Dockerfile and docker-compose for local development
- PostgreSQL migration system with `go:embed`
- Structured JSON logging with request correlation IDs
- Security headers (HSTS, CSP, X-Frame-Options, etc.)
- Rate limiting (per-IP and per-user)
- Body size limits and request timeouts
- DB-aware health/readiness endpoints
- Comprehensive test suite: unit, integration, E2E

### Security

- RBAC enforcement on all protected routes
- Server-side role assignment (first user = admin, others = staff)
- Atomic audit events with domain writes
- Password policy enforcement (12+ chars, letter + number)
- Email validation with regex
- Correlation IDs propagated to audit events
- Per-user rate limiting on auth endpoints with lockout after 5 failures
- OIDC remnants fully removed
- No internal error leakage in 500 responses
- Cross-tenant assignment validation

[Unreleased]: https://github.com/alrazihi/civora/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/alrazihi/civora/releases/tag/v0.1.0
