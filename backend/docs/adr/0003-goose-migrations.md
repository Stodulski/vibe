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
