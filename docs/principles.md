# CIVORA Principles

This document defines the core principles that guide all CIVORA
development decisions. Every pull request, architecture decision, and
feature proposal should be evaluated against these principles.

---

## 1. Open source

CIVORA is released under the Apache License 2.0. All development happens
in the open. No functionality is reserved for paid tiers or private
forks.

## 2. Public-interest first

CIVORA serves the public interest, not corporate profit. Decisions that
benefit shareholders of a single company are avoided unless they also
serve the public interest.

## 3. API-first

All functionality is exposed through documented, versioned HTTP APIs.
User interfaces are consumers of the API, not the API itself. This
ensures that the system is scriptable, automatable, and integrable.

## 4. Modular architecture

The codebase is organized into clearly bounded, loosely-coupled modules.
Each module has a single responsibility and communicates with others
through well-defined interfaces. This enables future extraction into
services and independent testing.

## 5. Secure by default

Security is a default requirement, not an opt-in feature:

- Authentication and authorization are enforced on every API endpoint by
  default.
- The system does not ship with default credentials.
- TLS is required for all external communication in production.
- Input validation is enforced at all boundaries.

## 6. Privacy-aware

Data minimization, user control, and auditability are built in:

- Personal data is processed only for stated purposes.
- Retention periods are configurable and enforced.
- Data export and deletion are supported throughout the system.
- Logs and audit records do not contain more information than necessary.

## 7. Human-centered

The system augments human decision-makers; it does not replace them:

- All consequential decisions require human review and approval.
- User interfaces are designed for clarity, not just efficiency.
- Errors are communicated clearly to users.
- The system surfaces uncertainty and confidence levels where relevant.

## 8. Auditable

Every action that affects state is recorded:

- Every API request that modifies data produces an audit event.
- Audit events are immutable and tamper-evident.
- The system can reproduce the state of any entity at any point in time
  given its event log.
- Audit data is stored separately from operational data and has its own
  retention and access controls.

## 9. Observable

The system produces metrics, logs, and traces that enable operators to
understand its behavior in production:

- Structured logging follows a consistent format.
- Key metrics (throughput, error rates, latency) are exposed via a
  standard metrics endpoint.
- Distributed tracing is supported for multi-step operations.
- Health checks are available for all critical subsystems.

## 10. Interoperable

CIVORA uses open standards and common formats:

- HTTP for all network communication.
- JSON for all API payloads.
- OpenAPI for API specifications.
- JWT (or compatible) for authentication tokens.
- Standard database formats and query languages.
- Adapter patterns for external service integrations to avoid lock-in.

## 11. Deployable on-premises

CIVORA must run on standard infrastructure without cloud-specific
dependencies:

- Runs on commodity Linux servers or containers.
- No mandatory SaaS dependencies.
- Configuration via environment variables and/or config files.
- Single-binary deployment or standard container image.

## 12. Deployable in cloud environments

CIVORA must also run efficiently in cloud, container-orchestration, and
platform-as-a-service environments:

- Compatible with Docker, Kubernetes, and standard container runtimes.
- Stateless by design where possible; state is externalized to databases
  or object storage.
- Health checks and readiness probes are supported.

## 13. Suitable for limited-infrastructure organizations

CIVORA must be deployable by organizations with limited IT resources:

- Minimal infrastructure dependencies.
- Low memory and CPU footprint by default.
- Optional components can be disabled.
- Clear, documented deployment procedures.

## 14. Vendor-neutral

CIVORA does not favor any specific vendor, cloud provider, or technology
stack. Integrations are pluggable.

## 15. Model-provider-neutral

AI functionality uses adapter patterns. Different model providers (or
no provider at all) can be used without core changes.

## 16. Database-provider-neutral where practical

The system avoids database features that are specific to a single vendor.
Where vendor-specific features provide significant benefit, they are
isolated behind an adapter interface.

## 17. Auditable AI (when AI is introduced)

When AI functionality is added (starting in milestone 0.7), it must
satisfy all of the following:

- **Human review**: AI output is never applied without explicit human
  approval for consequential decisions.
- **Explainability**: where practical, the system provides explanations
  for AI-generated recommendations.
- **Provenance**: the system records which model, prompt, and input
  produced each AI output.
- **Auditability**: AI actions are part of the immutable audit log.
- **Confidence/uncertainty**: AI outputs include confidence scores or
  uncertainty measures where the model supports them.
- **Model identification**: the specific model and version used is
  recorded.
- **Prompt/version tracking**: prompts and prompt versions are tracked.
- **Data minimization**: AI processing uses only the minimum data
  necessary.
- **Configurable approval**: the level of human oversight is
  configurable per workflow, per organization, and per action type.
- **Ability to disable**: AI functionality can be completely disabled at
  the organization or system level without affecting other features.

## 18. Correctness over performance

When a trade-off exists between correctness and performance, correctness
wins. Performance optimizations that compromise correctness, auditability,
or security are rejected.

## 19. No premature optimization

The system is designed to be correct and maintainable first. Performance
optimization happens after correctness is established and bottlenecks are
measured.

## 20. Explicit is better than implicit

System behavior should be as obvious as possible from its configuration
and code. Magic behavior, hidden state, and implicit side effects are
discouraged.
