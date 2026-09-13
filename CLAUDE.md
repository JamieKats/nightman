# CLAUDE.md

Working rules for this repo. Context: `docs/PROJECT_BRIEF.md` (what/why),
`docs/architecture/` (how it works today), `docs/plans/` (what's next,
one doc per remaining phase), and `docs/decisions/` (settled technical
decisions, ADR-style).

Nightman is an AI honeypot: it serves fake LLM-service endpoints to
internet scanners and logs every request. No real inference, anywhere.

## Build, test, run

| Task | Command |
|------|---------|
| Build | `go build ./...` |
| Test (verbose) | `go test ./... -v` |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run` |
| Format | `gofmt -l -w .` |
| Local Postgres + Grafana | `docker-compose up -d` |
| Apply migrations | `make migrate` (runs `migrations/*.sql` through `psql`) |

Run locally (config loading is Phase 1, persistence Phase 3 — names below
are fixed by the implementation plan):

```sh
NIGHTMAN_DB_DSN='postgres://nightman_ingest:nightman@localhost:5432/nightman?sslmode=disable' \
  go run ./cmd/nightman -config configs/nightman.yaml
```

- Config: YAML at `configs/nightman.yaml`; every secret is overridable via
  a `NIGHTMAN_*` env var (`NIGHTMAN_DB_DSN` is the DSN).
- `Makefile` wraps the common targets; the commands above are the source
  of truth. `make help` lists targets.

## Project layout

```
cmd/nightman/        binary entrypoint
internal/
  config/            YAML + NIGHTMAN_* env config loading
  server/            routing layer; owns the request-capture pipeline
  services/          mocked LLM services, one sub-package each:
    ollama/  openai/  vllm/  anthropic/
  response/          embedded canned response templates (embed.FS)
  capture/           per-request record model
  store/             PostgreSQL persistence (GORM, queries only)
  ratelimit/         per-IP rate limiting
migrations/          hand-written versioned SQL (NNNN_name.sql)
configs/             example configuration
deployments/         Terraform (DigitalOcean)
docs/                project brief + implementation plan
```

New mocked-service handlers go in `internal/services/<name>/`, following
the pattern in `internal/services/ollama/`. They implement
`services.Service` and are mounted by `internal/server` — never wired up
ad hoc.

## Testing conventions

- Tests live in `_test.go` files next to the code they cover.
- Table-driven tests (idiomatic Go).
- Every code path that handles an incoming request (untrusted data) must
  have test cases.
- Integration tests mock the DB layer (e.g. `go-sqlmock` behind GORM).
  Real-Postgres via testcontainers is a future CI/CD improvement — see the
  implementation plan.

## Linting & formatting

- `golangci-lint run` must pass before any change is considered done.
- Code is `gofmt`-clean.

## Code style

- No abstraction without 2+ real implementations — plain function over
  interface until then.
- No clever one-liners where a few plain lines read clearer.
- Comments explain *why*, not *what*.
- Standard Go naming conventions.

## Error handling & logging

- Wrap errors: `fmt.Errorf("doing x: %w", err)`. Never swallow an error.
- No `panic` in request-handling paths — a panicking handler is a DoS
  vector against an adversarial input source. Recover-and-log at the
  server boundary; handlers return errors.
- Structured logging via `log/slog` only.

## Nightman-specific rules

- NEVER proxy to, or call, a real LLM — not in prod, not in tests, not
  ever. All responses are static or templated.
- Never hardcode credentials. Never commit `.env` or any secret.
- Grafana connects to Postgres with the read-only role (`nightman_ro`),
  never the write/ingest role.
- Every endpoint goes through the shared request-capture pipeline in
  `internal/server`. No handler skips request logging.

## Security

- All request data is hostile input. Validate and sanitize every field;
  never execute, shell out with, or interpolate untrusted data.
- Actively guard against the OWASP Top 10 classes (injection, SSRF,
  broken auth, misconfiguration, etc.).

## Dependencies

- Prefer the standard library where reasonable.
- Ask before adding any new third-party dependency.

## Never do without asking

- Modify infra / Terraform / DB migrations that touch production.
- Run destructive commands: `DROP TABLE`, `git push --force`, `rm -rf` of
  tracked paths, truncating tables, etc.
