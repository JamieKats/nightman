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

Early scaffolding. The project layout and build plan are in place; service
handlers and the capture pipeline are not implemented yet.

## Documentation

- [docs/PROJECT_BRIEF.md](docs/PROJECT_BRIEF.md) — concept, architecture,
  infrastructure, and design principles.
- [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) — step-by-step
  build order, milestones, and settled build decisions.

## Layout

Follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout).

```
cmd/nightman/        binary entrypoint
internal/
  config/            YAML + env config loading
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

A running log of the non-obvious calls made on this project and why. See
[docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md#decisions) for
the full build-decision table.

- **Managed Postgres, not self-hosted.** The app host is the thing
  scanners and bots are actively attacking. Colocating the database on
  that box would let a compromised honeypot process reach the data
  locally regardless of DB-level role restrictions. A managed instance
  puts the DB on its own network boundary, reachable only through the
  roles below.
- **Two Postgres roles: `nightman_ingest` (write) and `nightman_ro`
  (read-only).** The Grafana dashboard is public; the ingestion service
  isn't. Separate least-privilege roles make it structurally impossible
  for the public-facing side to modify or exceed what it needs, rather
  than relying on application logic to enforce that.
- **DigitalOcean over AWS.** Equivalent AWS footprint (EC2 + ALB + RDS,
  no NAT Gateway) runs noticeably more expensive than DO's bundled
  Droplet + Load Balancer + Managed Database pricing for a project this
  small, funded out of pocket. The architecture stays cloud-agnostic on
  purpose (standard Postgres, stdlib HTTP, provider-specific pieces
  isolated in Terraform) so a later AWS/GCP port is a Terraform rewrite,
  not an application rewrite.
- **Single Go binary, internally decoupled.** One process is simpler to
  build, deploy, and log from while traffic volume is unproven. Each
  mocked service implements the same `services.Service` interface and
  knows nothing about the others, so any one of them can be lifted into
  its own container later without a rewrite.
- **GORM for queries, hand-written versioned SQL for schema.** Letting an
  ORM auto-migrate a real database can't express the least-privilege role
  grants above and risks running unreviewed schema changes against
  production. GORM stays scoped to reads/writes; migrations are
  plain, reviewable `.sql` files.
- **No real inference, ever — not configurable, not an env flag.** Every
  response is static or templated. This is a trust boundary, not a
  performance choice: it keeps the system fully deterministic/auditable,
  removes any cost-abuse or content-laundering risk, and means a
  compromised honeypot can never be used to generate real harmful output.
- **In-memory, per-process rate limiting (v1).** Good enough to stop the
  honeypot itself being used as a traffic amplifier, without adding
  Redis/shared state for a single-binary project at this scale. Revisit
  if/when services split into separate processes.

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
the persistence milestone — see the implementation plan.
