# Security Reference

## Reporting a Vulnerability

The CIVORA team takes security vulnerabilities seriously. We appreciate
your responsible disclosure.

### How to Report

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, report using the GitHub Security Advisory "Report a vulnerability"
form on the CIVORA repository, or contact a project steward directly via
GitHub.

When reporting, please include:

- A description of the vulnerability.
- Steps to reproduce (or a proof-of-concept).
- The potential impact.
- Your contact information and preferred method of contact.
- Whether you would like to be credited (and how).

### What Happens Next

1. A steward acknowledges your report within 48 hours.
2. The team triages and confirms the vulnerability.
3. A fix is developed in a private branch.
4. A release containing the fix is coordinated.
5. The vulnerability is publicly disclosed (with credit to the reporter,
   unless they prefer anonymity).

We will keep you informed throughout the process.

---

## Supported Versions

Only the latest `main` branch and the latest release are supported for
security fixes.

| Version | Supported |
|---------|-----------|
| `main`  | Yes       |
| Latest release | Yes |
| Older releases | No  |

---

## Security Architecture

This section describes the security architecture of CIVORA 1.0.0.

### Authentication

CIVORA authenticates every request using short-lived JWT access tokens and
opaque refresh tokens.

- **Access tokens** carry claims: `sub` (user ID), `organization_id`,
  `role`, `iss`, `iat`, `exp`, `jti` (JWT ID), `kid` (key ID).
- **Access token expiry** defaults to 20 minutes.
- **Access tokens are validated against server-side session state** on every
  request, allowing revocation before expiry.
- **Refresh tokens** are 32-byte random values, hashed with SHA-256 before
  storage, and rotated on each use with a new token family.

### Session Handling

The session architecture uses a two-level repository API:

- **Pre-authentication:** `FindByRefreshTokenHash(hash)` — base lookup without
  `organization_id`. Used in the refresh-token flow where the organization is
  not yet established from the request context.
- **Authenticated context:** `FindByIDForOrganization(jti, orgID)`,
  `FindActiveByUserIDForOrganization(userID, orgID)`,
  `RevokeForOrganization(id, orgID)`, `MarkUsedForOrganization(id, orgID)`,
  and related org-aware methods. Used after JWT verification to enforce tenant
  boundaries at the data layer.

This design exists because refresh-token validation legitimately occurs before
tenant context is established. Once authenticated, every session operation is
organization-scoped.

### Refresh-Token Rotation

Each refresh issues a new refresh token and a new token family. The old
`refresh_token_hash` is overwritten in the database. Reuse of a stolen refresh
token results in `ErrSessionNotFound` and is rejected with HTTP 401.

### Refresh-Token Reuse Detection

Because the stored hash is overwritten on each rotation, presenting an old
refresh token after a successful refresh returns `ErrSessionNotFound`. This
effectively detects reuse without requiring explicit reuse tracking tables.

### JTI Validation

Each access token's `jti` claim matches an `auth_sessions.id` row. The
`AuthRequired` middleware looks up the session via `FindByIDForOrganization(jti,
orgID)` and validates:

- The session exists and is not revoked.
- The session is not expired.
- The JWT's `user_id` matches the session's `user_id`.
- The JWT's `organization_id` matches the session's `organization_id`.

The session's `last_used_at` is updated on each request.

### Signing-Key Rotation

The JWT service maintains a `KeySet` (thread-safe multi-key storage) that
supports adding and removing keys. JWTs carry a `kid` (key ID) claim. During
verification, the correct key is selected by `kid`. This enables key rotation
without invalidating existing tokens.

### Session Invalidation

- **Logout:** Calls `RevokeForOrganization(sessionID, orgID)` to revoke the
  current session.
- **Password change:** `ChangePassword` updates the password hash and calls
  `RevokeAllUserSessions(userID, orgID)`, invalidating all active sessions for
  the user within the organization.
- **Role change:** The established pattern is to update `users.role_id` and
  call `RevokeAllUserSessions(userID, orgID)`, forcing re-authentication to
  acquire the new role in the JWT.

