# Never proxy to or call a real LLM, under any circumstance

## Decision

Every response Nightman serves is static or templated
(see [0007](0007-embedded-json-response-templates.md)). No request is
ever forwarded to, or used to generate content from, a real LLM — not in
production, not in tests, not behind a config flag, not ever.

## Context

This is the first of the brief's core design principles: "Never proxy to
a real model. All responses are static or templated." Nightman's whole
purpose is to attract and log unsolicited probing traffic without
becoming part of the infrastructure it's studying.

## Alternatives

- **Proxy to a real (possibly rate-limited/sandboxed) model for more
  convincing responses.** Would make the honeypot harder to
  fingerprint as fake. Rejected outright — not weighed as a cost/benefit
  tradeoff, because it directly creates the two risks the brief is
  explicit about avoiding: cost abuse (scanners triggering real inference
  spend) and becoming a laundering point for harmful content generation
  by whoever is probing it.
- **Real inference gated behind a config flag, off by default.** Would
  make the capability exist but unused. Rejected: an option that can be
  flipped on is a standing risk (misconfiguration, a future contributor
  enabling it without understanding why it's dangerous) that a
  non-existent capability doesn't carry.

## Rationale

This isn't a performance or cost optimization — it's the trust boundary
that makes every other claim about the system ("fully deterministic",
"auditable", "safe to expose publicly") true. Removing the capability
entirely removes the failure mode, rather than relying on configuration
or code review to keep it off.

## Consequences

- Response "realism" is capped at whatever a fixed template pool can
  achieve (see [0007](0007-embedded-json-response-templates.md)) —
  accepted as the cost of this boundary, with the brief's own framing:
  "just convincing enough... not to fool a human."
- There is no code path anywhere in the project that constructs a
  request to a real model provider, so this constraint needs no runtime
  enforcement (rate limit, feature flag, egress rule) to hold — it's
  true by the absence of the capability.
