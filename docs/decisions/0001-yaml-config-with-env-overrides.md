# Use a YAML config file with environment-variable overrides for secrets

## Decision

Load Nightman's runtime configuration from a YAML file
(`configs/nightman.yaml`): listener ports/services, rate limits, capture
queue size, SSE drip timing. Every value is overridable by a `NIGHTMAN_*`
environment variable, and that's the required path for secrets — the
Postgres DSN comes from `NIGHTMAN_DB_DSN`, never written into the file.

## Context

Nightman needs a handful of structured, nested settings (per-port service
lists, rate-limit numbers) plus exactly one real secret (the DB DSN). It's
deployed as a single binary/container, built and run by one person.

## Alternatives

- **Environment variables only.** No file at all. Works for a flat secret
  but awkward for nested/structured settings like "which services listen
  on which port" — you end up encoding structure into delimited strings.
- **JSON config file.** Stdlib-only (`encoding/json`), no new dependency.
  Rejected for ergonomics: no comments, trailing-comma errors, less
  pleasant to hand-edit than YAML for a file a human actually maintains.
- **File only, no env overrides.** Simpler, but then the DSN has to live
  in a file — in conflict with "never commit `.env`/secrets" and with
  keeping credentials out of version control entirely.

## Rationale

A file handles the structured settings naturally and is comfortable to
hand-edit; env overrides keep the one real secret out of the repo without
needing a second config mechanism for everything else. `github.com/goccy/go-yaml`
was added for parsing (pure Go, no transitive dependencies) — see
[docs/architecture/configuration.md](../architecture/configuration.md) for
the `Load`/`Validate` implementation.

## Consequences

- One new direct dependency (`goccy/go-yaml`), justified by "prefer stdlib
  where reasonable, ask before adding a dependency" since no stdlib YAML
  parser exists.
- Config loading has two failure surfaces to test (file parse, env
  override precedence) instead of one — both are covered by
  `internal/config`'s table-driven tests.
- `configs/nightman.yaml` must never contain a real DSN; the checked-in
  example ships with an empty `dsn` field and a comment pointing at the
  env var.