### Authorization

CIVORA uses role-based authorization within each organization.

- **Roles:** Each organization has admins and staff. Roles are assigned at
  the organization level.
- **Route-level enforcement:** `RequireAnyRole` middleware checks the JWT
  `role` claim against allowed roles for the route.
- **Workflow authorization:** Transitions can declare `allowed_roles`. The
  workflow service checks the actor's role against this list server-side.
- **Evidence authorization:** Evidence access is scoped by organization and
  case. Cross-tenant and cross-case access is rejected.

### Tenant Isolation

All CIVORA data is scoped by `organization_id`. Tenant isolation is enforced
at three independent layers:

1. **API middleware:** `RequireSameTenant` ensures the `orgId` path parameter
   matches the JWT `organization_id` claim.
2. **Service layer:** All service methods accept and validate `organizationID`.
3. **Repository layer:** All queries filter by `organization_id`.

### Workflow Authorization

Workflow transitions can declare `allowed_roles`. The `ExecuteTransition`
method checks the actor's role against the transition's allowed roles before
executing. This is enforced server-side regardless of what the frontend
displays.

### Evidence Authorization

Evidence access is scoped by `organization_id` and `service_request_id`
(case). Repository methods filter by both. Cross-tenant evidence reads are
rejected with 403. Cross-case evidence reads are rejected with 404 or 403.

### Audit Integrity

Audit events are recorded in a dedicated `audit` schema, isolated from the
`public` schema. Each event carries a SHA-256 hash linked to the previous
event for the same organization. The chain is computed over: organization ID,
actor, action, resource type, resource ID, outcome, request ID, metadata,
timestamp, and previous hash.

Audit writes occur in the same database transaction as the state change they
record (`RecordEventInTx`). If the audit write fails, the state change is
rolled back.

### Input Validation

All input is validated and sanitized before processing:

- **Path parameters:** Validated for format and type.
- **Request bodies:** Validated against JSON schemas.
- **Query parameters:** Validated for type and range.
- **User-supplied data:** No code execution from user input, including
  workflow definitions, form definitions, and rule sets.

### Concurrency Protection

- **Workflow transitions:** Optimistic concurrency control via `version` column.
- **Evidence verification:** Row-level locking (`SELECT ... FOR UPDATE`) inside
  a transaction.
- **Decisions:** Unique constraint on `(organization_id, service_request_id, version)`
  prevents duplicate versions.
- **Form fields:** Unique constraint per form version prevents duplicate keys.
- **Idempotency:** The idempotency middleware prevents duplicate resource
  creation from retried requests.

### Sensitive Logging Restrictions

- Request/response bodies are never logged.
- Export endpoints apply field-name-based sanitization. Fields matching
  patterns like `password`, `secret`, `token`, `api_key`, `credential`, `ssn`,
  `session_id`, `jti`, `jwt`, `authorization`, `cookie`, `csrf` are replaced
  with `"[REDACTED]"`.
- AI document processing applies PII redaction (SSN, credit card numbers,
  email, phone) before forwarding to external LLM providers.
- Secrets, tokens, and passwords are never included in structured log entries.

---

## Verified Security Properties in 1.0.0

The following security properties are verified by tests or implementation
evidence in the 1.0.0 release:

