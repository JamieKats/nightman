# v1 serves a deliberately small, fixed set of ports

## Decision

v1 listens on exactly two ports: `11434` for the Ollama mock only, and
`8080` for OpenAI-compatible, vLLM, and Anthropic-style, routed by path.
The managed load balancer terminates TLS and maps public `443` → `8080`;
`11434` is forwarded straight through.

## Context

The brief lists 11434, 443, 8080, "etc." as impersonated ports, without
pinning down the exact public set. Infra/security constraints require
inbound security-group rules scoped to exactly the impersonated ports —
an open-ended list can't be expressed as a firewall rule.

## Alternatives

- **One port serving everything by path.** Simplest infra, but loses the
  realism of Ollama's distinctive port (11434) — scanners specifically
  target that port because it's Ollama's default, zero-auth exposure.
- **One port per service.** Most realistic, but multiplies the number of
  load-balancer listeners/firewall rules for services that are otherwise
  simple HTTP APIs with no real reason to be on separate ports.
- **Open-ended/configurable port list from day one.** More flexible, but
  nothing yet justifies the complexity, and it conflicts with having a
  concrete, reviewable firewall rule set.

## Rationale

11434 carries real signal (it's Ollama's well-known port, so scanners hit
it specifically) and deserves to stay isolated. The other three mocked
APIs are conventionally reverse-proxied behind a normal HTTP(S) port
anyway, so sharing 8080 and routing by path matches how they're actually
found in the wild, while keeping the firewall/LB surface to two rules.

## Consequences

- More ports/services (LM Studio's default, text-generation-webui, a bare
  `:80`/`:8000`) are an explicit post-v1 experiment — see
  [docs/plans/response-breadth.md](../plans/response-breadth.md).
- `internal/server.New` takes a plain `map[int]http.Handler`, so adding a
  port later is a config/wiring change, not a rewrite.
