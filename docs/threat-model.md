# CIVORA Threat Model

> **Status**: Draft — covers the foundation phase (milestone 0.1–0.3).
> This is a living document. New threats must be added as the system
> evolves.

---

## 1. Document conventions

### 1.1 Trust boundaries

A **trust boundary** is a point where data crosses from one trust domain
to another. CIVORA's primary trust boundaries are:

- **Internet → Application**: external clients (browsers, mobile apps,
  API consumers) connecting to the CIVORA HTTP API.
- **Application → Database**: CIVORA writing to and reading from its
  database(s).
- **Application → Object storage**: CIVORA writing uploaded documents,
  evidence, and export archives to object storage.
- **Application → External services**: CIVORA calling identity providers,
  document verification services, payment gateways, etc.
- **Application → File system**: CIVORA writing local log files and
  cache.

### 1.2 Trust levels

- **Untrusted**: external clients, the internet, uploaded files, external
  API responses.
- **Trusted (internal)**: the CIVORA application process and its direct
  runtime environment.
- **Trusted (operator)**: infrastructure controlled by the deploying
  institution (database, storage, network).
- **Highly privileged**: CIVORA administrators, system operators.

---

## 2. Assets

| Asset | Description | Sensitivity |
|-------|-------------|-------------|
| User credentials / tokens | Passwords, session tokens, API keys, OAuth tokens. | Critical |
| Personal data | Names, addresses, demographic data, submitted evidence. | High |
| Workflow definitions | Process models, rules, policies. | Medium |
| Audit log | Immutable record of all actions. | Critical |
| Case data | Individual service requests, decisions, outcomes. | High |
| AI prompts/recommendations | When AI is introduced (milestone 0.7+). | Medium |
| Configuration | Tenant settings, credentials, encryption keys. | Critical |

---

## 3. Threat inventory

### T-01: Unauthorized access to protected resources

**Description**: An attacker gains access to data or functionality
belonging to an organization they are not authorized for.
**Affected assets**: All organization-scoped data.
**Mitigation strategies**:
- Authentication on every request.
- Organization-level (tenant) scoping on all queries.
- Role-based authorization checks at the API and business-logic layers.
**Severity**: High

### T-02: Privilege escalation

**Description**: An authenticated user obtains permissions beyond their
assigned role (e.g., a staff member performs admin actions).
**Affected assets**: Configuration, audit log, all tenant data.
**Mitigation strategies**:
- Server-side authorization checks on every action.
- No client-side trust in role claims.
- Administrative actions require re-authentication or step-up auth.
- Audit of all privilege-escalating actions.
**Severity**: High

### T-03: Tenant isolation failure

**Description**: A user or process from one organization accesses data
belonging to another (cross-tenant data leak).
**Affected assets**: All multi-tenant data.
**Mitigation strategies**:
- All database queries are scoped by organization ID.
- Query-layer enforcement (not just application-layer).
- Integration tests verify tenant isolation.
- Database schema uses foreign keys with organization scoping where
  applicable.
**Severity**: Critical

### T-04: Credential theft

**Description**: An attacker obtains user passwords, session tokens, API
keys, or service-account credentials.
**Affected assets**: User credentials, all data accessible to compromised
accounts.
**Mitigation strategies**:
- Passwords stored using a strong, slow hash (Argon2id or bcrypt with
  high work factor).
- Session tokens are random, short-lived, and revocable.
- API keys have scoped permissions and expiration.
- Secrets are never logged or stored in plaintext.
- Rate limiting on authentication endpoints.
**Severity**: High

### T-05: Injection attacks

**Description**: An attacker injects SQL, command, LDAP, or expression
payloads through form inputs, API parameters, or workflow definitions.
**Affected assets**: Database, operating system, workflow execution.
**Mitigation strategies**:
- Parameterized queries for all database access.
- No shell execution from user input.
- Input validation and sanitization at all boundaries.
- Rule/policy expressions evaluated in a sandboxed, resource-limited
  environment.
- Workflow definition evaluation is separated from code execution.
**Severity**: High

### T-06: Malicious document upload

**Description**: An attacker uploads a file that exploits a vulnerability
in document processing (e.g., PDF exploits, macro-enabled documents,
malformed images, zip bombs).
**Affected assets**: Application runtime, operator infrastructure.
**Mitigation strategies**:
- Upload size limits enforced.
- Content-type validation (declared and sniffed).
- File extension allow-lists.
- Anti-malware scanning at ingestion (where available).
- Document processing in a sandboxed environment with resource limits.
- No automatic execution of uploaded content.
- Stored files served with `Content-Disposition: attachment` and
  `X-Content-Type-Options: nosniff`.
**Severity**: Medium

### T-07: Compromised integrations

**Description**: An external service that CIVORA integrates with is
compromised, leading to data exfiltration or injection of malicious
data.
**Affected assets**: Data shared with integrations, workflow execution.
**Mitigation strategies**:
- Integrations use OAuth or signed request authentication.
- Webhook endpoints validate signatures.
- Data from external services is treated as untrusted and validated.
- Integration credentials are scoped to least privilege.
- Integration failures are isolated and logged.
**Severity**: Medium

