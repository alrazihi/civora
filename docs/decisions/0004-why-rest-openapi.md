# ADR-0004: Why REST and OpenAPI

- **Date**: 2026-09-06
- **Status**: Accepted
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA must expose its functionality through a well-defined API. The API
strategy affects how institutions integrate with the system, how the
frontend communicates with the backend, and how external systems access
CIVORA's capabilities.

The API must be: standards-based, versioned, documented, secure, and
accessible to a wide range of integrators including developers, third-party
UI builders, and automated systems.

## Decision

**Use REST over HTTP with JSON payloads, documented via OpenAPI 3.0.**

The API follows REST conventions: resource-oriented URLs, standard HTTP
methods (GET, POST, PUT, PATCH, DELETE), standard HTTP status codes, and
JSON for request/response bodies.

The API specification is the source of truth for the API contract, maintained
as an OpenAPI 3.0 YAML file (`api/openapi/openapi.yaml`).

## Alternatives considered

### Alternative A: REST + OpenAPI (chosen)

- **Pros**: Universally understood; works with any HTTP client; excellent
  tooling (OpenAPI generators, validators, documentation viewers); compatible
  with API gateways, proxies, and monitoring tools; easy to test with `curl`
  and common HTTP libraries; human-readable JSON payloads.
- **Cons**: Can result in over-fetching or under-fetching for complex
  queries; no built-in real-time capabilities (requires polling or WebSockets);
  versioning must be handled at the URL or header level.

### Alternative B: GraphQL

- **Pros**: Precise data fetching; single endpoint; strong typing; real-time
  subscriptions possible.
- **Cons**: More complex to implement and secure (query depth limiting,
  complexity analysis, N+1 prevention); tooling is more opinionated; less
  familiar to many public-sector developers; caching is more difficult.
- **Verdict**: GraphQL is powerful but introduces complexity that is not
  justified for the initial vertical slice. May be considered for future
  read-heavy endpoints where precise data fetching is important.

### Alternative C: gRPC

- **Pros**: Efficient binary protocol; strong typing via protobuf; streaming
  support; good for service-to-service communication.
- **Cons**: Requires HTTP/2; browser support requires a proxy (gRPC-Web);
  less human-readable; tooling is less familiar to typical public-sector
  developers; harder to debug with standard HTTP tools.
- **Verdict**: gRPC is suitable for inter-service communication in a
  microservices architecture, but CIVORA uses a modular monolith. REST is
  more appropriate for the public API.

### Alternative D: GraphQL + REST hybrid (BFF pattern)

- **Pros**: Best of both worlds for complex applications.
- **Cons**: Increases architectural complexity; requires maintaining two
  API styles; not justified for the initial scope.
- **Verdict**: May be reconsidered if the API surface grows significantly
  and precise data fetching becomes a bottleneck.

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| REST can be chatty | Multiple round-trips may be needed for complex operations. Acceptable for human-paced workflows. |
| No real-time push | Clients must poll for updates or use a separate real-time channel. Acceptable for initial scope. |
| Versioning via URL prefix | `/api/v1/` prefix used. Breaking changes require a new major version (e.g., `/api/v2/`). |

## Assumptions

- Consumers of the API can use standard HTTP clients (curl, Python requests,
  JavaScript fetch, etc.).
- JSON is an acceptable data format for all payloads.
- The API design conventions (pagination, error responses, status codes) are
  sufficient for the initial use cases.

## How this decision can be reversed or evolved

The API layer is the outermost layer of the system and is decoupled from the
internal domain logic. If REST becomes limiting (e.g., for real-time
collaboration), a GraphQL or gRPC endpoint can be added alongside REST
without modifying the domain or application layers. The OpenAPI spec can be
used to generate server stubs in alternative technologies if needed.

## Consequences

- All API endpoints are defined in `api/openapi/openapi.yaml`.
- API responses follow a standardized envelope format: `{ success, data, meta }`.
- Errors follow a standardized format with machine-readable error codes.
- All endpoints use Bearer token authentication (JWT).
- API endpoints are versioned via URL prefix (`/api/v1/`).
- Pagination is via `page` and `per_page` query parameters.
- OpenAPI validation is part of the CI pipeline.

## References

- [ADR-0001: Initial Architecture — Modular Monolith](0001-initial-architecture.md)
- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [RESTful Web Services](https://www.oreilly.com/library/view/restful-web-services/9780596529276/) by Leonard Richardson and Sam Ruby

---

*Note: The original reference URL (roy.gerrardbrayfieldfield.com) was broken during a site migration. It has been replaced with the canonical O'Reilly book reference.*
