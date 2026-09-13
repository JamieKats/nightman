# Serve canned response templates byte-for-byte via embed.FS

## Decision

Canned responses are plain JSON files embedded into the binary with Go's
`embed.FS` (`internal/response/templates/<service>/<endpoint>/*.json`),
grouped into pools keyed `"<service>/<endpoint>"` and served byte-for-byte.
There is no templating engine and no per-request customization based on
probe content.

## Context

The brief's v1 response strategy is explicit: "a small fixed pool (~10)
of canned template responses per endpoint, selected at random per
incoming request. No dynamic generation, no per-request customization
based on the probe's content." The word "template" in that strategy
refers to a fixed example response, not a `text/template`-style
substitution template.

## Alternatives

- **`text/template` (or similar) with placeholder substitution.** Would
  allow injecting a timestamp, a fake request-specific ID, or echoing
  part of the probe back. Rejected: the brief explicitly rules out
  per-request customization for v1 — it's flagged as a **future**
  improvement once real traffic data suggests it's worth the
  fingerprinting-risk tradeoff (an always-varying fake timestamp is its
  own kind of signal).
- **Templates as Go literals (structs or string constants) instead of
  files.** Avoids `embed.FS` and file I/O entirely. Rejected for
  ergonomics: JSON-shaped example responses are far easier to author,
  diff, and expand (step 20: ~10 per endpoint) as files than as inline Go
  source.
- **External files loaded at runtime (not embedded).** Lets templates
  change without a rebuild. Rejected: Nightman ships as a single static
  binary by design (easy deployment into a locked-down instance); runtime
  file dependencies would work against that.

## Rationale

Embedding keeps the "single static binary" deployment property intact
while keeping template authoring in plain, diffable JSON files rather
than Go source. Validating every template as JSON at `Load()` time (not
per-request) makes a malformed template a build-time failure, matching
the project's general fail-fast posture.

## Consequences

- Adding/editing a template requires a rebuild — acceptable since
  Nightman already rebuilds/redeploys as a unit.
- `response.Load`/`loadFrom` validate and group templates eagerly; a
  missing or malformed pool is a startup error, not a runtime surprise
  against a live probe.
