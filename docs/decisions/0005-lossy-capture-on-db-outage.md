# Drop capture records on DB outage rather than block or buffer to disk

## Decision

If Postgres is unavailable or the async capture queue is full, the
affected record is dropped, a counter is incremented, and it's logged to
stdout as a fallback. Request handling never blocks on the database, and
there's no disk-based buffer to replay later.

## Context

The honeypot's core rule is "log everything, act on nothing" — but a
probe still has to get a response regardless of the database's health,
and a slow/unavailable DB must never become a way to stall or pile up the
honeypot's own request handling (which would itself be an amplification
risk).

## Alternatives

- **Block the request until the write succeeds.** Guarantees no data
  loss, but ties response latency to database health — exactly the
  amplification/DoS risk the brief explicitly designs against ("rate
  limit per-IP so the honeypot itself can't be abused as an amplifier").
- **Buffer to local disk and replay once the DB recovers.** No data loss
  even across outages, but adds real complexity (durable queue, replay
  logic, disk-space limits) for a resume-scale project where occasional
  gaps in a research dataset aren't a meaningful cost.

## Rationale

This is a characterization dataset about bot behavior, not a
transactional system — losing some records during a rare DB outage costs
little, while blocking the request path on DB health would compromise
the thing the whole project is supposed to demonstrate (defensive,
abuse-resistant handling of adversarial traffic).

## Consequences

- The dropped-record counter (see
  [docs/plans/hardening-and-ci.md](../plans/hardening-and-ci.md), metrics
  step) is the only visibility into how much was lost during an outage.
- A sustained outage produces a stdout-only logging window; accepted as
  the tradeoff for this decision.
