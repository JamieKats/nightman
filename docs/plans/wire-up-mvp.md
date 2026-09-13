# Finish wiring the MVP binary

## Goal

Make `go run ./cmd/nightman` actually serve the Ollama mock and log every
probe to stdout — the last piece of milestone M1. Every piece this needs
already exists and is tested in isolation; nothing new needs designing,
only assembling.

## Current architecture

See [docs/architecture/overview.md](../architecture/overview.md) for the
full request flow. Today, `cmd/nightman/main.go` still runs the original
placeholder: a single hardcoded `:8080` listener returning
`501 Not Implemented` for everything, with a `logging.New(os.Stdout, "info")`
logger that isn't driven by any config. `internal/config`,
`internal/capture`, `internal/server`, `internal/response`, and
`internal/services/ollama` are all built and tested, but nothing in
`main.go` calls any of them yet.

## Proposed design

`main` (or a `run` helper, as today) does, in order:

1. Parse a `-config` flag (default `configs/nightman.yaml`) and call
   `config.Load`.
2. Build the logger via `logging.New(os.Stdout, cfg.LogLevel)` — replacing
   today's hardcoded `"info"`.
3. Call `response.Load()` to get the template `Registry`.
4. Build `ollama.New(registry)`.
5. For each `cfg.Listeners` entry naming `"ollama"`, wrap
   `ollamaService.Routes()` in
   `capture.Middleware(capture.NewStdoutSink(logger), logger, maxBodyBytes, "ollama")`
   and add it to the `map[int]http.Handler` passed to `server.New`.
   (A listener naming `openai`/`vllm`/`anthropic` has nothing to mount
   yet — see [Open questions](#open-questions).)
6. Call `server.New(handlers, logger)`, then `Run(ctx, shutdownTimeout)`
   under the existing `signal.NotifyContext(SIGINT, SIGTERM)`.

`maxBodyBytes` and `shutdownTimeout` need concrete values — not yet in
`config.Config` (capture-queue size and SSE timing are, but not a body
cap or shutdown timeout). Simplest: hardcode them in `main` for now
(e.g. `1<<20` bytes, `10*time.Second`), matching the level of config
surface that actually exists today; promoting them to config is Phase 5's
"Config surface" step (see
[docs/plans/hardening-and-ci.md](hardening-and-ci.md)), not this one.

## Constraints

- No new packages, no new dependencies — this step is pure assembly.
- Must not regress graceful shutdown (already implemented in `main.go`
  and `server.Run`).
- `configs/nightman.yaml`'s checked-in example must be valid input to
  this wiring (i.e. its `listeners` should resolve to real services).

## Implementation Phases

1. Update `configs/nightman.yaml` if needed so its `listeners` only name
   `ollama` for now (since OpenAI/vLLM/Anthropic don't exist yet) —
   otherwise `config.Validate` would accept a service name that `main`
   can't actually resolve to a handler.
2. Rewrite `cmd/nightman/main.go`'s `run` to perform the six steps above.
3. Manual check: `go run ./cmd/nightman`, then `curl localhost:11434/api/tags`
   and confirm a valid Ollama-shaped JSON response, and a capture line on
   stdout.
4. `Ctrl-C` and confirm clean shutdown (no hung goroutines, process exits).

## Decisions

No new decisions — this step only assembles already-decided pieces. See
[0003](../decisions/0003-stdlib-http-router.md) (router),
[0004](../decisions/0004-v1-port-map.md) (port map), and
[0007](../decisions/0007-embedded-json-response-templates.md) (templates).

## Open questions

- What happens if `cfg.Listeners` names a service that has no real
  implementation yet (`openai`, `vllm`, `anthropic`)? Options: reject at
  startup (extend `config.Validate` to only accept `ollama` until the
  others land), or have `main` build a placeholder 501 handler for
  unimplemented services. Leaning toward the former (fail fast, matches
  the project's general posture) but not decided.
- Where `maxBodyBytes`/`shutdownTimeout` should eventually live in
  `config.Config` — deferred to Phase 5, noted above, not blocking this
  step.
