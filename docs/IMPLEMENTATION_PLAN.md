# Nightman — Implementation Plan

A step-by-step build order for the honeypot described in
[PROJECT_BRIEF.md](PROJECT_BRIEF.md). Each numbered step is intended to be
roughly one PR / commit. Phases are ordered by dependency: earlier phases
produce something runnable and testable before later phases build on them.

All build decisions are settled — see [Decisions](#decisions) at the end.

---

## Guiding constraints (recap)

- **No real inference, ever.** Every response is static or templated.
- **Just convincing enough** for an automated client — correct JSON shape,
  plausible usage fields, working streaming. Not trying to fool a human.
- **Log everything, act on nothing.** Full request capture → Postgres.
- **Isolate hard.** Deny-by-default egress, no real secrets, dedicated
  read-only DB role for the public dashboard.
- Single Go binary for v1; per-service logic stays behind the
  `services.Service` interface so a handler can be split into its own
  container later without a rewrite.

---

## Milestones

| # | Milestone | Phases |
|---|-----------|--------|
| M1 | Honeypot runs locally, logs probes to stdout (Ollama only) | 1–2 |
| M2 | Probes persisted to Postgres via async capture pipeline | 3 |
| M3 | All four service mocks + streaming, ~10 templates each | 4 |
| M4 | Hardened: rate limiting, size caps, config, metrics, CI | 5 |
| M5 | Deployed to DigitalOcean via Terraform | 6 |
| M6 | Public Grafana dashboard linked from resume | 7 |

---

## Phase 1 — Foundations

**1. Config loading** (`internal/config`)
Define the config struct: enabled services, listener ports, per-IP rate
limits, Postgres DSN, capture-queue size, SSE drip timing. Load from a YAML
file (`configs/nightman.yaml`) with environment-variable overrides for
secrets — the DSN comes from `NIGHTMAN_DB_DSN`, and every override follows
the `NIGHTMAN_*` prefix pattern. Validate on startup; fail fast with a
clear error.
*Done when:* `config.Load()` returns a validated struct and has table tests
for the valid/invalid cases, including env override precedence.

**2. Logging setup**
Standardise on `log/slog` (already used in `cmd/nightman`). JSON handler,
level from config, a helper to attach a per-request ID. This is the
operational log — distinct from request *capture*.
*Done when:* a `logging.New(cfg)` helper exists and `main` uses it.

**3. Capture data model** (`internal/capture`)
Define `Record` — the per-probe struct matching the `requests` table:
source IP/port, method, path, service name, headers map, raw + parsed
body, timestamps, response status, latency, connection duration. Add
`RecordFromRequest(*http.Request)` that extracts everything cheaply and
size-caps the body read.
*Done when:* `Record` and the extractor exist with unit tests, including
oversized-body and malformed-body cases.

---

## Phase 2 — Routing + first service

**4. Server / listener manager** (`internal/server`)
Multi-port listener supervisor: bind each configured port, mount the
`http.Handler` for the service(s) on that port, coordinate graceful
shutdown. Router = stdlib `net/http` mux (method + path patterns).

v1 port map (a deliberately small set — more come later, see
[Future improvements](#future-improvements)):

| Port | Serves |
|------|--------|
| `11434` | Ollama mock only |
| `8080` | OpenAI-compatible + vLLM + Anthropic, routed by path |

The managed load balancer terminates TLS and maps public `443` → `8080`;
`11434` is forwarded straight through.

*Done when:* the server starts N listeners, serves a 404 on unknown
paths, and shuts down cleanly on SIGINT/SIGTERM.

**5. Capture middleware**
`http.Handler` wrapper: start timer → serve → build `capture.Record` →
hand to a `capture.Sink`. v1 sink is a stdout JSON logger (real store
lands in Phase 3). Records status and latency via a response-writer
wrapper.
*Done when:* every request through the middleware emits one capture line;
tested with `httptest`.

**6. Response templates** (`internal/response`)
Embed canned responses with `embed.FS` from
`internal/response/templates/<service>/*.json`. `Pool(service).Random()`
returns one, seeded per-process. Start with 1–2 templates per endpoint;
expand in step 20.
*Done when:* pools load at init, `Random()` is uniform, missing-pool is a
startup error.

**7. Ollama service mock** (`internal/services/ollama`)
Implement `services.Service` for `/api/tags`, `/api/show`, `/api/generate`,
`/api/chat` — **non-streaming responses only** at this step. Valid Ollama
JSON shapes, plausible `eval_count` / `eval_duration` fields, a small fake
model list.
*Done when:* each route returns schema-valid JSON; golden-file tests.

**8. Wire it together** (`cmd/nightman`)
Replace the stub handler: load config, build logging, construct the
router with Ollama registered, wrap in capture middleware, start
listeners.
*Done when:* `go run ./cmd/nightman` serves the Ollama mock and logs
probes. **← M1**

---

## Phase 3 — Persistence

**9. Schema migrations** (`migrations`)
Hand-written, versioned SQL — no ORM auto-migration.
- Replace the `0001_init.sql` placeholder with real DDL for `requests`:
  structured columns + `headers`/`body` as JSONB, plus the `ts` /
  `source_ip` / `service` indexes.
- Add `0002_roles.sql`: `nightman_ingest` (INSERT/SELECT on `requests`)
  and `nightman_ro` (SELECT only — the public Grafana datasource role).
Applied by a `make migrate` target that runs the ordered `.sql` files
through `psql`. Document the command in the README.

**10. Store implementation** (`internal/store`)
GORM (`gorm.io/gorm` + `gorm.io/driver/postgres`) for reads/writes only —
it never manages schema. Define `store.Request`, a GORM model mapped to
the `requests` table, with `gorm.io/datatypes`.`JSON` for the `headers`
and `body` columns. Expose `Insert(ctx, capture.Record)`,
`InsertBatch(ctx, []capture.Record)` (via `CreateInBatches`), and `Ping`
(through the underlying `*sql.DB`). Opened with the `nightman_ingest` DSN.
*Done when:* the store has table-driven tests against a mocked DB layer
(e.g. `go-sqlmock` behind GORM). Real-Postgres integration via
testcontainers is deferred — see [Future improvements](#future-improvements).

**11. Async capture sink**
Bounded channel + background writer that batches `InsertBatch` calls
(size/time flush). On queue overflow or DB outage: drop, increment a
counter, and fall back to logging the record to stdout. Lossy capture is
acceptable here (see [Decisions](#decisions) E) — request handling must
never block on the DB.
*Done when:* a load test shows request latency is unaffected by DB
latency; the overflow path is tested.

**12. Local dev environment**
`docker-compose.yml` with Postgres (and Grafana, for Phase 7). Makefile
targets: `make run`, `make migrate`, `make test`, `make lint`. README
"Running locally" section.
*Done when:* a fresh clone can `make up && make migrate && make run`.

**13. Swap in the store**
Make the async sink write to `internal/store`. Config flag to select
stdout vs Postgres sink.
*Done when:* probes land in the `requests` table end to end. **← M2**

---

## Phase 4 — Breadth + realism

**14. Streaming / SSE helper**
Shared utility to drip a templated completion token-by-token over SSE (or
Ollama's NDJSON), with configurable inter-token delay and a final
done/usage frame. Capture middleware records `connection_duration_ms` and
handles client disconnect mid-stream.
*Done when:* a streamed response is byte-plausible to a real client lib;
disconnect is handled without leaking goroutines.

**15. Ollama streaming variants** — `stream: true` on `/api/generate` and
`/api/chat`.

**16. OpenAI-compatible service** (`internal/services/openai`)
`/v1/models`, `/v1/completions`, `/v1/chat/completions` (streaming and
non-streaming). Emphasise `Authorization: Bearer sk-…` capture. Plausible
`usage` object, `finish_reason`, `id`/`created` fields.

**17. vLLM / TGW / LM Studio** (`internal/services/vllm`)
Layer the small shape differences (extra fields, model-name conventions,
`/v1` quirks) on top of the OpenAI shape.

**18. Anthropic `/v1/messages`** (`internal/services/anthropic`)
`x-api-key` capture, Messages response shape, `input_tokens` /
`output_tokens`, SSE event sequence for streaming.

**19. Register the new services** in `cmd/nightman` — OpenAI, vLLM and
Anthropic all mount on the `8080` listener, routed by path (see the
Phase 2 port map).

**20. Expand template pools** to ~10 per endpoint across all services.

---

## Phase 5 — Hardening

**21. Per-IP rate limiting** (`internal/ratelimit`)
In-memory token bucket keyed by source IP (`golang.org/x/time/rate` +
expiring map). Over-limit → `429` with a plausible body. Emit a
rate-limited metric.

**22. Request abuse caps** — max body size, max header count/size, read
and write timeouts, max concurrent conns per listener.

**23. Config surface** — enable/disable each service, bind ports, per-route
template pool overrides, rate-limit numbers. Document every key.

**24. Metrics** — Prometheus endpoint on a **private** port: request
counts by service/status, capture queue depth, dropped-record count,
rate-limit hits.

**25. Test suite** — unit (template selection, capture extraction, rate
limiter), handler tests via `httptest` per service, store tests against a
mocked DB layer. Every path that handles an incoming request must have
test cases. Target a meaningful coverage floor in CI.

**26. CI** — GitHub Actions: `golangci-lint`, `go test ./...`, `go build`,
and (optional) build + push a container image. Scaffold a minimal
lint+test workflow as soon as Phase 2 lands; expand it here. **Future
improvement:** add testcontainers-backed integration tests (real Postgres
in CI) once the mocked-DB suite is stable.
**← M4**

---

## Phase 6 — Deployment (DigitalOcean, Terraform)

Infra-as-code lives in `deployments/terraform`. All of Phase 6 assumes
M4 is done.

**27. Dockerfile** — multi-stage, static binary on `scratch`/distroless.
Put build assets under `build/package`.

**28. Terraform: network** — VPC + private subnet.

**29. Terraform: Managed Postgres** — cluster, `nightman_ingest` (write)
and `nightman_ro` (read-only, for Grafana) roles, trusted-sources
firewall limiting inbound to the droplet and Grafana only.

**30. Terraform: compute** — Droplet(s) with cloud-init to pull and run
the container; attached to the VPC, no public egress route beyond what's
needed to pull the image.

**31. Terraform: Managed Load Balancer** — public entry point, TLS via
DO's Let's Encrypt, forward the impersonated ports/paths to the droplet
over the private network. Inbound firewall: only the impersonated ports.

**32. Egress lockdown** — deny-by-default outbound firewall on the droplet.

**33. Secrets + runbook** — DB credentials via DO project env / Terraform
variables (never in the repo). Deploy + rollback runbook in `docs/`.
**← M5**

---

## Phase 7 — Dashboard (Grafana)

**34. Grafana provisioning as code** — datasource pinned to the
`nightman_ro` Postgres role; dashboards defined as JSON in the repo.

**35. Panels** — requests over time (overall + per service), hits by
endpoint, top fake models probed, unique source IPs per day (count, not
list), top User-Agents (grouped), latency / connection-duration stats,
running total since launch.

**36. Public sharing** — use Grafana's public-dashboard feature for the
specific dashboard only; do not expose the Grafana login publicly.

**37. Link from resume / README.** **← M6**

---

## Future improvements

Not scheduled — revisit once there's a running v1 collecting real traffic.

From the brief:

- TLS fingerprinting (JA3/JA4) — needs app-layer TLS termination, drops
  the managed-LB simplicity. Adds the `ja3_fingerprint` column + index.
- Country-level IP geolocation map — MaxMind GeoLite2 lookup step in the
  capture pipeline.
- Smarter/expanded response templates once real traffic is analysed.

Deferred build choices (to experiment with once something is running):

- Router: swap the stdlib `net/http` mux for `chi` and compare — routing
  ergonomics, middleware composition, per-service sub-routers.
- More impersonated ports / services — e.g. LM Studio's default port,
  text-generation-webui, a bare `:80` / `:8000`. v1 stays with `11434`
  and `8080` only.
- testcontainers integration tests — spin up a real Postgres in CI to
  exercise the store against actual SQL, once the mocked-DB suite is
  stable. Added when the CI/CD pipeline is built (Step 26).
- Cloud-provider agnosticism, enforced rather than just architectural —
  v0.1 targets DigitalOcean only (see Decisions, below). The app layer is
  already provider-neutral (standard Postgres, stdlib HTTP, no DO SDK
  calls); what's deferred is formalizing that as a checked rule — e.g. a
  CLAUDE.md constraint against provider-specific code outside
  `deployments/terraform` — and actually proving portability with a
  second Terraform target (AWS being the obvious one, see the cost
  comparison this decision was weighed against). Revisit once v0.1 is
  running on DO.

---

## Decisions

Settled. Recorded here so the reasoning isn't re-litigated mid-build.

| ID | Area | Decision |
|----|------|----------|
| A | Config | YAML file `configs/nightman.yaml`, with `NIGHTMAN_*` env-var overrides for secrets (`NIGHTMAN_DB_DSN`). |
| B | DB layer & migrations | **GORM** (`gorm.io/gorm`, `gorm.io/driver/postgres`, `gorm.io/datatypes`) for reads/writes only. Schema is hand-written versioned SQL under `migrations/`, applied via `make migrate` (`psql`). No auto-migration — it can't express the `nightman_ro` role and shouldn't run against the managed DB. |
| C | HTTP router | stdlib `net/http` mux. `chi` is a post-v1 experiment. |
| D | v1 port map | `11434` → Ollama; `8080` → OpenAI + vLLM + Anthropic by path; LB maps `443` → `8080`. Deliberately minimal; more ports post-v1. |
| E | Capture durability | Drop + counter + stdout fallback if Postgres is unavailable. Acceptable for a resume project; no disk buffer. |
| F | Rate-limit state | In-memory per process (resets on restart, not shared once services split — fine for v1). |
| G | CI | GitHub Actions from Phase 2, expanded at Step 26. |
| H | License | None — not expecting external contributors. |
| I | Template storage | `embed.FS` JSON under `internal/response/templates/<service>/`. |
