# CIVORA Audit Integrity Model

> **Status**: Implemented in v0.2
> This document describes the audit subsystem as it actually exists in the
> current codebase. It does not describe planned functionality.

## 1. Where audit events are stored

Audit events are stored in a single PostgreSQL table named `audit_events`,
defined in `migrations/0001_init.up.sql`. The table is **not** stored in a
separate database, separate schema, or separate storage system. It lives in
the same PostgreSQL instance and the same `public` schema as every other
CIVORA entity.

Table columns:

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | Primary key |
| `organization_id` | UUID | Foreign key to `organizations.id` |
| `actor_id` | UUID | Foreign key to `users.id` |
| `action` | TEXT | e.g. `case.created`, `case.transition` |
| `resource` | TEXT | e.g. `case`, `person`, `evidence` |
| `resource_id` | TEXT | String form of the affected resource UUID |
| `outcome` | TEXT | `success` or `failure` |
| `request_id` | TEXT | Correlation ID from the HTTP request |
| `metadata` | JSONB | Structured context (e.g. `{"from": "NEW", "to": "OPEN"}`) |
| `timestamp` | TIMESTAMPTZ | Event time |
| `hash` | TEXT | SHA-256 digest of the event content |
| `previous_hash` | TEXT | Hash of the preceding event for the same organization |

## 2. Transaction boundaries

Every audit write is part of the same database transaction as the state
change it records. The pattern is:

1. `database.InTransaction` begins a transaction.
2. The domain repository performs its state change (e.g. `UPDATE cases`).
3. `shared.RecordAuditEventInTx` writes the audit row **inside the same
   transaction**.
4. `tx.Commit()` commits both or rolls back both.

If the audit write fails, the entire transaction is rolled back and the
state change is not persisted. This is verified by
`TestAuditAtomicity_*` in `test/integration/integration_test.go`, which uses a
`failingAuditRepo` to prove that a case status change is not committed when
the audit write fails.

## 3. Integrity guarantees

### 3.1 Hash chain

Each audit event carries a SHA-256 hash computed over:

- organization ID
- actor ID
- action
- resource
- resource ID
- outcome
- request ID
- metadata (serialized as JSON)
- timestamp
- previous hash (the hash of the immediately preceding event for the same
  organization, or nil for the first event)

The hash chain is written inside the audit repository's transaction. The
previous hash is read from the database within the same transaction before
the new row is inserted, so concurrent writers cannot produce the same
previous hash.

### 3.2 What the hash chain provides

- **Tamper detection**: modifying any field of a past event breaks the hash
  chain. `AuditEvent.VerifyIntegrity()` recomputes the hash and compares it
  to the stored value.
- **Ordering**: because each event links to its predecessor, reordering or
  deleting events breaks the chain.
- **Detection in the API**: `internal/audit/api/handler.go` exposes
  `integrity_valid` on every event returned by the audit list endpoint.

### 3.3 What the hash chain does NOT provide

- **Non-repudiation**: the hash is computed by the same process that writes
  the data. A compromised application process can write a valid chain with
  false contents. The chain detects *accidental* or *offline* tampering with
  stored rows, not a malicious application.
- **Third-party verification**: there is no external anchor (e.g. a
  blockchain, append-only log, or external witness). Verification is
  possible only by re-reading the table and recomputing hashes.
- **Protection against database-level attacks**: if an attacker gains
  sufficient database privileges to insert or update rows directly, they
  can also update the hash chain consistently. The chain is a defense against
  accidental corruption and casual tampering, not against a database
  compromise.
- **Actor authentication**: the `actor_id` field is populated from the
  authenticated JWT subject. It does not independently prove who performed
  the action.

## 4. Tenant isolation

The `audit_events` table has a foreign key to `organizations.id`. Every audit
write is scoped to the organization of the event's subject. The
`RequireSameTenant` middleware ensures that the `orgId` path parameter matches
the JWT's `organization_id` claim before any handler runs, so an
authenticated user cannot read or write audit events for another
organization through the API.

The repository method `FindByOrganization` filters on
`organization_id = $1`.

## 5. What tamper resistance exists

- Hash chain with SHA-256, chained per organization.
- `integrity_valid` flag exposed in the audit API.
- All audit writes are inside the same transaction as the state change.

## 6. What is NOT guaranteed

- Audit events are not stored separately from business data.
- There is no append-only storage, no external verification, and no
  cryptographic anchoring to a third party.
- A database administrator with write access can modify audit rows and
  recompute the chain.
- Audit retention is configurable (`audit.retention_days`). The purge runs
  on startup and on every background maintenance tick
  (`internal/audit/infrastructure/maintenance.go`).

## 7. Audit verification

`AuditEvent.VerifyIntegrity()` recomputes the hash from the event's stored
fields and compares it to the stored hash. It is called:

- By the audit API handler for every event in a list response
  (`internal/audit/api/handler.go`).
- By the audit repository tests (`internal/audit/infrastructure/postgres/
  audit_repository_test.go`).
- By the audit maintenance service (`internal/audit/infrastructure/
  maintenance.go`), which runs `VerifyAllOrganizations` on startup and on
  every background tick. The server starts the background job in
  `cmd/civora/main.go`.

It is **not** called automatically on every read path outside the audit API.

## 8. Remaining limitations

1. **Single-table storage**: audit events share the same database and schema
   as business data. A schema migration that corrupts the `audit_events`
   table can corrupt both audit and business data.
2. **No export/archive**: there is no mechanism to export audit events to an
   append-only archive or WORM storage.
3. **No verification job**: hashes are not recomputed on a schedule.
4. **Actor ID is application-trusted**: the value comes from the JWT
   subject claim and is not independently verified against a second factor.
5. **Metadata is unstructured**: audit metadata is stored as JSONB. There is
   no schema enforcement on its contents, so an application bug can write
   sensitive data into audit metadata.