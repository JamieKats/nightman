# Service mocks

How a fake LLM service is built, and how it serves responses.

## The Service interface

```go
// internal/services
type Service interface {
    Name() string          // e.g. "ollama" — recorded on every captured request
    Routes() http.Handler  // this service's routes; mounted by internal/server
}
```

Each implementation lives in its own sub-package
(`internal/services/<name>`) and knows nothing about the others — see
[0011](../decisions/0011-single-binary-decoupled-services.md) for why.

## Response templates (`internal/response`)

Canned responses are embedded JSON files under
`internal/response/templates/<service>/<endpoint>/*.json`, grouped into
pools keyed `"<service>/<endpoint>"` (e.g. `"ollama/generate"`) — see
[0007](../decisions/0007-embedded-json-response-templates.md) for why
they're static files served byte-for-byte rather than a templating
engine.

- `response.Load() (*Registry, error)` walks the embedded filesystem,
  validates every file is JSON, and groups them by pool key. It fails
  (startup error) on invalid JSON, a template not nested exactly two
  levels deep, or finding nothing at all.
- `Registry.Pool(key string) (*Pool, error)` looks up one pool; a key
  nobody loaded is an error — callers resolve every pool they need once,
  at construction, so a missing pool is a startup failure rather than a
  surprise against a live probe.
- `Pool.Random() []byte` returns one template chosen uniformly at random.
  `math/rand/v2`'s global source auto-seeds from the OS per process, so
  nothing extra is needed to satisfy "seeded per-process."

## Ollama (`internal/services/ollama`) — implemented

Mocks `GET /api/tags`, `POST /api/show`, `POST /api/generate`,
`POST /api/chat`. `New(reg *response.Registry)` resolves all four pools
up front. `Routes()` registers Go 1.22+ method+path patterns on a plain
`http.ServeMux` — wrong method and unknown path get 405/404 for free from
the stdlib (see [0003](../decisions/0003-stdlib-http-router.md)).

Every route ignores the request body entirely and always returns a
random template with `Content-Type: application/json` — no per-request
customization, per
[0012](../decisions/0012-no-real-inference-ever.md) and
[0007](../decisions/0007-embedded-json-response-templates.md). v1 is
non-streaming only regardless of a probe's `"stream": true`; streaming
variants are tracked in
[docs/plans/response-breadth.md](../plans/response-breadth.md).

### Golden-file testing

`ollama_test.go` reads the real files under
`internal/response/templates/ollama/<endpoint>/` directly off disk (the
same files `response.Load` embeds) and asserts each route's response is
byte-for-byte one of them, plus that the parsed JSON has the expected
top-level key (`models`, `modelfile`, `response`, `message`). This avoids
a second, parallel `testdata/` fixture set that would just duplicate the
real templates.

## OpenAI, vLLM, Anthropic — not yet implemented

Tracked in [docs/plans/response-breadth.md](../plans/response-breadth.md).
They'll follow the same pattern: a `Service` implementation, a template
pool per endpoint, golden-file tests against the real template files.
