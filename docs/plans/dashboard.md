# Public Grafana dashboard

## Goal

A public, live dashboard of aggregate honeypot statistics, linkable from
a resume — without ever exposing raw per-record data or the full Grafana
instance. This is milestone M6, the last one.

## Current architecture

No Grafana provisioning exists yet. This phase assumes
[docs/plans/persistence.md](persistence.md) (real data in Postgres) and
[docs/plans/deployment.md](deployment.md) (Grafana running somewhere
reachable) are both done.

## Proposed design

**Provisioning as code**: Grafana's datasource is pinned to the
`nightman_ro` role (see
[0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md)) —
never `nightman_ingest`. Dashboards defined as JSON in the repo, not
clicked together and left unversioned.

**Panels** (aggregates/counts only, never raw bodies/headers/individual
IPs in browsable form):
- Requests over time, overall and per mocked service.
- Breakdown of hits by endpoint.
- Top fake models requested.
- Unique source IPs per day/week, as a count — not a list.
- Top User-Agent strings, aggregated/grouped.
- Response latency / connection-duration stats.
- Running total of requests collected since launch.

**Public sharing**: Grafana's built-in public-dashboard feature, scoped
to the specific dashboard — the full Grafana login is never exposed
publicly.

## Constraints

- Every panel shows aggregates/counts only — per the brief, source IPs
  are bucketed/counted, never listed individually on a public panel.
- The public-facing datasource connection must be structurally incapable
  of writing or reaching beyond what the dashboard needs — enforced by
  the `nightman_ro` role, not by which queries a panel happens to run.

## Implementation Phases

1. Grafana datasource provisioning (as code), pinned to `nightman_ro`.
2. Build and commit the panel set above as versioned dashboard JSON.
3. Enable Grafana's public-dashboard sharing for that dashboard only.
4. Link it from the resume / README.

## Decisions

[0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md) (the
read-only role this entire phase depends on).

## Open questions

- Country-level IP geolocation map panel — explicitly deferred in the
  brief until a geolocation lookup step (e.g. MaxMind GeoLite2) is added
  to the capture pipeline. Not part of this phase's initial panel set.
- Whether dashboard JSON lives under `deployments/` alongside Terraform
  or gets its own top-level directory (e.g. `grafana/`) — not yet
  decided.
