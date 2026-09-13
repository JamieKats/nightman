# Use GORM for queries only; hand-written versioned SQL for schema

## Decision

`internal/store` uses GORM (`gorm.io/gorm` + `gorm.io/driver/postgres`,
`gorm.io/datatypes` for the JSONB columns) for reads and writes only.
Schema is hand-written, versioned SQL under `migrations/*.sql`, applied
via `make migrate` (`psql`). GORM's `AutoMigrate` is never used.

## Context

The `requests` table needs two Postgres roles with different privileges
(`nightman_ingest` write, `nightman_ro` read-only — see
[0009](0009-separate-ingest-and-readonly-db-roles.md)), specific column
types (`INET`, `JSONB`), and indexes chosen deliberately. The developer is
already familiar with GORM.

## Alternatives

- **GORM `AutoMigrate` end to end.** Simplest, most GORM-native, lowest
  friction for a learning project. Rejected: it can't express role grants
  or `nightman_ro`, and auto-migrating a managed production database on
  every app restart is exactly the kind of unreviewed schema change the
  security-conscious design of this project argues against.
- **`pgx` + `sqlc`, no ORM.** Full control, arguably more idiomatic Go.
  Rejected in favor of GORM primarily because the developer already knows
  GORM and this is partly a learning project — optimizing for an
  unfamiliar toolchain wasn't the goal.
- **Hybrid: `AutoMigrate` in dev, versioned SQL in prod.** Fast local
  iteration, but two schema-definition paths to keep in sync. Rejected as
  not worth the drift risk for a project this size.

## Rationale

Versioned SQL is the only approach that can correctly express the two
least-privilege Postgres roles and keep schema changes reviewable — a
real resume-relevant practice for a security-themed project. GORM still
gets used where it adds value (struct-mapped reads/writes,
`CreateInBatches` for the async sink) without owning anything
security-sensitive.

## Consequences

- Two things to keep in sync by hand: the GORM model (`store.Request`)
  and the SQL DDL. Acceptable at this schema's size (one table).
- `internal/store`'s tests mock the DB layer (e.g. `go-sqlmock` behind
  GORM) rather than hitting real Postgres — see
  [0002's sibling deferral](../plans/persistence.md) and the
  testcontainers future improvement in
  [docs/plans/hardening-and-ci.md](../plans/hardening-and-ci.md).
- `make migrate` (not an ORM) is the only way schema changes reach a real
  database, in any environment.
