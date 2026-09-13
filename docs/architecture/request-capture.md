# Request capture

`internal/capture` is how Nightman fulfills "log everything, act on
nothing." It has three parts: the `Record` data model, the `Middleware`
that builds one per request, and the `Sink` that receives it.

## Record and RecordFromRequest

`Record` mirrors the `requests` table (`migrations/0001_init.sql`):
timestamp, source IP/port, method, path, headers, raw body, best-effort
JSON-parsed body, a truncation flag, a request ID, the matched service
name, and — filled in later, after the handler runs — response status,
latency, and connection duration.

`RecordFromRequest(r *http.Request, maxBodyBytes int64) (Record, error)`
extracts everything knowable from the incoming request:

- **Size-capped, never rejected.** Reads up to `maxBodyBytes`; if there
  was more, it sets `Truncated = true` and keeps exactly `maxBodyBytes` —
  an oversized body is never a reason to fail the request.
- **Best-effort JSON parse.** `RawBody` always holds what was actually
  sent; `ParsedBody` is `nil` if it wasn't valid JSON. A probe sending
  garbage is still logged, just without a parsed form.
- **The body is re-buffered onto `r.Body`** after reading, so the real
  handler that runs after capture still sees the full (or truncated)
  body — this is what makes the "capture the body, then still serve
  normally" ordering work.
- **Only two error cases**, neither attacker-controlled: a genuine
  body-read I/O error, or an unparsable `RemoteAddr` (not possible from a
  real accepted connection — only from a test deliberately setting a bad
  one).

## Middleware

`capture.Middleware(sink, logger, maxBodyBytes, service) func(http.Handler) http.Handler`
wraps one handler. `service` is fixed at construction (e.g. `"ollama"`),
not inferred dynamically — each mocked service gets wrapped individually
before being mounted, so there's no need to guess which handler matched
after the fact.

Per request: generate a request ID (`internal/logging.NewRequestID`),
call `RecordFromRequest`, serve the wrapped handler through a
`statusWriter` (captures the actual status code, including the implicit
200 when a handler only calls `Write`), then fill in
`ResponseStatus`/`LatencyMS`/`ConnectionDurationMS` and hand the finished
`Record` to the `Sink`.

**A capture-layer failure never changes what the probe sees.** If
`RecordFromRequest` errors, the wrapped handler still runs and responds
exactly as it would have — only that one probe goes uncaptured (logged as
an operational error via `logging.WithRequestID`). Letting a capture
hiccup turn into e.g. a different response code would itself be a
fingerprinting signal.

`ConnectionDurationMS` equals `LatencyMS` for now — they diverge once
streaming responses exist (see
[docs/plans/response-breadth.md](../plans/response-breadth.md)).

## Sink

```go
type Sink interface {
    Capture(rec Record)
}
```

No error return, no context: a `Sink` must never block or fail the
request path (see
[0005](../decisions/0005-lossy-capture-on-db-outage.md) for what that
means once the real store exists).

`StdoutSink` is the v1 implementation — one structured JSON log line per
`Record`, mirroring the `requests` table's columns. The Postgres-backed
sink (async, batched, drop-on-backpressure) lands in
[docs/plans/persistence.md](../plans/persistence.md).

## Request IDs

`internal/logging.NewRequestID()` generates a short random hex ID;
`WithRequestID` tags a logger with it. `Middleware` uses both so a
probe's capture line and any operational error logged about that same
request can be cross-referenced by `request_id`.
