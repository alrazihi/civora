# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for CIVORA.

## What is an ADR?

An ADR is a document that captures an important architectural decision:
**what** was decided, **why**, what **alternatives** were considered, and
what the **trade-offs** and **assumptions** were. Every significant
technical decision must be recorded as an ADR.

## ADR list

| # | Title | Status | Date |
|---|-------|--------|------|
| [0001](0001-initial-architecture.md) | Initial Architecture — Modular Monolith | Accepted | 2026-09-06 |
| [0002](0002-why-go.md) | Why Go for the Backend | Accepted | 2026-09-06 |
| [0003](0003-why-postgresql.md) | Why PostgreSQL | Accepted | 2026-09-06 |
| [0004](0004-why-rest-openapi.md) | Why REST and OpenAPI | Accepted | 2026-09-06 |
| [0005](0005-why-react-typescript.md) | Why React and TypeScript for the Frontend | Superseded | 2026-09-06 |
| [0006](0006-configurable-workflow-engine.md) | Configurable Workflow Engine | Accepted | 2026-09-06 |
| [0006-b](0006-public-interest-domain-model.md) | Public-Interest Domain Model | Accepted | 2026-09-06 |
| [0007](0007-ai-observation-integration-boundaries.md) | AI Observation Integration Boundaries | Accepted | 2026-09-17 |

The second 0006 document uses the file `0006-public-interest-domain-model.md`;
it is indexed here as `0006-b` to avoid a numbering collision while preserving
the existing filename.

## ADR template

New ADRs should follow the template in [0000-template.md](0000-template.md).

## Process

1. Create a file: `NNNN-short-title.md` where `NNNN` is the next number.
2. Fill in the template.
3. Submit a pull request for review.
4. Once accepted, the status is updated to "Accepted" or "Rejected".

See [CONTRIBUTING.md](../CONTRIBUTING.md#architecture-decisions) for
details.

---

## ADR index (by status)

### Accepted
- [0001](0001-initial-architecture.md) — Initial Architecture — Modular Monolith
- [0002](0002-why-go.md) — Why Go for the Backend
- [0003](0003-why-postgresql.md) — Why PostgreSQL
- [0004](0004-why-rest-openapi.md) — Why REST and OpenAPI
- [0006](0006-configurable-workflow-engine.md) — Configurable Workflow Engine
- [0006-b](0006-public-interest-domain-model.md) — Public-Interest Domain Model
- [0007](0007-ai-observation-integration-boundaries.md) — AI Observation Integration Boundaries

### Superseded
- [0005](0005-why-react-typescript.md) — Why React and TypeScript for the Frontend (frontend implemented as a static HTML/CSS/JS SPA; React deferred)

### Rejected
- (none)
