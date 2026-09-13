# Use two Postgres roles: write-only ingest, read-only dashboard

## Decision

Postgres has two roles: `nightman_ingest` (`INSERT`/`SELECT` on
`requests`, used by the capture pipeline) and `nightman_ro` (`SELECT`
only, used exclusively by the public Grafana dashboard). The dashboard
never has write access, and the ingest role is never exposed to Grafana.

## Context

Per the brief, the Grafana dashboard is intended to be **public** and
linked from a resume — a materially larger attack surface than an
internal tool. Raw per-record data (full bodies, headers, individual IPs)
must never be browsable there; only aggregates.

## Alternatives

- **One shared role for both ingestion and dashboard reads.** Simpler to
  provision. Rejected outright: a public-facing dashboard with write
  access to the capture table would let anyone who found a way into
  Grafana's datasource config (or an unpatched Grafana bug) modify or
  delete the evidence the whole project exists to collect.
- **Rely on application-level checks instead of DB roles** (e.g. the
  dashboard's queries are "supposed to" only ever `SELECT`). Rejected:
  enforcement at the database layer is structural — it holds even if a
  query is crafted to do otherwise — whereas an application-level
  convention is just a convention.

## Rationale

Least privilege, enforced where it can't be bypassed by a bug or a
misconfigured panel: the role itself, not application logic, is what
makes it *impossible* — not just discouraged — for the public dashboard
to write or exceed its own scope.

## Consequences

- Schema migrations (see
  [0002](0002-gorm-with-versioned-sql-migrations.md)) must create and
  grant both roles explicitly; there's no ORM shortcut for this.
- Any future panel or ad hoc query against the public dashboard is
  automatically constrained to read-only, with no extra review needed to
  confirm it can't mutate data.
