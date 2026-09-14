# Postgres backups

Railway does not back up its managed Postgres automatically. `.github/workflows/backend-backup.yml`
runs `pg_dump` once a day and uploads the result to Cloudflare R2. This page is what to read to
restore a database, to set the workflow up, or to check whether it is actually working.

## Quick path

1. Backups run automatically, daily at 06:00 UTC, and land in `postgres/` in the R2 bucket named
   by the `R2_BACKUP_BUCKET` secret. Nothing to do day-to-day.
2. Need one right now (before a risky migration, before an incident)? Run the workflow manually:
   `gh workflow run backend-backup.yml` (or the "Run workflow" button on the Actions tab).
3. Need to restore? Jump to [Restore procedure](#restore-procedure) below.

## What runs, and where it lands

`.github/workflows/backend-backup.yml`, on a schedule (`cron: "0 6 * * *"`) and on manual dispatch:

1. Installs `postgresql-client-18` from the official PGDG apt repository and calls it through
   `/usr/lib/postgresql/18/bin`, because the runner's own client (and PGDG's `/usr/bin/pg_dump`
   wrapper) resolved to an older major and a run failed with `server version: 18.6; pg_dump
   version: 16.15`. A step before the dump compares the two majors and fails naming both. The
   runner's default Postgres client is not pinned to a version that is guaranteed `>=` the
   server's — `pg_dump` can only dump a server at or below its own major version, never above it).