### T-08: Data leakage

**Description**: Sensitive data is exposed through misconfiguration,
insecure logging, error messages, or exports.
**Affected assets**: All data.
**Mitigation strategies**:
- No sensitive data in logs (PII, credentials, tokens).
- Error messages are generic; details go to secure logs.
- Export endpoints enforce authorization. (Future)
- TLS in transit; encryption at rest is planned for a future milestone.
- Data classification labels guide handling.
**Severity**: High

### T-09: Audit tampering

**Description**: An attacker modifies or deletes audit records to hide
their actions.
**Affected assets**: Audit log.
**Mitigation strategies**:
- Audit records are append-only.
- Audit log is stored separately from operational data.
- Audit records are signed or hashed (tamper-evident chain) where
  feasible.
- Administrative access to audit log is restricted and itself audited.
- Audit log deletion is not supported (only retention-based archival).
**Severity**: Critical

### T-10: Injection through workflow definitions

**Description**: A workflow definition (rules, expressions, templates)
contains malicious logic that is executed by the engine, potentially
accessing data or executing code.
**Affected assets**: All data accessible to the workflow engine.
**Mitigation strategies**:
- Rule expressions are evaluated in a sandboxed evaluator (not a general
  expression engine or code evaluator).
- Workflow definitions are validated at creation time.
- The engine does not execute arbitrary code from definitions.
- Template rendering is sandboxed (no server-side code execution).
- Review/auditing of workflow definitions is supported.
**Severity**: High

### T-11: AI prompt injection

**Description**: (Applies only when AI is introduced in milestone 0.7+)
An attacker injects malicious content into input data that is fed to an
AI model, causing the model to produce harmful output or ignore
instructions. Also includes prompt leakage where model prompts are
extracted via adversarial queries.
**Affected assets**: AI recommendations, data processed by AI.
**Mitigation strategies**:
- Input sanitization before passing data to AI.
- Prompt is stored and tracked (provenance).
- AI output is never used without human review for consequential
  decisions.
- System and user prompt separation is enforced.
- Token limits and resource limits on AI calls.
**Severity**: Medium (only relevant when AI is implemented)

### T-12: Denial of service

**Description**: An attacker exhausts system resources (CPU, memory,
database connections, storage) to make CIVORA unavailable.
**Affected assets**: System availability.
**Mitigation strategies**:
- Rate limiting on all API endpoints.
- Resource limits on workflow and rule evaluation.
- Request timeouts.
- Database connection pooling.
- Upload size and count limits.
**Severity**: Medium

### T-13: Insecure configuration

**Description**: CIVORA is deployed with insecure defaults (e.g.,
debug mode enabled, default credentials, TLS disabled).
**Affected assets**: All.
**Mitigation strategies**:
- Secure-by-default configuration.
- Startup checks fail if critical security settings are missing.
- No default credentials; first-run setup enforces admin password.
- TLS enforced in production; clear warnings in development.
**Severity**: High

---

## 4. Out of scope

The following are **not** covered by this threat model because they are
the responsibility of the deploying operator:

- Network-level attacks (firewalls, DDoS at the network layer).
- Physical security of the hosting infrastructure.
- Compromise of the operator's identity provider (IdP) beyond what
  CIVORA can detect via token validation.
- Social engineering attacks against CIVORA administrators.
- Side-channel attacks on the host infrastructure.

These are documented for clarity; operators should consider them in their
own risk assessments.

---

## 5. Review and revision schedule

This threat model is reviewed:

- At the start of each milestone.
- When a new trust boundary is introduced.
- When a new integration or external dependency is added.
- After any security incident.

Last reviewed: 2026-09-07.
Next review due: Before milestone 0.3.

## 6. v0.2 additions

The following threats apply specifically to the service delivery lifecycle
modules added in v0.2 (eligibility, assistance, follow-up):

### T-14: JSONB injection in eligibility criteria

**Description**: Malicious JSONB content injected into eligibility criteria
fields could exploit the JSON unmarshal path or cause panics.
**Affected assets**: Eligibility data, case data.
**Mitigation strategies**:
- JSONB fields are scanned using `json.Unmarshal` from `[]byte`, not
  directly into Go types.
- Input validation on eligibility criteria before storage.
- Case number collision retry logic prevents DB-level injection.
**Severity**: Medium

### T-15: UPSERT race conditions in assistance/follow-up

**Description**: Concurrent UPSERT operations on assistance or follow-up
records could lead to lost updates or inconsistent state.
**Affected assets**: Assistance records, follow-up records.
**Mitigation strategies**:
- PostgreSQL `ON CONFLICT DO UPDATE` ensures atomic upserts.
- `SELECT ... FOR UPDATE` used in audit hash chain to prevent concurrent
  modification.
- Service layer uses `SaveTx` with explicit transaction boundaries.
**Severity**: Low
