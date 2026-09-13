# One Go binary for v1, with services decoupled behind an interface

## Decision

Nightman ships as a single Go binary/process. Every mocked service
(Ollama, OpenAI, vLLM, Anthropic) implements the same `services.Service`
interface (`Name() string`, `Routes() http.Handler`) and knows nothing
about the others.

## Context

The brief's stated future direction is to split each mocked service into
its own container once the single-binary approach outgrows itself (e.g.
if isolating a compromised or heavily-hammered service becomes
important). Traffic volume and which services get hammered hardest are
both unknown before any real data is collected.

## Alternatives

- **Separate containers/processes per service from day one.** Matches
  the eventual target architecture exactly and gives per-service
  isolation immediately. Rejected for v1: meaningfully more to build and
  deploy (N containers, N sets of infra) before there's any evidence
  which services actually need that isolation — premature for traffic
  patterns that are still unknown.
- **One binary with services tightly coupled (shared internal state,
  direct calls between them).** Least code for v1. Rejected: it would
  make the brief's planned future split into a real rewrite rather than
  an extraction, defeating the stated purpose of even having that future
  direction in mind now.

## Rationale

A single binary is simplest to build, deploy, and log from while nothing
is yet known about real traffic or which service needs isolating first.
Keeping every service behind `services.Service` with no inter-service
knowledge means that decision can be revisited per-service later — lift
one out into its own container/process — without restructuring the
others.

## Consequences

- `internal/server` composes already-built `http.Handler`s per port; it
  has no special knowledge of any one service's internals.
- Splitting a service out later (the brief's explicit future direction)
  is additive: the interface doesn't change, only where `Routes()`'s
  handler actually runs.
