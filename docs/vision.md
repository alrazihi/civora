# CIVORA Vision

## What CIVORA is

CIVORA is an open-source platform that provides the digital infrastructure
for public-interest institutions to **create**, **operate**, **audit**, and
**improve** complex service-delivery processes.

Concretely, CIVORA provides:

- A **workflow engine** for modeling and executing multi-step service
  processes (e.g., emergency assistance, permit applications, benefit
  eligibility checks).
- **Form and document management** for collecting and storing evidence and
  submissions, with cryptographic audit trails.
- **Case management** for tracking individual service requests as they move
  through a workflow.
- **Policy and rule evaluation** as a first-class, auditable concern.
- **AI assistance** (in future milestones) for summarizing, classifying, and
  surfacing recommendations — always under human authority.
- **Observability and audit** tooling that makes every decision, action,
  and state change attributable and reproducible.
- An **extensible integration layer** for connecting to identity providers,
  document verification services, payment systems, and other external APIs.

CIVORA is **API-first**: all functionality is reachable through
standards-based HTTP APIs, published as OpenAPI specifications.

## What problem CIVORA solves

Public-interest institutions — governments, NGOs, humanitarian agencies,
social-service organizations — must deliver services to people in need.
These services are often complex, involving many steps, many actors, and
many systems. Today, institutions typically solve this by:

1. **Vendor lock-in**: adopting proprietary platforms that are expensive,
   opaque, and difficult to leave.
2. **Fragmented tooling**: stitching together spreadsheets, email, shared
   drives, and bespoke software that cannot interoperate or be audited.
3. **No meaningful audit**: decisions affecting people are made without
   traceable, reproducible records.
4. **No human review of automated decisions**: AI and automation are
   applied opaquely, with no requirement for human oversight or
   explainability.

CIVORA addresses these problems by providing a single, open, modular
platform that institutions can self-host, audit, and extend.

## Who CIVORA serves

| Category | Needs |
|----------|-------|
| **Governments** | Deliver social services, emergency response, permits, and benefits at scale while maintaining auditability and citizen trust. |
| **NGOs / Humanitarian** | Coordinate aid delivery, track beneficiaries, and maintain evidence chains in resource-constrained or crisis environments. |
| **Social service organizations** | Manage case workflows, collect documentation, and coordinate among multiple staff roles. |
| **Developers & integrators** | Extend CIVORA, write custom workflows, and integrate with existing institutional systems. |
| **Auditors & researchers** | Inspect system behavior, verify compliance, and study service-delivery outcomes. |
| **Service recipients** | Access clear information about their case status, submitted evidence, and decision rationale. |

## What CIVORA explicitly does NOT attempt to do

CIVORA is deliberately scoped. It does **not** aim to:

- **Be a generic CRM or ERP** for all institutional management. CIVORA
  focuses on *process execution* and *case/service delivery workflows*.
- **Replace direct citizen-facing portals out of the box.** CIVORA provides
  the underlying engine and APIs; front-end applications are expected to be
  built by implementers (or built as thin UI layers on top of the API).