| Property | Verification |
|----------|-------------|
| Authentication on every request | AuthRequired middleware + 401 tests |
| JWT signature and expiry validation | Unit + integration tests |
| Session binding via JTI | `FindByIDForOrganization(jti, orgID)` in middleware |
| Server-side session revocation | `RevokeForOrganization`, `RevokeAllUserSessions` |
| Refresh token rotation | New hash + new family on each refresh |
| Refresh token reuse detection | Old hash overwritten; reuse returns 401 |
| Concurrent refresh safety | Exactly 1 success, 1 failure in race |
| Password-change invalidation | All sessions revoked on `ChangePassword` |
| Role-change invalidation | Pattern: update role + `RevokeAllUserSessions` |
| JTI/session binding | JWT `jti` matched to `auth_sessions.id` |
| Signing-key rotation support | `KeySet` with `kid` claim in JWT |
| Cross-tenant API rejection | `RequireSameTenant` middleware |
| Cross-tenant data rejection | Organization-scoped repository queries |
| Cross-tenant session rejection | Org-aware session lookup |
| Evidence storage reference protection | Excluded from API responses |
| Evidence concurrent verification safety | Row-level locking (`FOR UPDATE`) |
| Workflow concurrent transition safety | Optimistic concurrency (version column) |
| Decision supersession integrity | Version increment + `SupersededByID` |
| Audit hash-chain integrity | `VerifyChain` + background maintenance |
| Transactional audit recording | `RecordEventInTx` — atomic with state change |
| Timing-safe auth paths | Constant-time comparison for secrets |
| Generic rate-limit responses | No information leakage in 429 responses |
| Evidence state-machine enforcement | Server-side transition validation |
| AI architectural isolation | No code path from AI outputs to consequential functions |
| PII sanitization for AI | Configurable PII redactor before external LLM calls |

---

## Security Boundaries and Limitations

### Application / Database Boundary

The audit hash chain is tamper-evident, not tamper-proof against a privileged
insider or database administrator with direct database access. A compromised
application process or DBA can write a valid chain with false contents.

### Audit Chain

The hash chain detects accidental or offline tampering within the
application/database boundary. It is not an externally anchored immutable
ledger. External append-only anchoring is a post-1.0 enhancement.

### Encryption at Rest

Encryption at rest is not currently provided by CIVORA. PostgreSQL data is
stored unencrypted by default. Operators should configure PostgreSQL-level
encryption (e.g., tablespace encryption) or filesystem/disk-level encryption
if required.

### Backup and Disaster Recovery

CIVORA does not include built-in backup or disaster recovery functionality.
Operators are responsible for configuring PostgreSQL backups, testing restore
procedures, and defining RTO/RPO objectives.

### Compliance Frameworks

CIVORA does not implement SOC 2 Trust Services Criteria or HIPAA technical
safeguards. Compliance with these frameworks depends on the operator's
deployment configuration, policies, and controls.

---

## Operator Responsibilities

Operators must provide:

- TLS termination (reverse proxy or direct cert/key files).
- Database backups, restore testing, and disaster-recovery planning.
- Secrets management and rotation for database credentials, JWT signing keys,
  and AI provider keys.
- Network-level DDoS protection, firewall rules, and host hardening.
- Log aggregation, monitoring, and alerting.
- Compliance procedures required by the operator's regulatory environment
  (e.g., GDPR, HIPAA, SOC 2).

---

## Transport Security

CIVORA must not be exposed directly to the Internet over plain HTTP. Operators
must choose one of the following deployment models:

1. **Reverse-proxy TLS termination** — Place CIVORA behind Caddy, nginx, or
   another reverse proxy that terminates TLS. Configure
   `CIVORA_SERVER_TRUSTED_PROXIES` to the proxy's IP/CIDR and set
   `CIVORA_SERVER_FORCE_HTTPS=true` so HSTS is emitted.
2. **Direct TLS termination** — Provide certificate and key files via
   `CIVORA_SERVER_TLS_CERT` and `CIVORA_SERVER_TLS_KEY`. CIVORA will listen on
   HTTPS and emit HSTS automatically.

Exposing CIVORA on plain HTTP to the Internet is a security vulnerability.

---

## Disclosure Policy

- We ask that you give us a reasonable amount of time to fix the issue
  before public disclosure.
- We will not initiate legal action against anyone who reports a
  vulnerability in good faith.
- If you make a good-faith attempt to avoid privacy breaches and system
  disruption, you will not face any penalties.

## Bug Bounty

CIVORA does not currently offer a paid bug bounty program. We may introduce
one in the future if the project grows sufficiently.
