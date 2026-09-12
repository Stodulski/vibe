# 0004. Unify the notification queue on a Postgres `jobs` table

## Status

Accepted, and implemented for the notification queue: `db/migrations/004_jobs.sql` creates the
table and `internal/jobs` is the queue. `internal/notifier` is gone. The two payment queues have
NOT moved; see "What did not move" below for why, which is the part of this decision that
changed on contact with the code.

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

## What happened

`internal/notifier` is deleted. `internal/jobs` is the queue: `jobs.Store` owns the table,
`jobs.Pool` is the worker pool, `jobs.Enqueuer` is the publishing side, and
`notifications.Queue` is unchanged apart from one added argument — the deduplication key.

Two things were added that the Redis queue did not have and could not easily be given:

- **A deduplication key** (JOB-04). The queue is at-least-once, and the Redis one had no key at
  all: a redelivered MercadoPago webhook, or a claim whose acknowledgement was lost, sent the
  client a second confirmation email and a second WhatsApp message. `jobs.DedupKey` hashes the
  task type, the address it goes to and the booking it is about; a second enqueue under an
  existing key is a no-op, enforced by a partial unique index rather than by a check-then-write.
- **Jitter on the backoff** (OUT-02). Every row that failed against one provider outage used to
  carry the same next attempt to the second.

The claim is `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING`, which is
what lets every instance run the same statement on the same tick and take disjoint rows without
waiting on each other.

## What did not move, and why

`webhook_events` and `failed_refunds` stay where they are. The plan above treated them as two
instances of the same queue; they are not.

- `failed_refunds` is a ledger before it is a queue. Since refunds became claim-first it holds
  one row per refund attempted, written inside the same transaction as the payment status flip,
  with foreign keys to `payments`, `bookings` and `complexes`, `ON DELETE CASCADE` from the
  tenant, an `amount > 0` check, a `tenant_isolation` row-level-security policy and three
  indexes on columns the tests query it by. A generic `jsonb` payload keeps none of that, and
  five integration tests read the table by `payment_id`, `booking_id` and `complex_id`.
- `webhook_events` carries a status vocabulary that contradicts this table's. `exhausted` there
  is a *pause* — the sweeper picks the row up again once its backoff elapses, which is what
  stopped a provider outage permanently abandoning captured payments, and
  `TestAnExhaustedEventComesBackWhenItsBackoffElapses` pins it. `failed` here is terminal. One
  column cannot mean both without the unification being a rename.

Moving either would have weakened a guarantee to satisfy a shape. The duplication that remains
is two sweepers in the payments domain, which is a smaller cost than the one the move would
have paid.

## Consequences

- Notifications no longer depend on Redis. A Redis that loses its data no longer loses queued
  email; the queue's load is on Postgres instead, which is bounded by the worker count and the
  poll interval rather than by traffic.
- Redis is still required at boot, and the reason in `cmd/api/main.go` is now the honest one:
  the token blacklist, the user cache, distributed rate limiting, slot locking and the SSE relay
  all fall back to per-instance state, which is silently wrong the moment a second instance is
  running. That is a different argument from the one this ADR used to carry, and it is written
  down at the guard.
- The queue's `expvar` map is injected rather than registered globally (CON-07). The name
  `/debug/vars` publishes it under is unchanged — an operator's dashboard reads it.
