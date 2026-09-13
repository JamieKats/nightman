# Configuration

`internal/config` loads and validates Nightman's runtime configuration.
See [0001](../decisions/0001-yaml-config-with-env-overrides.md) for why
this is a YAML file with env overrides rather than env-only or JSON.

## Shape

```go
type Config struct {
    LogLevel  string
    Listeners []Listener  // Port int; Services []string
    RateLimit RateLimit   // RequestsPerSecond float64; Burst int
    Capture   Capture     // QueueSize int
    Streaming Streaming   // TokenIntervalMS int
    Database  Database    // DSN string
}
```

`Listener.Services` names must be one of the known mocked services
(`ollama`, `openai`, `vllm`, `anthropic` — kept in sync with
`internal/services`' sub-packages). `Streaming.TokenInterval()` converts
`TokenIntervalMS` to a `time.Duration` for callers.

## Load

`config.Load(path string) (*Config, error)`:

1. Reads the file at `path` and unmarshals it strictly (`goccy/go-yaml`
   with `yaml.Strict()` — unknown or duplicate keys are parse errors, not
   silently ignored typos).
2. Applies `NIGHTMAN_*` environment overrides
   (`NIGHTMAN_DB_DSN` → `Database.DSN`, `NIGHTMAN_LOG_LEVEL` → `LogLevel`)
   — these win over whatever the file says.
3. Fills defaults for anything still zero-valued (`LogLevel` → `"info"`,
   `RateLimit` → `{1, 5}`, `Capture.QueueSize` → `1000`,
   `Streaming.TokenIntervalMS` → `40`). A value the file set explicitly,
   even an invalid one, is never overwritten — that's `Validate`'s job to
   reject.
4. Calls `Validate()` and returns its error, wrapped with the file path,
   if anything is wrong.

Every error is scoped to the offending key (e.g.
`listeners[1] (port 8080): unknown service "bogus"`), so a misconfigured
deploy fails with an actionable message rather than a generic parse
error.

## Validate

Rejects: an unrecognized `LogLevel`; zero listeners; a port outside
1–65535; two listeners claiming the same port; a listener with no
services or a duplicated service name; an unrecognized service name;
`RequestsPerSecond <= 0`; `Burst < 1`; `Capture.QueueSize < 1`;
`Streaming.TokenIntervalMS < 0`; an empty `Database.DSN`.

## Example file

See `configs/nightman.yaml` — checked in with an empty `dsn` and a
comment pointing at `NIGHTMAN_DB_DSN`. It must never contain a real
secret.

## Tests

`internal/config/config_test.go` is table-driven: one valid full config,
one minimal config proving defaults apply, env-override precedence (both
when the file sets a value and when it doesn't), and one case per
`Validate` rejection reason above, plus missing-file and empty-path
cases.
