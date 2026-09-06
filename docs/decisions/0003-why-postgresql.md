# ADR-0003: Why PostgreSQL

- **Date**: 2026-09-06
- **Status**: Accepted
- **Deciders**: CIVORA founding stewards

---

## Context

CIVORA requires a primary database that supports multi-tenancy, strong tenant
isolation, ACID transactions, auditability, migrations, and the ability to
evolve the schema over time. The database must be suitable for on-premises
deployment by institutions with limited infrastructure, as well as cloud
deployments.

The system must avoid vendor lock-in where practical, but must also provide
the reliability and features needed for a public-interest service-delivery
system.

## Decision

**Use PostgreSQL as the primary database.**

PostgreSQL is selected as the default and primary database for CIVORA.

## Alternatives considered

### Alternative A: PostgreSQL (chosen)

- **Pros**: Battle-tested; strong ACID guarantees; excellent support for
  transactions, constraints, and referential integrity; strong multi-tenancy
  support via row-level security and schema-based isolation; extensible (custom
  types, functions); strong JSONB support for flexible metadata; open source
  with a large community; runs on commodity hardware; no licensing cost.
- **Cons**: Slightly heavier than SQLite for embedded use; requires a separate
  server process for production deployments.

### Alternative B: SQLite

- **Pros**: Zero-configuration; single-file; embedded; great for development
  and very small deployments.
- **Cons**: Limited concurrency (database-level write locking); no row-level
  security; weak multi-tenant isolation; not suitable for production
  deployments with concurrent writers; limited for large-scale audit logging.
- **Use**: SQLite is acceptable for local development and very small
  deployments, but PostgreSQL is the primary target.

### Alternative C: MySQL / MariaDB

- **Pros**: Widely deployed; familiar to many operators; good performance.
- **Cons**: Weaker transaction semantics than PostgreSQL (e.g., limited
  serializability support); less mature row-level security; JSONB handling is
  less powerful than PostgreSQL's; the dual-licensing model of MySQL is less
  favorable for an open-source infrastructure project.
- **Verdict**: Acceptable but not preferred; PostgreSQL is superior for
  CIVORA's audit and multi-tenancy requirements.

### Alternative D: Microsoft SQL Server

- **Cons**: Proprietary (licensing costs); not suitable for on-premises
  deployment by public-interest organizations with limited budgets; not
  widely available in cloud environments without significant cost.

### Alternative E: Oracle Database

- **Cons**: Proprietary (high licensing costs); not suitable for
  public-interest open-source infrastructure; vendor lock-in risk.

## Trade-offs

| Trade-off | Implication |
|-----------|-------------|
| Requires a database server process | Slightly more complex deployment than SQLite, but standard for production systems. Mitigated by Docker Compose for local dev. |
| PostgreSQL-specific features (e.g., JSONB, RLS) | May complicate future database switching. Mitigated by repository pattern (data access isolated behind interfaces). |

## Assumptions

- Most deploying institutions have or can install PostgreSQL.
- PostgreSQL's resource requirements are acceptable for the target
  deployment environments.
- The repository pattern will allow database changes in the future without
  affecting the domain layer.

## How this decision can be reversed or evolved

The data access layer is encapsulated behind repository interfaces (see
`internal/.../domain/repository.go` in each module). If a different database
is needed in the future, a new repository implementation can be provided.
Migrations are written in standard SQL, minimizing database-specific
lock-in. However, some PostgreSQL-specific features (e.g., JSONB columns)
may require migration effort if a different database is chosen.

## Consequences

- All database migrations are written as standard SQL files, stored in
  `migrations/`.
- The project uses the `pgx` driver for PostgreSQL connectivity.
- Database schema is organized by module ownership (see ARCHITECTURE.md).
- A test database (`civora_test`) is expected for integration testing.
- Docker Compose provides a local PostgreSQL instance for development.

## References

- [ADR-0001: Initial Architecture — Modular Monolith](0001-initial-architecture.md)
- [PostgreSQL official site](https://www.postgresql.org/)
- [pgx driver](https://github.com/jackc/pgx)
