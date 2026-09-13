# Deployment to DigitalOcean

## Goal

Get a hardened Nightman binary running on real internet-facing
infrastructure, isolated per the brief's constraints, provisioned as
code. This is milestone M5. Assumes M4 (hardening/CI) is done.

## Current architecture

`deployments/terraform` is an empty directory. There's no Dockerfile,
no CI container build, no infra of any kind yet — everything so far runs
locally via `go run`/`make run`.

## Proposed design

**Container** (`build/package` + a Dockerfile): multi-stage build,
static binary on `scratch`/distroless.

**Terraform** (`deployments/terraform`), targeting DigitalOcean — see
[0010](../decisions/0010-digitalocean-over-aws.md):

- **Network**: VPC + private subnet.
- **Managed Postgres**: cluster with the `nightman_ingest` and
  `nightman_ro` roles (see
  [0008](../decisions/0008-managed-postgres-over-self-hosted.md),
  [0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md)),
  trusted-sources firewall limiting inbound to the Droplet and Grafana
  only.
- **Compute**: Droplet(s) with cloud-init to pull and run the container,
  attached to the VPC, no public egress beyond what's needed to pull the
  image.
- **Managed Load Balancer**: public entry point, TLS via DigitalOcean's
  Let's Encrypt integration, forwarding the impersonated
  ports/paths — see [0004](../decisions/0004-v1-port-map.md) — to the
  Droplet over the private network. Inbound firewall: only the
  impersonated ports.
- **Egress lockdown**: deny-by-default outbound firewall on the Droplet.

**Secrets + runbook**: DB credentials via DigitalOcean project env /
Terraform variables, never in the repo. A deploy + rollback runbook
alongside this doc.

## Constraints

- No real secrets or functioning credentials anywhere in the honeypot —
  nothing a caller who "succeeds" against it could mistake for access to
  something real.
- Inbound security-group rules scoped to exactly the impersonated ports
  (see [0004](../decisions/0004-v1-port-map.md)) — nothing else.
- Egress deny-by-default: a compromised container must not be able to
  pivot or attack others.
- Grafana's datasource connects with `nightman_ro` only, never
  `nightman_ingest` — enforced at the database layer
  ([0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md)),
  not just by Terraform config.

## Implementation Phases

1. Dockerfile + multi-stage build producing a static binary image.
2. Terraform: VPC + private subnet.
3. Terraform: Managed Postgres + both roles + trusted-sources firewall.
4. Terraform: Droplet(s) + cloud-init.
5. Terraform: Managed Load Balancer + TLS + inbound firewall.
6. Egress lockdown on the Droplet.
7. Secrets wiring (DO project env / Terraform variables) + runbook.

## Decisions

[0008](../decisions/0008-managed-postgres-over-self-hosted.md),
[0009](../decisions/0009-separate-ingest-and-readonly-db-roles.md),
[0010](../decisions/0010-digitalocean-over-aws.md).

## Open questions

- Exact Terraform module layout (one flat config vs. modules per
  concern — network/db/compute/lb) — not yet decided.
- Cloud-provider agnosticism is architectural today (no DigitalOcean SDK
  calls in application code) but not yet *enforced* as a rule, and
  there's no second Terraform target proving it. Both are explicitly
  deferred until after v0.1 is running on DigitalOcean — see
  [0010](../decisions/0010-digitalocean-over-aws.md)'s consequences.
- Whether `doctl` is used alongside Terraform for anything (the brief
  mentions both) — not yet decided; Terraform alone may cover everything
  needed.
