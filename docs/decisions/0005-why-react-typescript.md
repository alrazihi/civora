# ADR-0005: Why React and TypeScript for the Future Frontend

- **Date**: 2026-09-06
- **Status**: Proposed
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA's API is usable independently of any frontend. However, the project
anticipates building a user-facing UI in the future. The frontend technology
must be: open source, accessible (WCAG 2.1 AA), internationalizable, familiar
to a broad pool of contributors, and deployable alongside or independently of
the backend.

The frontend is explicitly NOT being implemented in the initial scope
(Milestone 0.1). This ADR records the rationale for the future technology
choice.

## Decision

**Use React with TypeScript for the future CIVORA frontend.**

This decision is proposed but not yet implemented. The frontend will be
developed as a separate package within the repository, consuming the REST API.

## Alternatives considered

### Alternative A: React + TypeScript (chosen)

- **Pros**: Largest ecosystem; most contributors have React experience;
  TypeScript provides static typing; strong accessibility tooling (React ARIA,
  Reach UI); internationalization libraries (react-i18next); deployable as
  static files behind any web server.
- **Cons**: React's ecosystem evolves rapidly (hooks, new renderer APIs);
  bundle sizes can be large if not optimized.

### Alternative B: Vue.js

- **Pros**: Simpler mental model; smaller bundle sizes; excellent
  internationalization support.
- **Cons**: Smaller talent pool in the public-sector space compared to React;
  fewer UI component libraries for accessible, government-grade interfaces.

### Alternative C: Svelte / SvelteKit

- **Pros**: Compile-time optimization; minimal runtime; small bundles;
  excellent performance.
- **Cons**: Smaller ecosystem; fewer accessible component libraries; less
  familiar to typical contributors.

### Alternative D: Vanilla TypeScript (no framework)

- **Pros**: No framework overhead; maximum control; zero bundle bloat.
- **Cons**: Development velocity would be slower for complex, interactive
  UIs; requires building all state management, routing, and component
  infrastructure from scratch; harder for community contributors to
  onboard.

### Alternative E: Server-side rendered HTML (HTMX, server components)

- **Pros**: Minimal JavaScript; good progressive enhancement; fast initial
  load.
- **Cons**: More complex backend; less interactive UX; not suitable for the
  richer workflows CIVORA anticipates; harder to test as a separate artifact.

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| Learning curve for new contributors | Mitigated by React's popularity and extensive documentation. |
| Bundle size | Mitigated by code splitting, tree-shaking, and using accessible component libraries. |
| Ecosystem churn | Mitigated by pinning versions and using established libraries (e.g., React Query, Zod). |

## Assumptions

- A frontend will be needed in future milestones (0.3+).
- Accessibility (WCAG 2.1 AA) compliance is achievable with React's
  ecosystem.
- TypeScript's type safety will reduce bugs in the frontend codebase.
- The frontend will be a thin API consumer, with business logic remaining
  in the backend.

## How this decision can be reversed or evolved

If React proves unsuitable (e.g., due to ecosystem complexity or bundle
size), the frontend can be rewritten in another framework. Since the
frontend consumes a REST API and has no shared code with the backend, the
switching cost is limited to the frontend codebase.

This decision is marked as **Proposed** (not Accepted) because the frontend
has not yet been implemented. It will be confirmed or revised when the
frontend development begins (milestone 0.3).

## Consequences

- When the frontend is implemented, it will live in a separate directory
  (e.g., `frontend/`) with its own package.json.
- The frontend will be built as static assets, deployable behind any web
  server or CDN.
- The frontend will use the REST API exclusively for data; no direct
  database or internal API access.
- TypeScript types will be generated from the OpenAPI specification where
  practical.

## References

- [ADR-0001: Initial Architecture — Modular Monolith](0001-initial-architecture.md)
- [ADR-0004: Why REST and OpenAPI](0004-why-rest-openapi.md)
- [React official site](https://react.dev/)
- [TypeScript official site](https://www.typescriptlang.org/)
