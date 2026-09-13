# Use the standard library's net/http mux, not a third-party router

## Decision

Route requests with Go's standard `net/http.ServeMux` (Go 1.22+'s
method + path pattern syntax: `"GET /api/tags"`), not a third-party
router like `chi`.

## Context

Each mocked service (`internal/services/<name>`) needs a handful of
fixed routes with method matching and automatic 404/405 behavior. The
project's dependency policy prefers the standard library where
reasonable.

## Alternatives

- **`chi`.** Popular, well-tested, nicer middleware composition and
  sub-router ergonomics for larger route sets. Rejected for v1: the
  stdlib mux (with Go 1.22's pattern matching) already covers everything
  needed — exact-path registration, per-method routes, free 404/405 —
  and adding a router dependency for routes this simple isn't justified
  yet.

## Rationale

Nothing about the current route sets (Ollama's 4 routes; OpenAI/vLLM/
Anthropic's similarly small sets) needs `chi`'s extra features. The
stdlib mux keeps the dependency list smaller and the routing logic
directly inspectable.

## Consequences

- `internal/server.Server` composes per-service handlers itself (see
  [docs/architecture/overview.md](../architecture/overview.md)) rather
  than relying on a router's sub-mounting helpers.
- Revisit if/when route composition across many services on one port
  gets unwieldy — tracked as a post-v1 experiment in
  [docs/plans/response-breadth.md](../plans/response-breadth.md).
