# 0005. Single timezone by design: process at UTC, product wall-clock at America/Argentina/Buenos_Aires

## Status

Accepted.

## Context

Two different things both get called "the timezone" here, and conflating them is the actual
risk:

1. **The process's own clock.** What `time.Now()` reports without an explicit location, what
   log timestamps carry, what a container's OS-level clock is set to. Left unset, this is
   whatever the host or base image defaults to — unpredictable across a laptop, a CI runner, and
   Railway.
2. **The product's wall-clock.** Every complex on the platform is in Argentina today, and
   `internal/timezone.Argentina` is a single package-level `*time.Location` (see its doc
   comment) loading `America/Argentina/Buenos_Aires`, used wherever the domain needs "today" or
   "now" in the sense a person in Argentina means it — booking dates, cancellation windows,
   payment month boundaries. The schema encodes the same zone directly: `db/migrations/001_init.sql`'s
   `booking_starts_at()` and `local_day()` functions convert with
   `AT TIME ZONE 'America/Argentina/Buenos_Aires'` written literally into the SQL, not passed as
   a parameter.

A 2026-09-11 audit flagged the hardcoded zone (in both `internal/timezone` and the SQL
functions) as something to parameterize per-complex. The owner's decision, recorded here, is the
opposite: **keep it a single, named constant — by design — because every complex actually is in
one timezone today**, and defer parameterization to whenever (if ever) the product supports
complexes outside Argentina. Threading a `zone` parameter through the domain and every SQL
function today would be speculative generality against a requirement that does not exist yet.

## Decision

- The **process** runs at `TZ=UTC`, fixed in the Docker image (`backend/Dockerfile`'s final
  stage: `ENV TZ=UTC`), not left to whatever the host or base image happens to default to. This
  keeps `time.Now()` without an explicit location, log timestamps, and anything else that is not
  routed through `internal/timezone` anchored to UTC — the one zone that has no ambiguity and no
  DST.
- The **product's** wall-clock stays a single named IANA zone,
  `America/Argentina/Buenos_Aires`, defined once in `internal/timezone` (Go) and repeated
  literally in the schema's generated columns and functions (SQL) — not derived from a
  per-complex column, not passed as a request parameter.
- Conversion between the two happens at the presentation edge: `internal/timezone.Now()` /
  `.Today()` are the only places business logic asks "what time is it", and they explicitly
  convert from the process's UTC clock into the product's Argentina wall-clock before answering.
  Nothing in the domain layer or the database is expected to reason in the server's local time.

## Consequences

- `time.LoadLocation("America/Argentina/Buenos_Aires")` needs the IANA tzdata database to
  resolve historical DST-era offsets correctly (Argentina has had none since 2009, but the
  zone's history predates that). `internal/timezone.Argentina` degrades to UTC rather than
  panicking if tzdata is missing — silently wrong by three hours, not down — which is why the
  Docker image still copies `/usr/share/zoneinfo` from the builder stage despite running at
  `TZ=UTC` itself (see the Dockerfile's comment on that `COPY`).
- Every date/time comparison in the domain and in SQL assumes a single zone. Supporting a
  complex outside Argentina is a real migration (Go and SQL both), not a config flag — this ADR
  is explicit that doing so is out of scope until the product actually needs it.
- `SENTRY_RELEASE`, request logs and anything timestamped by the process directly (not through
  `internal/timezone`) reads in UTC; anyone reading logs and comparing them against a user's
  report of "9am" needs to convert.