2. Runs `pg_dump --format=custom "$BACKUP_DATABASE_URL"` and fails the job if the output is
   suspiciously small (under 512 bytes — a real dump always carries at least the custom-format
   header, so anything smaller means `pg_dump` failed or the connection dropped). Ownership and
   grants are deliberately kept in the archive: see [Why the dump keeps ownership and
   grants](#why-the-dump-keeps-ownership-and-grants).
3. Names the file `vibe-<UTC timestamp>-<short commit sha>.dump`, gzips it, and uploads it with
   `aws s3 cp` to `s3://$R2_BACKUP_BUCKET/postgres/` against R2's S3-compatible endpoint
   (`https://$R2_BACKUP_ACCOUNT_ID.r2.cloudflarestorage.com`).
4. Deletes the local file from the runner afterwards. The upload log never prints the destination
   URL (it embeds the account id and bucket name); it only confirms the byte count.

**Retention**: not set by this workflow. Recommended: an R2 **lifecycle rule** on the bucket that
expires objects under `postgres/` after 30 days. This has to be set in the R2 dashboard (or with
`aws s3api put-bucket-lifecycle-configuration` against the R2 endpoint) — there is no repository
config for it, and it is not set yet. Until it is, the bucket grows forever.

**Checking the Railway Postgres version**, needed if `postgresql-client-18` ever stops being
`>=` the server: Railway dashboard → the Postgres service → **Data** tab, or run
`SELECT version();` against `BACKUP_DATABASE_URL`. Bump the `postgresql-client-18` install step in
the workflow, and the `/usr/lib/postgresql/18/bin` path beside it, if Railway upgrades past major
version 18; the check step fails first and names both versions.

## Restore procedure

Restoring into a **fresh** Railway Postgres service (never the live one, unless deliberately
overwriting it — `pg_restore --clean` drops and recreates objects).

**Before anything: check the client version.** `pg_restore` cannot read an archive produced by a
newer `pg_dump`, and a distribution's default client is usually older than Railway's server. A
Postgres 18 archive fails on a 16 client. If `pg_restore --version` is below 18, run the restore
inside a container that has the right one:

```bash
docker run --rm -v "$PWD:/w" -w /w postgres:18-alpine pg_restore --version
```

1. Create a new Postgres service in Railway (or reuse an empty one), and copy its connection string.
2. **Create the two application roles in the target cluster, with passwords**, before restoring.
   Roles are cluster-level and are never in the dump (a password in a migration is a password in
   git), but the archive's `OWNER TO`/`GRANT` statements name them:
   ```sql
   CREATE ROLE vibe_migrator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
   CREATE ROLE vibe_app      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
   ALTER ROLE vibe_migrator LOGIN PASSWORD '<generated>';
   ALTER ROLE vibe_app      LOGIN PASSWORD '<generated>';
   ```
   Skipping this step is the one way to get a restore that "succeeds" and leaves an API that cannot
   read its own database — see the section below.
3. Download the dump you need from the R2 bucket (`postgres/vibe-<timestamp>-<sha>.dump.gz`) — the
   Cloudflare dashboard, or:
   ```bash
   aws s3 cp "s3://$R2_BACKUP_BUCKET/postgres/<file>.dump.gz" . \
     --endpoint-url "https://$R2_BACKUP_ACCOUNT_ID.r2.cloudflarestorage.com"
   ```
4. Check the archive before trusting it, then decompress: `gunzip -t <file>.dump.gz` (gzip
   integrity) and `gunzip <file>.dump.gz`.
5. Restore, letting the archive's own ownership and grants apply:
   ```bash
   pg_restore --clean --if-exists --exit-on-error \
     --dbname "$NEW_DATABASE_URL" <file>.dump
   ```
   `--exit-on-error` is not optional here: without it `pg_restore` reports failures and still exits
   0, which is how a half-restored database gets promoted. `--clean --if-exists` drops existing
   objects first, so this is also how to roll a database fully back to the dump's point in time.
6. Verify, in this order — each of these is a check a drill has actually caught something with:
   ```bash
   # schema shape
   psql "$NEW_DATABASE_URL" -tAc "select count(*) from pg_tables where schemaname='public'"
   psql "$NEW_DATABASE_URL" -tAc "select count(*) from pg_policies where schemaname='public'"
   # nothing left unvalidated by the restore
   psql "$NEW_DATABASE_URL" -tAc "select count(*) from pg_constraint where not convalidated"
   # data is there and coherent
   psql "$NEW_DATABASE_URL" -tAc "select coalesce(sum(n_live_tup),0) from pg_stat_user_tables"
   # the tables belong to vibe_migrator, not to whoever ran the restore
   psql "$NEW_DATABASE_URL" -tAc "select tableowner, count(*) from pg_tables where schemaname='public' group by 1"
   ```
   Then the check that matters most, **as the application role**, because every check above can pass
   on a database the API cannot read:
   ```bash
   psql "postgresql://vibe_app:<password>@<host>:<port>/<db>" -tAc 'select count(*) from complexes'
   ```
   What is being checked here is that the statement runs at all, not the number it returns: a `0`
   is the expected answer, because `vibe_app` is `NOBYPASSRLS` and the `tenant_isolation` policy
   filters every row out until a request sets `app.complex_id`. `permission denied for table
   complexes` is the failure — it means the grants did not come across. Fix it with [Re-applying
   access after a stripped restore](#re-applying-access-after-a-stripped-restore) rather than by
   pointing the API at it.
7. Point `DATABASE_URL` (and `DB_MIGRATOR_URL`) at the restored database only after step 6 passes,
   including the `vibe_app` check.

### Why the dump keeps ownership and grants

`vibe_app` owns nothing and is `NOBYPASSRLS`: its entire access comes from the `GRANT` statements in
the ACCESS section of `001_init.sql`. Nothing in the recovery path re-runs that section, so a dump
taken with `--no-owner --no-privileges` restores into a database whose tables are owned by whoever
ran the restore and carry an empty ACL — the schema and every row are intact, and the API still
fails with `permission denied for table complexes`. The 2026-09-14 drill below found exactly that.

Keeping them costs nothing, because `pg_restore` accepts `--no-owner --no-privileges` itself. The
information in the archive is what preserves the choice; dropping it at dump time cannot be undone.

### Re-applying access after a stripped restore

Only needed when the restore could not use the archive's ownership and grants — the target cluster
had no `vibe_migrator`/`vibe_app` and the restore was run with `--no-owner --no-privileges`. Create
the roles (step 2 above), then hand the schema back to `vibe_migrator` and re-issue the grants:

```sql
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT c.oid::regclass AS obj
             FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'S')
    LOOP
        EXECUTE format('ALTER TABLE %s OWNER TO vibe_migrator', r.obj);
    END LOOP;
END $$;

ALTER SCHEMA public OWNER TO vibe_migrator;
GRANT USAGE, CREATE ON SCHEMA public TO vibe_migrator;
GRANT USAGE ON SCHEMA public TO vibe_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO vibe_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO vibe_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO vibe_app;
REVOKE ALL ON TABLE goose_db_version FROM vibe_app;
REVOKE ALL ON SEQUENCE goose_db_version_id_seq FROM vibe_app;
```

Then re-run the `vibe_app` check from step 6. `ALTER TABLE ... OWNER TO` is the right statement for
views and sequences too; PostgreSQL accepts it for every relkind listed above. The authoritative
version of these grants is the ACCESS section at the end of `db/migrations/001_init.sql` — read it
there if this ever disagrees with it.

`backend/scripts/backup.sh` remains available as the manual, ad-hoc path (a local `pg_dump`, kept
for a quick dump against any reachable database without touching GitHub Actions or R2) — it is not
what the scheduled workflow uses.

## Restore drills

An untested backup is a hope, not a backup. Run a real restore into a scratch Railway Postgres on
a recurring basis and log it here.

| Date | Who | Outcome |
|------|-----|---------|
| 2026-09-14 | owner | **Passed with one defect found.** `vibe-20260914T223210Z-daa9bcc.dump.gz` restored into a throwaway `postgres:18.6-alpine` container. `pg_restore --exit-on-error` was clean; the restored schema was byte-identical to a fresh replay of `001_init.sql` (21 tables, 101 indexes, 24 policies, 12 FORCE-RLS tables, 3 EXCLUDE constraints, 0 unvalidated constraints, 32 rows, no orphans). `vibe_app` could not read it — the dump's `--no-owner --no-privileges` flags dropped every grant. Flags removed; see [Why the dump keeps ownership and grants](#why-the-dump-keeps-ownership-and-grants). |

## Secrets to create (GitHub → repository → Settings → Secrets and variables → Actions)

| Secret | Value | Least-privilege notes |
|--------|-------|------------------------|
| `BACKUP_DATABASE_URL` | A Postgres connection string with read access to the database being backed up | A dedicated read-only role is preferable to reusing `DATABASE_URL`/`DB_MIGRATOR_URL`; `pg_dump` needs `SELECT` on every object, nothing else |
| `R2_BACKUP_ACCOUNT_ID` | Cloudflare account id | Not secret by itself, but stored alongside the credentials that are |
| `R2_BACKUP_ACCESS_KEY_ID` | R2 API token access key id | Scope the R2 API token to this one bucket, **Object Read & Write** only (no Admin, no other buckets) |
| `R2_BACKUP_SECRET_ACCESS_KEY` | R2 API token secret | Same token as above |
| `R2_BACKUP_BUCKET` | Bucket name | Should not be the same bucket `R2_BUCKET_NAME` uses for client-uploaded images — keep backups and public assets in separate buckets so a bucket-scoped token for one can never touch the other |

None of these are configured by this PR — creating them in GitHub, scoping the R2 token, and
setting the lifecycle rule are all steps the project owner does outside the repository.
