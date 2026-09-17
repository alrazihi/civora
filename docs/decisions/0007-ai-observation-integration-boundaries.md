# ADR-0007: AI Observation Integration Boundaries

- **Date**: 2026-09-17
- **Status**: Accepted
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA's mission is to provide auditable, deterministic infrastructure for
public-service workflows and human-in-the-loop decision making. As the platform
adds AI-powered observation assistance in Milestone 0.8, a critical
architectural question arises:

> Where may AI participate in the system without compromising determinism,
auditability, or human agency over outcomes?

Public-interest institutions cannot delegate authoritative judgments to
non-deterministic models. AI may help humans *observe* and *interpret* case
information, but it must never autonomously advance workflow state, trigger
rules, or produce decisions. Any AI-generated content must flow through
explicit human gates before it can influence the system.

Without clear boundaries, AI capabilities can creep into deterministic paths
(workflow transitions, rule evaluation, decision records), eroding the
tamper-evident audit trail and the guarantee that every outcome is
human-accountable.

## Decision

**AI is confined to the observation slot with mandatory human review.**
AI never produces decisions, advances workflows, or triggers rules.

The AI Observations module is introduced as an independent module that owns
observation entities, human review tracking, and provider abstraction. The
following boundaries are enforced in code and by design in 0.8:

1. **Observation slot only.** AI output is captured as an observation entity.
   Observations are advisory artifacts attached to cases and evidence. They do
   not write to, call into, or influence the Decisions, Workflow, or Rules
   modules through any code path.

2. **Two human gates.** Every observation requires (a) human review of the AI
   output and (b) explicit acceptance before the observation is recorded as
   reviewed. Unreviewed or rejected observations have no effect on case state,
   workflow transitions, or rule evaluations.

3. **No AI-to-decision code.** There is no code path from an AI provider
   response to a decision, transition, rule evaluation, or notification. An
   AI observation can never directly or indirectly cause a state change.

4. **Noop default provider.** When `CIVORA_AI_ENABLED` is unset or disabled, the
   Noop provider is used. The Noop provider produces no observations and
   performs no external calls, ensuring zero behavioral change when AI is off.

5. **Off-by-default configuration.** AI is controlled by
   `CIVORA_AI_ENABLED` (default off). The provider is selectable between
   Noop, Local, and OpenAI. No tenant, case, or workflow is processed by AI
   unless the operator has explicitly enabled it.

6. **Observations not fed to rules in 0.8.** In this milestone, observation
   entities are not surfaced as facts to the Rules engine. AI observations are
   human-consumed artifacts only. (Future milestones may expose reviewed
   observations as an optional fact source, subject to a separate ADR.)

All AI observations produce hash-chained audit events (`observation_created`,
`observation_reviewed`, `observation_provided`) so that every AI interaction
is attributable, immutable, and reproducible.

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| AI cannot act autonomously | Reduces "smart" automation but preserves human agency and audit integrity. |
| Extra human gate latency | Observations must be reviewed before being marked reviewed; this is intentional friction. |
| Limited AI scope in 0.8 | Observations are not fed to rules, constraining AI's immediate utility. Expanded integration deferred. |
| Noop provider overhead | A no-op code path exists alongside real providers; this is negligible cost for safety and testability. |

## Assumptions

- Public-interest institutions require human accountability for case outcomes.
- Auditable systems must remain deterministic at the workflow and decision
  layers.
- Human reviewers can meaningfully evaluate AI-generated observations.
- Operators will explicitly opt in to AI features via configuration.

## How this decision can be reversed or evolved

The AI Observations module is isolated behind a provider abstraction and a
feature flag. If AI integration is determined to be inappropriate, the module
can be disabled entirely with no effect on core system behavior (Noop default).

Future expansion of AI into other system paths (e.g., surfacing reviewed
observations as rule facts) will require a new or updated ADR documenting the
additional human gates, audit requirements, and risk mitigations. Such expansion
will not occur without explicit architectural approval.

## Consequences

### Positive

- AI assistance enhances human observation without compromising determinism.
- All AI interactions are auditable and attributable.
- Human agency over decisions, workflow, and rules is preserved.
- Off-by-default deployment ensures no surprise behavioral changes.

### Negative

- AI's immediate utility is limited to a single observation review step.
- Additional human review steps may slow case processing for teams that opt in.

### Neutral

- The Noop provider ensures the AI module is exercised in tests without
  external dependencies or real model calls.

## Alternatives considered

### Alternative A: Confine AI to the observation slot (chosen)

- **Pros**: Preserves determinism, auditability, and human agency; clear
  boundary easy to verify and test; off-by-default and Noop-safe.
- **Cons**: Limits AI's immediate impact on workflow efficiency.

### Alternative B: Allow AI to influence rule facts or workflow suggestions

- **Pros**: Could accelerate case processing by providing actionable AI insight.
- **Cons**: Risks eroding determinism; introduces non-deterministic inputs
  into auditable paths; increases attack surface; contradicts human-in-the-loop
  guarantee. **Rejected.**

### Alternative C: Run AI as a fully autonomous actor with human override

- **Pros**: Maximum automation.
- **Cons**: Violates the core principle that every outcome must be human-
  accountable; unsuitable for public-interest infrastructure. **Rejected.**

## References

- [ADR-0001: Initial Architecture — Modular Monolith](0001-initial-architecture.md)
- [ADR-0006: Configurable Workflow Engine](0006-configurable-workflow-engine.md)
- [Architecture — Module table](ARCHITECTURE.md)
- [Audit module documentation](internal/audit/README.md)
