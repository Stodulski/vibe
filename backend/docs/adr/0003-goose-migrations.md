# 0003. goose migrations, forward-only, direct edits while there are no production users

## Status

Accepted. The "no production users" condition below is what makes this safe, and it will stop
being true the day the product has a real deployment with real data — see Consequences.

## Context

Schema changes need a tool that applies them in order, records what has run, and works both
from a CLI (local development) and embedded in the binary (the deploy path). See
`backend/CLAUDE.md`'s Database section for the full mechanism: migrations live in
`db/migrations/`, are embedded via `db/migrations.go`, and `internal/migrate` applies them with
[goose](https://github.com/pressly/goose) as a library — the same version, against the same
`goose_db_version` table, whether run by the CLI or by the binary's `-migrate-only` mode.

Migrations are ordinarily immutable once applied: the rule exists so that two databases that
both ran migration N are guaranteed to agree on what N did. `db/migrations/001_init.sql` breaks
that rule on purpose — it replaces what were migrations 001 through 042 into one 3,700-line
squash, because the product has never been deployed. There is no production database, no
staging database, and no customer row anywhere; the only databases that have ever run the old
chain were developer machines, the e2e database (truncated by the test suite), and throwaway
databases agents create and drop — every one of them rebuilt from zero, never migrated forward.
Under those conditions, immutability protected nothing and cost a schema file 3,700 lines of
dead intermediate states (columns added and dropped again, a search vector that outlived its
column). `001_init.sql`'s own header carries the full proof this squash is schema-equivalent to
the chain it replaced (`pg_dump` diff, catalog comparison of policies/grants/owners).

## Decision

- Migrations are goose SQL files under `db/migrations/`, forward-only in the normal case
  (`make migrate-up`); `make migrate-down` exists for local rollback during development, gated
  behind a confirmation prompt.
- While there are no production users, an existing migration file may be edited or squashed
  directly instead of being patched forward with a new file, provided (like `001_init.sql`) the
  change carries proof that the resulting schema is equivalent to what would exist without the
  edit.
- `make migrate-create name=<name>` (added by CI-01/DOC-02's PR) scaffolds a new migration with
  `goose -dir ./db/migrations create $(name) sql`, so a new migration always starts from goose's
  own template rather than a copy-pasted file.

## Consequences

- Local development and CI both get a fast, simple migration history: no archaeology through
  forty intermediate files to understand the current schema.
- **This decision expires at first production deployment.** The moment a real database exists
  outside a developer's machine or CI, "edit a migration directly" reverts to the general rule —
  immutable, forward-only, patched with a new file — because a second database can now disagree
  with the first about what already ran. Nothing in tooling enforces that expiry; it is a
  standing responsibility for whoever ships the first production deploy to update this ADR (and
  stop squashing) at that point.

## 2026-09-14 — the second squash

The four migrations that had landed on top of `001_init.sql` were folded into it and deleted:

| File | What it carried |
|---|---|
| `002_user_identities.sql` | The `user_identities` table (Sign in with Google), its two unique constraints and its index. |
| `003_optimistic_concurrency.sql` | `version` on `complexes`, `courts` and `court_prices`, `trigger_bump_version()` and three `set_version` triggers, and both views rebuilt to carry the column. |
| `004_jobs.sql` | The `jobs` table (the durable work queue), its four indexes and its comments. |
| `005_tenant_columns.sql` | `complex_id` on `court_prices`, `blocked_slots`, `slot_locks` and `booking_link_tokens`, the composite foreign keys and `set_complex_id` triggers that keep it honest, the four policies rewritten from an `EXISTS` subquery to a column comparison, and `uuidv7()` defaults on `bookings`, `payments`, `audit_log` and `webhook_events`. |

Each object was merged by hand into the section it belongs to rather than appended, and the
"add column, backfill, set not null, drop and re-add the foreign key" churn that existed only
because of file ordering is gone: those columns are now declared in their `CREATE TABLE`, and the
backfills have nothing to repair on a fresh database. The uuidv7 availability guard moved to the
top of the file as a server capability check. Every comment the four files carried was kept beside
the object it explains. `db/migrations/` is once again a single file.

**How equivalence was proved.** `squash_old` was migrated with the old five-file chain taken from
`origin/main`, `squash_new` with the new `001_init.sql` alone, both on PostgreSQL 18.6. A
`pg_dump --schema-only --no-owner --no-privileges` of each, with `goose_db_version` and the
`SET`/`\restrict` preamble stripped, diffs empty. A catalog comparison then covered what those two
flags hide — relation and function and type ownership, `relacl`, `pg_policy` (name, command,
permissiveness, `USING`, `WITH CHECK`), `pg_default_acl`, the schema ACL, view `reloptions` and
every non-internal trigger definition — and matched everywhere except one place, described below.
`db/truncate_all.sql` and `db/seed_benchmark.sql` were run against `squash_new`: both succeed, and
the seed's 280 `court_prices` rows come back with `complex_id` filled by the trigger.

**One thing the squash silently repaired.** Under the old chain, `jobs`, `user_identities`,
`trigger_bump_version()`, `set_complex_id_from_court()` and `set_complex_id_from_booking()` were
owned by whoever ran migrations 002–005 (`vibe`, the superuser) rather than by `vibe_migrator`:
`001_init.sql`'s ownership walk runs in its ACCESS section, which had already executed before
those four files created anything. The privileges were right anyway — `ALTER DEFAULT PRIVILEGES`
is registered for both creators — so nothing was broken, only inconsistent with the posture the
ACCESS section states. In the merged file every object is created above that section, so all five
are owned by `vibe_migrator`. This is why a new object must never be added after the ACCESS
section, and `backend/CLAUDE.md` now says so.

**What a database that already ran 001–005 does.** Production's `goose_db_version` holds rows for
versions 1 through 5 while the embedded chain now contains only version 1. That is a clean no-op,
not an error: goose's `Up` compares the files it has against the rows for those files, finds
`001_init.sql` applied and nothing pending, and never consults a row whose file is absent. Proved
on `squash_prod` (migrated with the old chain, then handed to the new binary):
`go run ./cmd/api -migrate-only` logs `version_before=5 version_after=5 applied=[] count=0`, then
`total=1 pending=0`, and exits 0; a second run does the same; `goose ... status` lists only
`001_init.sql`; `goose ... up` prints `no migrations to run. current version: 5` and exits 0. The
rows for 2–5 stay in the table untouched. They are harmless, and the one thing that would make
them dangerous is a future migration numbered 2 through 5 — which cannot happen, because
`make migrate-create` numbers from a timestamp. A rollback is the other direction and is not
covered: `001_init.sql`'s Down drops the whole schema, and on such a database goose would then
still hold four rows for files that do not exist.

## 2026-09-15 — three sport values added directly

`sport_type` grew `volleyball`, `hockey` and `pickleball` (courts already supported `padel`,
`tennis`, `soccer`, `basketball`) by editing the `CREATE TYPE sport_type` line in `001_init.sql`
in place rather than adding a new `ALTER TYPE ... ADD VALUE` migration. This is the same
direct-edit allowance the Decision section describes, not an exception to it: every database this
product runs today — developer machines, the e2e database, throwaway agent databases — is
rebuilt from `001_init.sql` from zero, never migrated forward, so there is no already-applied
`ADD VALUE` step an incremental migration would need to reach. Schema equivalence is trivial
here (an enum literal gained three members; nothing else in the file changed) and was checked by
applying the edited file to a fresh throwaway database and confirming
`SELECT enum_range(NULL::sport_type)` returns all seven values in declaration order. The same
expiry as every other direct edit in this ADR applies: the day a real deployed database exists,
adding a sport becomes a new numbered `ALTER TYPE ... ADD VALUE` migration instead.
