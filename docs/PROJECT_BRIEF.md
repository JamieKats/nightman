# Project: Nightman — an AI honeypot for LLM-scanning bots

**Context:** this is a personal portfolio/resume project — a way to build
and demonstrate something genuinely interesting (not a toy CRUD app) for
job hunting. Scope should stay realistic for a project built and maintained
by one person in spare time. Raw collected data (full request bodies,
headers, source IPs at the individual-record level) is not intended for
publication or public release. However, an **aggregate statistics
dashboard** (see Dashboard section) is intended to be public and linked
from a resume — so the distinction is: raw logs stay private, aggregate/
summary stats are intentionally public-facing.

## Concept

An AI honeypot that exposes fake, internet-facing endpoints mimicking common
self-hosted and API-based LLM services. The goal is to attract and log
unsolicited traffic from bots, scanners, and researchers probing the internet
for exposed LLM infrastructure (open Ollama instances, leaked API keys,
prompt-injection probes, model-scraping attempts, etc.), without ever running
a real model or doing real inference.

This project actively responds to probes using static or templated
responses — enough to sustain a basic back-and-forth with an automated
client — but never sends a request to, or generates content from, a real
LLM. No real inference happens anywhere in the system.

## Why this is interesting

Classic honeypots (SSH, HTTP, SMB) are well-trodden. LLM-service honeypots are
newer territory — there's active internet-wide scanning for:
- Open Ollama instances (port 11434) — people hunting free GPU compute
- OpenAI-compatible endpoints (`/v1/chat/completions`, `/v1/models`) — key
  stuffing, leaked-key validation, scraping
- vLLM / text-generation-webui / LM Studio style endpoints
- Generic `/v1/messages`-style Anthropic-format endpoints

## Tech stack

- **Language: Go.** Good fit for this — strong concurrency primitives for
  handling many simultaneous scanner connections, easy static binary
  deployment into a locked-down VPC instance/container, and solid standard
  library support for HTTP servers, TLS, and streaming responses (SSE for
  mimicking token-by-token generation).

## Core design principles

1. **Never proxy to a real model.** All responses are static or templated.
   No real inference happens, ever. This avoids cost abuse, avoids becoming
   a laundering point for harmful content generation, and keeps behavior
   fully deterministic and auditable.
2. **Just convincing enough.** Responses need valid shape (correct JSON
   schema, plausible token usage fields, streaming support) to keep an
   automated client engaged long enough to be interesting — not to fool a
   human. See Response strategy for the concrete v1 approach.
3. **Isolate hard.** Runs in its own VPC/subnet with no route to real infra,
   no real credentials anywhere in the environment, and locked-down egress
   (deny-by-default) so a compromised container can't pivot or be used to
   attack others.
4. **Log everything, act on nothing.** Full request capture (IP, headers,
   body, timing) shipped to a queryable log store. Rate limit per-IP so the
   honeypot itself can't be abused as an amplifier.

## Architecture

**Starting point:** single reverse proxy that routes by port/path to
internal handlers for each mocked service (Ollama, OpenAI-compatible, etc.),
all within one Go binary/process. Simpler to build, deploy, and log from
initially.

**Future direction:** split each mocked service out into its own container
once the single-binary approach outgrows itself (e.g. if isolating a
compromised or heavily-hammered fake service becomes important, or if
different services need different scaling/resource profiles). The routing
layer should be built with this split in mind — keep per-service logic
decoupled behind clear interfaces so a handler can be lifted out into its
own container/process later without a rewrite.

## Cloud infrastructure

**Provider: DigitalOcean.**

- **Droplets** — compute for the Go application (reverse proxy + mocked
  service handlers).
- **VPC** — isolated private network per the infra/security constraints
  above; Droplets and the managed database communicate over the private
  network, not the public internet.
- **Managed Load Balancer** — public entry point; terminates TLS (SSL
  termination, decrypting at the load balancer and forwarding unencrypted
  traffic to the Droplet over the private network), routes by port/path to
  the reverse proxy. Certificate management via DigitalOcean's Let's
  Encrypt integration.
- **Managed Database (PostgreSQL)** — the Postgres instance described in
  Logging requirements, run as a managed cluster instead of self-hosted in
  a container; supports restricting inbound connections to trusted
  sources, which is how the ingestion service (write role) and Grafana
  (read-only role) will be scoped separately.
- **doctl / Terraform** — worth using for infrastructure-as-code rather
  than clicking through the control panel, both for repeatability and
  because it's a nice thing to point to on a resume alongside the app
  itself.

## Target endpoints (priority order)

1. **Ollama-compatible** — `/api/generate`, `/api/chat`, `/api/tags`,
   `/api/show`. Highest scan volume; often exposed with zero auth.
2. **OpenAI-compatible** — `/v1/chat/completions`, `/v1/completions`,
   `/v1/models`. High volume, frequently targeted for key validation
   (`Authorization: Bearer sk-...` header capture is a key log field here).
3. **vLLM / text-generation-webui / LM Studio** — similar shape to
   OpenAI-compatible, worth covering for breadth.
4. **Anthropic-style `/v1/messages`** — lower scan volume, nice to have for
   completeness.

## Logging requirements

Capture per-request:
- Source IP + port
- Full headers (esp. `Authorization`, `User-Agent`, `X-Api-Key`)
- Full request body (raw + parsed)
- Timestamp, response latency
- TLS fingerprint (JA3/JA4) — **deferred, see Future improvements.** Not
  captured in v1; TLS is terminated at a managed load balancer instead.
- Which fake endpoint/model was hit
- Connection duration (useful for streaming endpoints — how long does a bot
  stay attached to an SSE/token stream before giving up?)

