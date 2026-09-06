# Security Policy

## Reporting a vulnerability

The CIVORA team takes security vulnerabilities seriously. We appreciate
your responsible disclosure.

### How to report

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, report by email to: **security@civora.org**

If you do not receive a response within 48 hours, or if the matter is
urgent, contact a project steward directly via GitHub.

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

## Credits

We maintain a list of individuals who have responsibly disclosed
security vulnerabilities. This list will be published in
`docs/security-credits.md` (to be created when the first vulnerability
is reported).

---

Last updated: 2026-09-06