- **Make consequential decisions about people autonomously.** AI
  assistance is advisory only; human approval is always required for
  impactful decisions. See the [AI principle](docs/principles.md#ai-principle).
- **Provide a hosted SaaS offering.** CIVORA is software to be deployed on
  the institution's own infrastructure. The founders do not operate a SaaS
  service.
- **Guarantee security by documentation alone.** Security is an ongoing
  practice, not a statement. See
  [docs/threat-model.md](threat-model.md).
- **Enforce any specific regulatory regime.** CIVORA provides the
  technical building blocks (audit logs, retention controls, tenant
  isolation); compliance with GDPR, HIPAA, local data-protection laws, etc.
  is the responsibility of the deploying institution and its implementers.
- **Pre-package domain-specific workflows** as the primary product. Domain
  workflows are provided as **examples and starter templates**, not as the
  system's core value proposition.
- **Become a microservices platform** in its first version. See
  [ARCHITECTURE.md](ARCHITECTURE.md) and
  [docs/decisions/0001-initial-architecture.md](decisions/0001-initial-architecture.md).

## Public-interest mission

CIVORA exists to reduce the societal cost of bureaucratic opacity and
technological lock-in in the public sector. By providing free, open
infrastructure for service delivery, CIVORA aims to:

- Reduce the cost of delivering public services.
- Increase transparency and accountability in service-delivery decisions.
- Enable citizen participation and civic oversight through auditable
  systems.
- Ensure that public institutions are not held hostage by proprietary
  vendors.
- Lower the barrier for NGOs and humanitarian organizations to adopt
  professional-grade process infrastructure.

The project's success is measured not by downloads or stars, but by
whether it is adopted, deployed, and trusted by institutions that serve
the public interest — and whether those institutions can operate,
understand, and improve their own systems independently.

## Open-source philosophy

CIVORA is, and will always be, released under the Apache License 2.0. The
project does not use copyleft or "source-available" licenses for its core
components, because:

- Institutions (including governments) often have legal or procurement
  constraints that limit their ability to use copyleft-licensed software.
  Apache 2.0 is broadly acceptable.
- Apache 2.0 provides patent protection for contributors and users, which
  is critical for infrastructure software.
- Permissive licensing encourages commercial adoption and integration,
  which funds further development through ecosystem growth.

The project accepts contributions under the same license. See
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Governance philosophy

CIVORA follows a **community-driven, consensus-oriented** governance model:

- Decisions are made transparently, primarily through GitHub issues and
  pull requests, with architecture decisions recorded as Architecture
  Decision Records (ADRs).
- The project is governed by a small group of **stewards** who provide
  technical direction, review contributions, and maintain project health.
  See [GOVERNANCE.md](../GOVERNANCE.md).
- All significant proposals are discussed publicly before implementation.
- Leadership is earned through sustained contribution, not appointed from
  outside.
- The project resists corporate capture: a single entity should not be
  able to unilaterally redirect the project's technical direction.

## Security philosophy

Security in CIVORA is approached as an ongoing, collaborative practice, not
a checklist:

- **Security by design**: security considerations are integrated into
  architecture decisions from the start.
- **Defense in depth**: no single layer is trusted to provide all
  security; multiple independent controls protect against common failure
  modes.
- **Transparency**: the threat model and security posture are documented
  openly. Private reports are handled through a coordinated disclosure
  process (see [SECURITY.md](../SECURITY.md)).
- **Minimal privilege**: the system follows least-privilege principles
  internally and externally.
- **No magic security**: security is achieved through inspectable code and
  standard practices, not proprietary or opaque mechanisms.

## Privacy philosophy

Privacy is a design constraint, not an afterthought:

- **Data minimization**: CIVORA collects only the data necessary to
  fulfill its functions. Personal data is processed only as needed for
  service delivery and auditability.
- **Privacy by configuration**: the system supports deployment without
  collecting or retaining personal data beyond what is legally required.
- **Data ownership**: data belongs to the institution that collected it.
  CIVORA does not retain copies of production data.
- **Portability**: data must be exportable in standard formats. The system
  must be able to be fully decommissioned without data loss.

## Interoperability philosophy

CIVORA is designed to connect, not to isolate:

- **Standard exchange formats**: data is stored and exported in
  well-documented, preferably open-standard, formats.
- **Open APIs**: all platform functionality is exposed via documented HTTP
  APIs with OpenAPI specifications.
- **Pluggable integrations**: external systems (identity providers,
  payment systems, document verification, etc.) connect through a
  provider-agnostic interface so new providers can be added without
  core changes.
- **Model-provider-neutral AI**: AI functionality uses adapter patterns
  so different model providers can be swapped without core changes.
- **Database-provider-neutral**: the system avoids vendor-specific
  database features where practical, to support a range of deployment
  environments.

## Accessibility requirements

CIVORA targets compliance with **WCAG 2.1 Level AA** for any user-facing
components the project ships. This includes:

- Semantic HTML structure in built-in UI (if any).
- Keyboard navigation support.
- Screen-reader compatibility.
- Sufficient color contrast.
- Support for text scaling and alternative input methods.

For API-first components, accessibility is extended through providing
machine-readable API specifications so that third-party UI developers can
build accessible interfaces.

## Localization / Internationalization strategy

CIVORA is designed for global use:

- All user-facing strings are externalized into locale resource files.
- All timestamps are stored as UTC and rendered in the requester's
  timezone where appropriate.
- All names and text fields support Unicode.
- The system is designed to avoid culturally specific assumptions
  (e.g., name parsing, address formats, date formats) by using flexible,
  schema-driven data structures wherever user-defined data is involved.
- Locales are loaded dynamically; the system does not hardcode a single
  language.

The project will support localization of its own documentation and UI, but
does not maintain translations in-tree for all languages — it provides the
mechanism and infrastructure for the community to contribute translations.
