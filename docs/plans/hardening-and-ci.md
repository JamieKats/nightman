# Hardening and CI

## Goal

Make the honeypot safe to expose to real adversarial traffic at scale:
rate limiting, abuse caps, a proper config surface, metrics, a real test
suite, and CI enforcing all of it. This is milestone M4.

## Current architecture

`internal/ratelimit` is an empty package. There's no request-size/header
cap beyond `capture.RecordFromRequest`'s body truncation (see
[docs/architecture/request-capture.md](../architecture/request-capture.md)).
`config.Config` covers listeners, rate-limit numbers, capture queue size,
and SSE timing, but not per-service enable/disable or per-route template
overrides. There's no metrics endpoint and no CI workflow yet —
`golangci-lint run ./...` and `go test ./...` are currently run by hand.

## Proposed design

**Rate limiting** (`internal/ratelimit`): in-memory token bucket keyed by
source IP (`golang.org/x/time/rate` + an expiring map) — see
[0006](../decisions/0006-in-memory-rate-limiting.md). Applied as another
middleware layer alongside `capture.Middleware`; over-limit responds
`429` with a plausible body and increments a rate-limited metric.

**Abuse caps**: max header count/size and read/write timeouts on the
`http.Server`s `internal/server` builds (body size is already capped via
`config` → `capture.RecordFromRequest`'s `maxBodyBytes`, currently
hardcoded in `main` pending
[docs/plans/wire-up-mvp.md](wire-up-mvp.md)'s open question about where
that value should live).

**Config surface**: extend `config.Config` so each service can be
enabled/disabled independently, and per-route template-pool overrides
become possible (useful once there are ~10 templates/endpoint and a
reason to weight selection). Document every key.

**Metrics**: a Prometheus endpoint on a private (non-impersonated) port —
request counts by service/status, capture-queue depth, dropped-record
count (ties to [0005](../decisions/0005-lossy-capture-on-db-outage.md)),
rate-limit hits.

**Test suite**: fill remaining gaps — every request-handling path needs
test cases (already true for what's built; keep it true going forward),
store tests against a mocked DB layer (no real Postgres required), a
meaningful coverage floor enforced in CI.

**CI**: GitHub Actions running `golangci-lint run`, `go test ./...`,
`go build ./...` on every push/PR — the same three gates already run by
hand for every change in this project. Optionally build + push a
container image. Real-Postgres integration via testcontainers is an
explicit future addition once the mocked-DB suite is stable (not part of
this phase's initial CI).

## Constraints

- Rate limiting and abuse caps must never affect *legitimate* probe
  responses below the limit — only over-limit traffic sees different
  behavior.
- CI must enforce exactly the gates already required locally (see
  CLAUDE.md's "Linting & formatting" and "Testing conventions") — no new
  rules invented here, just automation of existing ones.

## Implementation Phases

1. `internal/ratelimit` implementation + tests; wire as middleware.
2. Request abuse caps on `internal/server`'s `http.Server`s.
3. Extend `config.Config` for per-service enable/disable and template
   overrides; document every key.
4. Metrics endpoint on a private port.
5. Close out remaining test-coverage gaps.
6. GitHub Actions workflow (lint, test, build); consider a coverage
   floor.

## Decisions

[0006](../decisions/0006-in-memory-rate-limiting.md) (in-memory rate
limiting). CI timing (run from early in the project, formalized here) was
folded into this plan rather than getting its own ADR — it's a scheduling
choice, not an architectural one.

## Open questions

- Exact coverage floor number for CI — not yet decided; will likely be
  set empirically once the suite is more complete.
- Whether metrics needs its own config section or reuses existing
  listener config (a metrics port is, structurally, just another
  listener) — leaning toward the latter, not decided.
