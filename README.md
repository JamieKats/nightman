# Nightman

AI honeypot for LLM-scanning bots. Nightman exposes fake, internet-facing
endpoints that mimic common self-hosted and API-based LLM services (Ollama,
OpenAI-compatible, vLLM, Anthropic-style), attracts unsolicited traffic from
scanners and bots probing for exposed LLM infrastructure, and logs every
request for analysis.

**No real inference happens anywhere in the system.** Every response is
static or templated — just convincing enough to keep an automated client
engaged, never enough to fool a human or generate real content.

## Status

Config loading, logging, the request-capture pipeline, the listener
supervisor, the embedded response-template system, and the Ollama mock
are built and tested. Not yet wired into the binary itself — see
[docs/plans/wire-up-mvp.md](docs/plans/wire-up-mvp.md), the last step
before the honeypot actually runs end to end.

## Documentation

- [docs/PROJECT_BRIEF.md](docs/PROJECT_BRIEF.md) — the original concept,
  infrastructure, and design principles.
- [docs/architecture/](docs/architecture/) — how the system works today,
  updated as code changes. Start at
  [overview.md](docs/architecture/overview.md).
- [docs/plans/](docs/plans/) — how it's going to change, one doc per
  remaining phase (Goal / Current architecture / Proposed design /
  Constraints / Implementation Phases / Decisions / Open questions).
- [docs/decisions/](docs/decisions/) — ADR-style records of significant
  technical decisions: context, alternatives considered, rationale,
  consequences.

## Layout

Follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout).

```
cmd/nightman/        binary entrypoint
internal/
  config/            YAML + env config loading
  logging/           structured operational logging (log/slog)
  server/            routing layer (port/path -> service)
  services/          Service interface + per-service mocks
    ollama/  openai/  vllm/  anthropic/
  response/          canned template responses (embedded)
  capture/           per-request logging model
  store/             PostgreSQL persistence (GORM)
  ratelimit/         per-IP rate limiting
migrations/          hand-written versioned SQL
configs/             example configuration
deployments/         Terraform (DigitalOcean)
```

## Tech stack

Go · PostgreSQL (JSONB) · GORM · Grafana · DigitalOcean · Terraform

## Design decisions

The non-obvious calls made on this project, with full context/alternatives/
rationale, now live in [docs/decisions/](docs/decisions/) as individual
ADRs — good resume talking points:

- [Managed Postgres, not self-hosted](docs/decisions/0008-managed-postgres-over-self-hosted.md)
- [Separate ingest (write) and dashboard (read-only) DB roles](docs/decisions/0009-separate-ingest-and-readonly-db-roles.md)
- [DigitalOcean over AWS](docs/decisions/0010-digitalocean-over-aws.md)
- [Single Go binary, internally decoupled services](docs/decisions/0011-single-binary-decoupled-services.md)
- [GORM for queries, hand-written versioned SQL for schema](docs/decisions/0002-gorm-with-versioned-sql-migrations.md)
- [No real inference, ever](docs/decisions/0012-no-real-inference-ever.md)
- [In-memory, per-process rate limiting (v1)](docs/decisions/0006-in-memory-rate-limiting.md)

See [docs/decisions/](docs/decisions/) for the complete list, including
the smaller build-tooling choices (config format, router, port map,
capture durability, template storage).

## Prerequisites

- **Go** — version per [go.mod](go.mod) (currently 1.25).
- **golangci-lint** — the linter. `golangci-lint run` must pass before any
  change is considered done (see [CLAUDE.md](CLAUDE.md)).

  ```sh
  # installs into $(go env GOPATH)/bin — make sure that's on your PATH
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
    | sh -s -- -b "$(go env GOPATH)/bin"
  golangci-lint --version
  ```

- **Docker + Docker Compose** — for the local Postgres and Grafana
  containers (needed from the persistence milestone onward).
- **psql** — PostgreSQL client, used by `make migrate` to apply
  `migrations/*.sql` (persistence milestone onward).

Go module dependencies are fetched automatically by `go build` / `go test`;
run `go mod download` to pre-fetch them.

## Building

```sh
go build ./...
```

Local run instructions (Docker Compose, migrations, config) will land with
the persistence milestone — see
[docs/plans/persistence.md](docs/plans/persistence.md).
