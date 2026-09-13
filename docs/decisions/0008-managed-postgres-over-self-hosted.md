# Use a managed Postgres instance, not self-hosted on the app box

## Decision

Run Postgres as a DigitalOcean Managed Database rather than self-hosting
it (in a container or otherwise) on the same instance(s) running the
honeypot binary.

## Context

The honeypot's compute host is, by design, the thing internet scanners
and bots are actively probing and attacking. The brief's "isolate hard"
principle requires that a compromised container can't pivot or reach
anything beyond what it needs.

## Alternatives

- **Self-host Postgres in a container alongside the app, on the same
  instance.** Cheaper (no managed-DB line item) and simpler networking
  (no separate service to provision). Rejected: colocating the database
  with the adversarial-facing host means a compromised honeypot process
  has *local* access to the data regardless of any database-level role
  restrictions — the network isolation that makes the two-role split
  (see [0009](0009-separate-ingest-and-readonly-db-roles.md))
  meaningful disappears entirely if both live on the same box.
- **Self-host Postgres on a dedicated second instance (not managed,
  not colocated).** Preserves the isolation property but still leaves
  backups, patching, and HA to the operator. Rejected mainly on
  cost-benefit: a managed instance gets the same isolation plus
  operational features for a modest price difference (see the DO-vs-AWS
  cost comparison behind [0010](0010-digitalocean-over-aws.md)).

## Rationale

The isolation property — not operational convenience — is the deciding
factor: the database must sit on its own network boundary, reachable
only through the scoped roles, so that owning the honeypot host doesn't
automatically mean owning the data. A managed instance gets this for
free alongside backups/patching/metrics.

## Consequences

- Recurring cost (~$15/mo on DigitalOcean) instead of "free" colocation —
  judged worth it for the isolation guarantee on a project whose whole
  premise is defensive security.
- Database access is governed entirely by the two Postgres roles and
  DigitalOcean's trusted-sources firewall, not by anything the app
  process itself enforces.
