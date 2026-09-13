# Architecture overview

How Nightman works today. For *why* it's shaped this way, see
[docs/decisions/](../decisions/); for the original concept and
constraints, see [docs/PROJECT_BRIEF.md](../PROJECT_BRIEF.md); for what's
not built yet, see [docs/plans/](../plans/).

## The shape

Nightman is one Go binary (see
[0011](../decisions/0011-single-binary-decoupled-services.md)) that
listens on a small set of ports, mimics fake LLM-service APIs on them,
and logs every request it receives. Nothing in the process ever calls a
real model — see
[0012](../decisions/0012-no-real-inference-ever.md).

## Request flow

```
            ┌───────────────────────┐
 probe ───▶ │ internal/server        │  one net.Listener per configured
            │ (listener supervisor)  │  port, already bound before Run
            └───────────┬───────────┘  starts serving (see server.go)
                         ▼
            ┌───────────────────────┐
            │ internal/capture       │  times the request, builds a
            │ .Middleware            │  capture.Record, lets the real
            └───────────┬───────────┘  handler run, then captures
                         ▼
            ┌───────────────────────┐
            │ internal/services/*    │  e.g. Ollama: routes by method+path,
            │ .Service.Routes()      │  picks a random template, responds
            └───────────┬───────────┘
                         ▼
            ┌───────────────────────┐
            │ internal/response      │  embedded, byte-for-byte JSON
            │ .Pool.Random()         │  templates — no dynamic content
            └───────────────────────┘
                         │
          (back up through Middleware, which now knows the
           response status/latency and finishes the Record)
                         ▼
            ┌───────────────────────┐
            │ internal/capture.Sink  │  v1: StdoutSink (one JSON line).
            │                        │  Postgres sink: docs/plans/persistence.md
            └───────────────────────┘
```

Each mocked service is wrapped in its *own* `capture.Middleware` instance
at mount time (so the middleware knows which service matched without any
dynamic dispatch), then the resulting handler is registered onto whichever
port's `http.Handler` the listener supervisor serves. See
[service-mocks.md](service-mocks.md) for how a service is built and
[request-capture.md](request-capture.md) for what the middleware does.

## The listener supervisor (`internal/server`)

`server.New(handlers map[int]http.Handler, logger) (*Server, error)` binds
a `net.Listener` for every port **synchronously**, before returning — a
port already in use is reported immediately as a construction error, with
any listeners already bound cleaned up, rather than surfacing later once
`Run` starts serving. `Run(ctx, shutdownTimeout)` then serves every
listener until `ctx` is cancelled, and shuts all of them down together
within the given timeout.

`New` takes pre-built handlers, not `config.Config` directly — composing
multiple services onto a shared port (the `8080` case) is the caller's
job, done once real per-service paths exist (see
[0004](../decisions/0004-v1-port-map.md)).

## Package map

```
cmd/nightman/        binary entrypoint — wires everything together
internal/
  config/            YAML + NIGHTMAN_* env config loading     → configuration.md
  logging/           structured operational logging (log/slog)
  server/            multi-port listener supervisor            → this doc
  capture/           Record, RecordFromRequest, Middleware, Sink → request-capture.md
  response/          embedded, byte-for-byte template pools     → service-mocks.md
  services/          Service interface + per-service mocks      → service-mocks.md
    ollama/            implemented
    openai/ vllm/ anthropic/  not yet implemented — docs/plans/response-breadth.md
  store/             PostgreSQL persistence (GORM)             — not yet implemented
  ratelimit/         per-IP rate limiting                      — not yet implemented
migrations/          hand-written versioned SQL
configs/             example configuration
deployments/         Terraform (DigitalOcean)
```

## Current status

Built and tested: `internal/config`, `internal/logging`,
`internal/capture`, `internal/server`, `internal/response`,
`internal/services/ollama`. Not yet wired into `cmd/nightman` — that's
the only remaining piece of M1, tracked in
[docs/plans/wire-up-mvp.md](../plans/wire-up-mvp.md). Everything past M1
(persistence, the other three service mocks, hardening, deployment, the
dashboard) is tracked one plan doc per phase under
[docs/plans/](../plans/).
