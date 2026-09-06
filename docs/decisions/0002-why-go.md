# ADR-0002: Why Go for the Backend

- **Date**: 2026-09-06
- **Status**: Accepted
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA needs a backend implementation language. The language must be suitable
for a modular monolith serving public-interest institutions with limited
infrastructure. Key requirements include: efficient resource usage, fast
compile times, strong standard library, good tooling, clear error handling,
strong concurrency primitives, and broad accessibility for contributors.

The language must also support clear module boundaries within a single
binary, have reliable database drivers for PostgreSQL, and produce static
binaries that can be deployed without complex runtime environments.

## Decision

**Use Go (Golang) as the backend implementation language.**

Go is selected for the initial implementation because it directly satisfies
the project's core constraints: simplicity, efficiency, clarity, and
deployability.

## Alternatives considered

### Alternative A: Go (chosen)

- **Pros**: Fast compilation, single static binary deployment, excellent
  standard library, built-in concurrency, straightforward error handling,
  idiomatic code is readable and maintainable, broad industry adoption.
- **Cons**: Generic programming is more limited than Rust; garbage
  collection pause times may be higher than C++ (acceptable for this
  use case); no generics until Go 1.18 (now available).

### Alternative B: Rust

- **Pros**: Zero-cost abstractions, memory safety without GC, high
  performance.
- **Cons**: Steeper learning curve; ownership model may deter community
  contributors; longer compile times; smaller talent pool for public-sector
  contributors.

### Alternative C: Python

- **Pros**: Familiar to many public-sector developers; rich ecosystem for
  data processing and AI.
- **Cons**: Poor concurrency model (GIL); slow runtime; large memory footprint;
  requires a managed runtime environment. Not suitable for resource-constrained
  deployments.

### Alternative D: Node.js / TypeScript (backend)

- **Pros**: Familiar to frontend developers; large npm ecosystem.
- **Cons**: Asynchronous-by-default model is error-prone; larger memory
  footprint than Go; runtime dependency required; type safety requires
  additional tooling (tsc, strict mode).

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| No generic polymorphism | Code may have some repetitive patterns, but is highly readable. Generics (Go 1.18+) are used sparingly. |
| Garbage collection | Minor latency pauses are acceptable for a service-delivery workflow system with human-paced interactions. |
| Single-threaded compile per package | Fast incremental builds; acceptable as the codebase is modularized for parallel compilation. |

## Assumptions

- Contributors will have basic familiarity with Go or can learn it quickly.
- The performance characteristics of Go are sufficient for the expected
  load (tens to low hundreds of concurrent users per deployment initially).
- Static binary deployment aligns with the on-premises and limited-infrastructure
  deployment goals.

## How this decision can be reversed or evolved

Go is a language choice for the initial implementation. The modular monolith
architecture (ADR-0001) means that language choices are isolated per module.
Individual modules could be reimplemented in another language (e.g., Rust for
a performance-critical module) without rewriting the entire system, as long as
module boundaries are maintained through defined interfaces.

However, changing the primary language would require significant effort and is
not anticipated for the foreseeable future.

## Consequences

- The project will use idiomatic Go patterns: `context.Context` for request
  scoping, explicit error returns, small focused functions.
- The project will use Go modules for dependency management.
- CI pipelines will use `go vet`, `gofmt`, and `go test`.
- Deployment will produce a single static binary.

## References

- [ADR-0001: Initial Architecture — Modular Monolith](0001-initial-architecture.md)
- [Go official website](https://go.dev/)
- [Go: Why Go?](https://go.dev/doc/why)
