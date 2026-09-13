# Persistence: Postgres-backed capture

## Goal

Replace `StdoutSink` with a real, durable capture pipeline: probes land
in a managed Postgres `requests` table, without request handling ever
blocking on the database. This is milestone M2.

## Current architecture

See [docs/architecture/request-capture.md](../architecture/request-capture.md).
`capture.Sink` is already the seam this plugs into — nothing about
`Record`, `Middleware`, or the `Sink` interface needs to change.
`migrations/0001_init.sql` is currently a placeholder; `internal/store`
and `internal/ratelimit` are empty packages; there's no `docker-compose.yml`
or `Makefile` `migrate`/`up` target with real commands yet.

## Proposed design

**Schema** (`migrations/`, hand-written SQL — see
[0002](../decisions/0002-gorm-with-versioned-sql-migrations.md)):
- `0001_init.sql`: real DDL for `requests` — structured columns
  matching `capture.Record` (minus `ParsedBody`, which becomes the
  `body` JSONB column), plus indexes on `ts`, `source_ip`, `service`.
- `0002_roles.sql`: `nightman_ingest` (INSERT/SELECT) and `nightman_ro`
  (SELECT only) — see
  [0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md).
- Applied via a `make migrate` target running the ordered `.sql` files
  through `psql`.

**Store** (`internal/store`): a GORM model `Request` mapped to the
`requests` table (`datatypes.JSON` for `headers`/`body`), opened with the
`nightman_ingest` DSN. `Insert(ctx, capture.Record)` and
`InsertBatch(ctx, []capture.Record)` (via `CreateInBatches`); `Ping` for
health checks. GORM never touches schema — see
[0002](../decisions/0002-gorm-with-versioned-sql-migrations.md).

**Async sink**: a new `capture.Sink` implementation wrapping a bounded
channel + background writer that batches `InsertBatch` calls (size/time
flush). On queue overflow or DB outage: drop the record, increment a
counter, and fall back to `StdoutSink`'s behavior for that one record —
see [0005](../decisions/0005-lossy-capture-on-db-outage.md). The request
path only ever pushes to the channel; it never waits on the database.

**Local dev**: `docker-compose.yml` with Postgres (and Grafana, for
`docs/plans/dashboard.md`). `Makefile` targets `make up`, `make migrate`,
`make run` get real implementations (currently `TODO` stubs). A "Running
locally" section in the README.

## Constraints

- Request latency must be provably unaffected by database latency/outage
  (this is the whole point of the async sink).
- No ORM auto-migration against a real database, ever (restated from
  [0002](../decisions/0002-gorm-with-versioned-sql-migrations.md)).
- `internal/store`'s own tests mock the DB layer (e.g. `go-sqlmock`
  behind GORM) — no real Postgres required to run `go test ./...`. Real
  Postgres integration via testcontainers is deferred, see
  [docs/plans/hardening-and-ci.md](hardening-and-ci.md).

## Implementation Phases

1. Write real `0001_init.sql` DDL + `0002_roles.sql`; wire `make migrate`.
2. Implement `internal/store` (GORM model, `Insert`/`InsertBatch`/`Ping`),
   tested against a mocked DB layer.
3. Implement the async capture sink (bounded channel, batching,
   overflow/outage fallback), with a test proving request latency is
   decoupled from DB latency and the overflow path is exercised.
4. `docker-compose.yml` + real `Makefile` targets + README "Running
   locally" section.
5. Wire the new sink into `cmd/nightman` behind a config choice of
   stdout vs Postgres sink; confirm probes land in `requests` end to end.

## Decisions

[0002](../decisions/0002-gorm-with-versioned-sql-migrations.md) (GORM +
versioned SQL), [0005](../decisions/0005-lossy-capture-on-db-outage.md)
(lossy capture), [0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md)
(two roles).

## Open questions

- Exact batch-flush tuning (size vs time threshold) for the async
  writer — will likely need adjusting once there's real traffic to
  observe; not blocking an initial implementation.
- Whether the stdout/Postgres sink choice belongs in `config.Config` as
  its own field, or is inferred from whether a DSN is set. Leaning toward
  an explicit field (clearer intent, easier to test), not yet decided.
