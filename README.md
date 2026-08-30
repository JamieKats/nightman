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
  honeypot/          Service interface + per-service mocks
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

## Building

```sh
go build ./...
```

Local run instructions (Docker Compose, migrations, config) will land with
the persistence milestone — see the implementation plan.