Storage backend: **PostgreSQL**, run as a DigitalOcean Managed Database
(see Cloud infrastructure) rather than self-hosted in a container.
Structured columns for fields we query/filter/aggregate on constantly
(timestamp, source IP, service, path, response status, latency), with a
**JSONB** column for the variable-shaped parts (headers, request body) so we
get NoSQL-like flexibility without giving up relational query power or
needing a separate datastore. Revisit only if write volume grows into
genuine time-series-analytics territory (e.g. ClickHouse/Loki), which is not
expected at honeypot scale.

Rough shape:
```
requests (
  id, ts, source_ip, source_port,
  service,            -- 'ollama' | 'openai' | 'vllm' | ...
  path, method,
  headers   JSONB,
  body      JSONB,
  ja3_fingerprint,    -- nullable; not populated until TLS fingerprinting
                       -- is implemented, see Future improvements
  response_status,
  latency_ms,
  connection_duration_ms
)
```
Index on `ts`, `source_ip`, `service`; add index on `ja3_fingerprint` once
that column is actually populated. GIN index on `headers`/`body` if/when we
need to search inside them.

Pipeline direction beyond the DB itself (open to revisiting): whether to
also ship structured JSON logs to S3/CloudWatch or a self-hosted ELK/Loki
stack for dashboards, in addition to Postgres as the primary store.

## Infra/security constraints

- Isolated VPC subnet, no route to real infra or credentials
- Security groups: inbound only on impersonated ports (11434, 443, 8080,
  etc.), nothing else
- Egress: deny-by-default
- Per-IP rate limiting on all endpoints
- No real secrets or functioning API keys anywhere in the honeypot — nothing
  that could be mistaken for a real service with real consequences for a
  caller who "succeeds"
- Grafana (public-facing) connects to Postgres via a dedicated read-only
  role — never the ingestion service's write credentials

## Open questions / decisions to make with Claude Code

None currently open — revisit as the build progresses.

## Future improvements

- **TLS fingerprinting (JA3/JA4).** Not part of the initial build. v1
  accepts TLS termination at a managed load balancer, which does not expose
  the raw ClientHello needed to compute JA3/JA4 hashes. Revisit later if
  bot-family clustering becomes a priority — would require terminating TLS
  at the app layer ourselves (custom `net.Listener`/library, since Go's
  standard `net/http`/`tls` packages don't expose this by default) and
  trades away the simplicity of a managed LB.
- **Smarter/expanded response templates.** See Response strategy — v1 uses
  a small fixed pool of random canned responses; revisit variety and
  selection logic once real traffic data has been collected and analyzed.
- **Country-level IP geolocation map on the dashboard.** See Dashboard —
  requires adding a geolocation lookup step (e.g. MaxMind GeoLite2) to the
  ingestion pipeline to resolve source IP → country before it can be shown
  as a map panel. Not part of the initial panel set.

## Response strategy

**v1:** a small fixed pool (~10) of canned template responses per endpoint,
selected at random per incoming request. No dynamic generation, no
per-request customization based on the probe's content — just enough
variety to avoid being trivially fingerprinted as "always returns the exact
same byte-for-byte response."

**Future improvement:** once enough real traffic has been collected and
analyzed, revisit response design based on what was actually observed —
e.g. more templates, smarter selection (matching response shape to what a
given probe seems to expect), or other refinements. Not a v1 concern; v1's
job is to collect the data that will inform this.

## Dashboard

**Tool: Grafana**, connected to the Postgres instance as a data source.
Goal is a **public, live dashboard** linkable from a resume — showing
aggregate stats about the dataset, not raw per-record data.

**Public-exposure design constraints:**
- Grafana's data source connection should use a **dedicated read-only
  Postgres role**, separate from whatever credentials the ingestion
  service uses to write — a public dashboard is a larger attack surface
  than an internal one, and it should be structurally impossible for it to
  modify data or reach anything beyond what the dashboard needs.
- Panels should show **aggregates and counts only** — request volume over
  time, breakdowns by endpoint/service, top fake models probed, etc. — not
  raw request bodies, raw headers, or individual full IP addresses in a
  browsable/searchable form. Aggregating or bucketing source IPs (e.g. by
  ASN/country, or "unique IPs per day" counts) is preferable to listing
  them individually on a public panel.
- Likely implementation: Grafana's built-in public dashboard sharing
  feature (share specific panels/dashboards without requiring viewer
  login), rather than exposing the full Grafana instance/login publicly.

**Example panel ideas (starting point, not final):**
- Requests over time (overall, and per mocked service)
- Breakdown of hits by endpoint (Ollama vs OpenAI-compatible vs vLLM, etc.)
- Top "fake models" requested
- Unique source IPs per day/week (as a count, not a list)
- Top User-Agent strings seen (aggregated/grouped)
- Response latency / connection duration stats
- Running total of requests collected since launch
- **(Future improvement)** Country-level map of request origins, based on
  IP geolocation — requires adding a geolocation lookup step to the
  ingestion pipeline (e.g. MaxMind GeoLite2 or similar) to resolve source
  IP → country before it can be visualized. Not part of the initial panel
  set.

## Non-goals

- Not trying to fool a human operator — only automated scanners/bots
- Not proxying or wrapping any real LLM provider
- Not collecting more than needed to characterize bot behavior (no scope
  creep into deanonymizing operators, etc.)

## Naming

Project name: **Nightman** (Always Sunny in Philadelphia themed — "The
Nightman Cometh" for whoever's out there scanning at 3am). Sub-component
names TBD — could extend the theme (e.g. fake Ollama service, logging
pipeline, dashboard could riff on "Dayman", "Cometh", etc.).