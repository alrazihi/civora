# Security Policy

## Reporting a vulnerability

The CIVORA team takes security vulnerabilities seriously. We appreciate
your responsible disclosure.

### How to report

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

### What happens next

1. A steward acknowledges your report within 48 hours.
2. The team triages and confirms the vulnerability.
3. A fix is developed in a private branch.
4. A release containing the fix is coordinated.
5. The vulnerability is publicly disclosed (with credit to the reporter,
   unless they prefer anonymity).

We will keep you informed throughout the process.

## Supported versions

Only the latest `main` branch and the latest release are supported for
security fixes.

| Version | Supported |
|---------|-----------|
| `main`  | Yes       |
| Latest release | Yes |
| Older releases | No  |

## Security practices

CIVORA's security practices are documented in:

- [docs/threat-model.md](docs/threat-model.md) — comprehensive threat
  model.
- [ARCHITECTURE.md](ARCHITECTURE.md) — security-relevant design
  decisions.
- [docs/principles.md](docs/principles.md) — security and privacy
  principles.
- [docs/0.9.5-stage-02-secure-transport.md](docs/0.9.5-stage-02-secure-transport.md)
  — transport security and deployment hardening.
- [docs/0.9.5-stage-03-confirmed-defects.md](docs/0.9.5-stage-03-confirmed-defects.md)
  — confirmed security defects and their fixes.
- [docs/0.9.5-stage-04-security-truth.md](docs/0.9.5-stage-04-security-truth.md)
  — what CIVORA provides, what operators must provide, and known limitations.

## What CIVORA provides

- Authentication and authorization on every request.
- Tenant isolation at the database, service, and API layers.
- A tamper-evident, hash-chained audit trail (SHA-256) stored in a
  dedicated `audit` schema.
- Timing-safe authentication paths and generic rate-limit responses.
- Evidence state-machine enforcement and decision supersession
  integrity.
- AI observation assistance that is architecturally isolated from
  consequential system functions and requires explicit human review.

## Operator / deployment responsibilities

CIVORA does not provide the following out of the box; operators are
responsible for them:

- TLS certificate management (when using direct TLS termination).
- Reverse-proxy configuration, HSTS headers, and trusted-proxy
  allowlisting.
- Database backups, restore testing, and disaster-recovery planning.
- Secrets management and rotation for database credentials, JWT
  signing keys, and AI provider keys.
- Network-level DDoS protection, firewall rules, and host hardening.
- Log aggregation, monitoring, and alerting.
- Compliance procedures required by the operator's regulatory
  environment (e.g., GDPR, HIPAA, SOC 2).

## Limitations

- **Not production-ready**: CIVORA is in early development
  (Milestones 0.x). It is not yet production-ready.
- **No encryption at rest**: PostgreSQL data is stored unencrypted by
  default. Operators should use tablespace encryption or disk-level
  encryption if required.
- **No built-in backup/DR**: Operators must configure PostgreSQL
  backups and test restores.
- **No GDPR data export/deletion APIs**: Bulk data-export and
  per-user deletion endpoints are not implemented.
- **No SOC 2 / HIPAA controls**: CIVORA does not implement SOC 2
  Trust Services Criteria or HIPAA technical safeguards. Compliance
  with these frameworks depends on the operator's deployment
  configuration, policies, and controls.
- **Audit trail limitations**: The hash chain detects accidental or
  offline tampering, but a compromised application process or database
  administrator can write a valid chain with false contents. There is
  no external anchor or append-only storage.
- **JWT non-revocable**: Tokens are short-lived (24h) but cannot be
  revoked before expiry; a logout/revocation endpoint is planned for
  a future milestone.

---

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

## Disclosure policy

- We ask that you give us a reasonable amount of time to fix the issue
  before public disclosure.
- We will not initiate legal action against anyone who reports a
  vulnerability in good faith.
- If you make a good-faith attempt to avoid privacy breaches and system
  disruption, you will not face any penalties.

## Bug bounty

CIVORA does not currently offer a paid bug bounty program. We may
introduce one in the future if the project grows sufficiently.

---
