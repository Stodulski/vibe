# 0002. sqlc + pgx/v5 for database access

## Status

Accepted.

## Context

The service needs typed Go from hand-written SQL, against PostgreSQL specifically (row-level
security, `CITEXT`, generated range columns with `EXCLUDE` constraints, enums — see the main
`README.md`'s Architecture notes). Two broad choices existed: an ORM that generates or infers
schema, or a code generator that starts from SQL the team already writes and reviews.

## Decision

- [sqlc](https://sqlc.dev) generates `internal/db/*.sql.go` from `db/queries/*.sql`, against the
  schema in `db/migrations/`. The SQL is the source of truth; sqlc only produces the Go binding
  for it. `internal/db` is generated and never hand-edited — `db/queries/*.sql` is, followed by
  `make sqlc`.
- [pgx/v5](https://github.com/jackc/pgx) is the driver and pool, configured as sqlc's
  `sql_package: "pgx/v5"`, not `database/sql`. pgx exposes PostgreSQL-specific types
  (`pgtype.UUID`, `pgtype.Timestamptz`, range types) directly, which the schema's use of native
  Postgres features needs — round-tripping them through `database/sql`'s generic interface would
  mean reimplementing the same conversions by hand.

## Consequences

- Every query is plain, reviewable SQL; there is no query builder or ORM magic translating Go
  method calls into SQL that has to be reasoned about in reverse.
- Type conversions between pgx types and domain types are centralized in
  `internal/data/convert.go` — the coding convention (see `backend/CLAUDE.md`) is to always go
  through those helpers rather than construct `pgtype.*` values by hand elsewhere.
- `sqlc vet`'s `sqlc/db-prepare` rule (added for CI-01/DB-09; see `backend/sqlc.vet.yaml`) can
  validate every query against a real, migrated schema — a query referencing a dropped column or
  passing the wrong parameter count fails vet instead of surfacing as a runtime error.
- The generated code is tied to whatever `sql_package` and schema shape sqlc was pointed at; a
  driver swap would mean regenerating everything under `internal/db`, not just relinking a
  different package.
