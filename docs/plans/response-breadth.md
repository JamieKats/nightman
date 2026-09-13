# Response breadth: streaming, more services, bigger template pools

## Goal

Cover the rest of the brief's target endpoints (OpenAI-compatible, vLLM,
Anthropic-style) and add streaming so a probe that expects SSE/NDJSON
gets something plausible — this is milestone M3.

## Current architecture

See [docs/architecture/service-mocks.md](../architecture/service-mocks.md).
Only Ollama exists, non-streaming only. `internal/services/openai`,
`vllm`, and `anthropic` are empty packages. `internal/response`'s
template pools hold 1–2 templates per Ollama endpoint; no streaming
helper exists anywhere in the codebase yet.

## Proposed design

**Streaming helper** (new, shared code — likely `internal/response` or a
new small package): drip a templated completion token-by-token over SSE
(OpenAI/Anthropic style `data: {...}\n\n` frames) or Ollama's own NDJSON,
with a configurable inter-token delay
(`config.Streaming.TokenInterval()`, already defined) and a final
done/usage frame. `capture.Middleware`'s `ConnectionDurationMS` needs to
start actually differing from `LatencyMS` here — the middleware's
`statusWriter` will need to track when the *connection* closes, not just
when the handler function returns, and handle a client disconnecting
mid-stream without leaking goroutines.

**Ollama streaming**: `stream: true` on `/api/generate` and `/api/chat`
uses the new helper instead of the existing one-shot response.

**OpenAI-compatible** (`internal/services/openai`): `/v1/models`,
`/v1/completions`, `/v1/chat/completions`, streaming and non-streaming,
following the same `Service` + template-pool pattern as Ollama. Capture
emphasizes `Authorization: Bearer sk-…` (already captured generically by
`capture.Record.Headers`; nothing new needed there beyond making sure the
templates' `usage`/`finish_reason`/`id`/`created` fields look plausible).

**vLLM / text-generation-webui / LM Studio** (`internal/services/vllm`):
same OpenAI-compatible shape with the small real-world differences
(extra fields, model-name conventions) layered on top.

**Anthropic-style** (`internal/services/anthropic`): `/v1/messages`,
`x-api-key` capture (same as above — generic header capture already
covers it), Messages response shape, `input_tokens`/`output_tokens`, SSE
event sequence for streaming.

**Wiring**: all three new services mount on the `8080` listener
alongside each other, routed by path — see
[0004](../decisions/0004-v1-port-map.md).

**Template pools**: expand every endpoint (Ollama included) to ~10
templates, once real traffic data suggests what variety is worth having
— see [0007](../decisions/0007-embedded-json-response-templates.md).

## Constraints

- No per-request customization based on probe content, still — see
  [0012](../decisions/0012-no-real-inference-ever.md) and
  [0007](../decisions/0007-embedded-json-response-templates.md). Streaming
  changes *delivery*, not content generation.
- A disconnecting client mid-stream must not leak a goroutine or block
  the listener.
- Golden-file testing pattern (read the real template files off disk)
  continues for every new service — see
  [docs/architecture/service-mocks.md](../architecture/service-mocks.md).

## Implementation Phases

1. Build the streaming helper + its own tests (byte-plausible output,
   disconnect handling, no goroutine leaks).
2. Ollama streaming variants on `/api/generate`, `/api/chat`.
3. OpenAI-compatible service + templates + tests.
4. vLLM service + templates + tests.
5. Anthropic-style service + templates + tests.
6. Register all three in `cmd/nightman`.
7. Expand every service's template pools toward ~10/endpoint.

## Decisions

[0004](../decisions/0004-v1-port-map.md) (shared `8080` port),
[0007](../decisions/0007-embedded-json-response-templates.md) (static
templates, applies to streamed content too),
[0003](../decisions/0003-stdlib-http-router.md) (router — revisit only if
composing three services on one port gets unwieldy).

## Open questions

- Whether streaming needs its own package (`internal/streaming`?) or
  stays inside `internal/response` — not yet decided, will likely become
  clear once the first implementation (Ollama) is attempted.
- Exact vLLM/LM Studio shape differences worth mocking — the brief notes
  these are "similar shape to OpenAI-compatible," but the specific
  quirks worth reproducing aren't pinned down yet; likely to be informed
  by what real scanners actually probe for.
