# Rate limit per-IP in-memory, not via a shared store

## Decision

Per-IP rate limiting (step 21 of the honeypot build) uses an in-memory
token bucket keyed by source IP (`golang.org/x/time/rate` + an expiring
map) inside the single Go process. No Redis or other shared store.

## Context

The brief requires per-IP rate limiting on every endpoint so the honeypot
itself can't be abused as a traffic amplifier. Nightman runs as one
process for v1 (see
[0011](0011-single-binary-decoupled-services.md)).

## Alternatives

- **Redis-backed shared rate limiter.** Needed if/when the mocked
  services split into separate processes/containers and need a
  consistent view of per-IP limits across them. Rejected for v1: adds a
  new infra dependency (another managed/self-hosted store) for a
  guarantee — shared state across processes — that doesn't exist yet,
  since everything runs in one process.

## Rationale

A single process already has a consistent, in-memory view of every
request, so in-process state is sufficient and avoids standing up
infrastructure purely to coordinate something that isn't distributed.

## Consequences

- Rate-limit state resets on process restart — acceptable for a honeypot
  (worst case, a brief window of looser limiting after a deploy).
- Revisit once/if a mocked service is lifted into its own container (the
  brief's stated future direction) — at that point, per-IP limits would
  need to be shared across processes to stay meaningful.
