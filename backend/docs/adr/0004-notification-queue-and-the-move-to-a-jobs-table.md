# 0004. Redis-backed notification queue today, unify on a Postgres `jobs` table later

## Status

Accepted (current state) with a planned direction. The unification described below is a stated
owner decision, not started — nothing in this repository depends on it yet.

## Context

Emails and WhatsApp messages are sent asynchronously through `internal/notifier`, a durable,
at-least-once task queue built directly on Redis primitives (see the package doc comment in
`internal/notifier/notifier.go`): a pending list, a processing list for claimed tasks, a hash of
claim timestamps, a delayed-retry sorted set, and a capped dead-letter list. It is deliberately
built with the same vocabulary — claim, reclaim, stale, exhausted, dead — as this codebase's
other durability-critical queues (webhook processing, refund retry) use, some of which are
Postgres-based rather than Redis-based. This is why Redis has no fallback for notifications even
though rate limiting, the token blacklist, and the SSE hub all degrade gracefully without it
(see `README.md`'s Architecture notes): the notification queue is the one Redis-backed
subsystem with no substitute path.

Having durable job semantics implemented twice — once against Redis primitives (LPUSH/BRPOPLPUSH
equivalents, a ZSET for delayed retries, a hash for claims), once against Postgres (row locks,
`SELECT ... FOR UPDATE SKIP LOCKED`-style claiming) — is two implementations of the same
guarantee to reason about, test, and operate.

## Decision

Keep the Redis-backed queue as-is for now; nothing about the durability guarantee it provides is
wrong. The intended direction is to unify every durable queue in this service — notifications
included — onto a single Postgres `jobs` table, using the same claim/reclaim/dead-letter
vocabulary `internal/notifier` already uses, backed by row locking instead of Redis data
structures. This removes a second durability implementation to maintain and removes
notifications' hard dependency on Redis being reachable, at the cost of moving that queue's
read/write load onto Postgres instead.

This has not started. No code in this PR or any open branch implements a `jobs` table or begins
migrating `internal/notifier`'s callers off it.

## Consequences

- No change today: this ADR documents intent, not a migration in progress.
- When undertaken, the migration needs to preserve `internal/notifier`'s existing contract
  (`Enqueue`, at-least-once delivery, backoff, dead-letter) so callers throughout `cmd/api` do
  not need to change, only the queue's storage underneath them.
- Once done, Redis becomes purely an optional performance/availability layer (rate limiting,
  token blacklist, SSE hub, slot locking) with every subsystem degrading gracefully without it —
  removing the one asymmetric hard dependency called out in `README.md` today.
